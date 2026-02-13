package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ProxyParserGO/pkg/checker"
	"ProxyParserGO/pkg/fetcher"
	"ProxyParserGO/pkg/models"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Flags
var (
	proxyLimit  = flag.Int("proxy", 0, "Number of valid proxies to find (0 = no limit)")
	proxyType   = flag.String("type", "", "Type of proxy: http, socks4, socks5")
	validations = flag.Int("validations", 1, "Number of times to validate each proxy")
	outputFile  = flag.String("file", "valid_proxies.txt", "Output file for valid proxies")
	threads     = flag.Int("threads", 10, "Number of concurrent threads")
	timeout     = flag.Int("timeout", 10, "Timeout in seconds for checking")
	checkURL    = flag.String("check-url", "https://www.google.com", "URL to use for checking proxy connectivity")
	debug       = flag.Bool("debug", false, "Enable debug logging to debug.txt")
)

type appState int

const (
	stateFetching appState = iota
	stateChecking
	stateDone
)

// Messages
type logMsg string
type newProxiesMsg []models.Proxy
type validProxyMsg models.Proxy
type finishedFetchingMsg struct{}
type finishedCheckingMsg struct{}

// Channels for coordination
var checkResults = make(chan models.Proxy, 100)
var checkProgress = make(chan bool, 1000)
var checkDone = make(chan bool)

// Model
type model struct {
	state    appState
	logs     []string
	viewport viewport.Model
	spinner  spinner.Model
	progress progress.Model

	allProxies   []models.Proxy
	checkedCount int32
	validCount   int32
	totalToTest  int

	outputFile *os.File

	width  int
	height int
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(80, 20)
	vp.SetContent("Starting Proxy Parser...\n")

	prog := progress.New(progress.WithDefaultGradient())

	f, _ := os.Create(*outputFile)

	return model{
		state:      stateFetching,
		spinner:    s,
		viewport:   vp,
		progress:   prog,
		outputFile: f,
		logs:       []string{},
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.outputFile != nil {
				m.outputFile.Close()
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 5
		m.progress.Width = msg.Width - 4

	case logMsg:
		m.logs = append(m.logs, string(msg))
		if len(m.logs) > 1000 {
			m.logs = m.logs[len(m.logs)-1000:]
		}
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		m.viewport.GotoBottom()
		return m, nil

	case newProxiesMsg:
		m.allProxies = append(m.allProxies, msg...)

	case finishedFetchingMsg:
		m.state = stateChecking
		m.deduplicate()

		timeoutDuration := time.Duration(*timeout) * time.Second

		// Start checking
		startChecking(m.allProxies, *threads, *validations, timeoutDuration, *checkURL, *debug)
		return m, waitForCheckEvent()

	case validProxyMsg:
		if m.outputFile != nil {
			m.outputFile.WriteString(msg.IP + ":" + msg.Port + "\n")
		}
		atomic.AddInt32(&m.validCount, 1)
		limit := *proxyLimit
		if limit > 0 && int(m.validCount) >= limit {
			return m, tea.Quit
		}
		return m, waitForCheckEvent()

	case int: // Progress tick
		atomic.AddInt32(&m.checkedCount, 1)
		pct := float64(m.checkedCount) / float64(m.totalToTest)
		if m.totalToTest == 0 {
			pct = 0
		}
		cmd = m.progress.SetPercent(pct)
		return m, tea.Batch(cmd, waitForCheckEvent())

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case finishedCheckingMsg:
		m.state = stateDone
		return m, tea.Quit
	}

	switch m.state {
	case stateFetching:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) deduplicate() {
	unique := make(map[string]models.Proxy)
	filterType := strings.ToLower(*proxyType)

	for _, p := range m.allProxies {
		if filterType != "" && string(p.Protocol) != filterType {
			continue
		}
		key := fmt.Sprintf("%s:%s", p.IP, p.Port)
		unique[key] = p
	}

	m.allProxies = make([]models.Proxy, 0, len(unique))
	for _, p := range unique {
		m.allProxies = append(m.allProxies, p)
	}
	m.totalToTest = len(m.allProxies)
}

func (m model) View() string {
	if m.state == stateFetching {
		header := fmt.Sprintf("%s Fetching proxies... Total fetched: %d", m.spinner.View(), len(m.allProxies))
		return fmt.Sprintf("%s\n\n%s\n\nPres q to quit.", header, m.viewport.View())
	}

	if m.state == stateChecking {
		valid := atomic.LoadInt32(&m.validCount)
		checked := atomic.LoadInt32(&m.checkedCount)

		status := fmt.Sprintf("Checking... Valid: %d | Checked: %d / %d", valid, checked, m.totalToTest)
		return fmt.Sprintf("\n%s\n%s\n\nPress q to quit.", status, m.progress.View())
	}

	return "Done!"
}

// Helpers

func startChecking(proxies []models.Proxy, threads int, validations int, timeout time.Duration, checkURL string, debugMode bool) {
	jobs := make(chan models.Proxy, len(proxies))

	// Debug file setup
	var debugFile *os.File
	var debugMu sync.Mutex
	if debugMode {
		var err error
		debugFile, err = os.Create("debug.txt")
		if err != nil {
			fmt.Printf("Error creating debug.txt: %v\n", err)
		} else {
			defer debugFile.Close() // This defer runs when startChecking returns, which is immediately.
			// We need to pass the file pointer or handle lifecycle differently.
			// Actually, goroutines will run after this returns.
			// We must NOT close it here if we want workers to write to it.
			// Better: open it, keep it open, let OS close it on exit?
			// Or just open/append/close for each error (slower but safer)
		}
	}

	debugLog := func(msg string) {
		if !debugMode {
			return
		}
		debugMu.Lock()
		defer debugMu.Unlock()
		if debugFile == nil {
			// Try to open/append
			f, err := os.OpenFile("debug.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return
			}
			defer f.Close()
			f.WriteString(msg + "\n")
		} else {
			debugFile.WriteString(msg + "\n")
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				isValid := true
				var lastErr error
				for v := 0; v < validations; v++ {
					ok, err := checker.Check(p, checkURL, timeout)
					if !ok {
						isValid = false
						lastErr = err
						break
					}
				}

				checkProgress <- true
				if isValid {
					checkResults <- p
				} else {
					// Log failure
					if debugMode && lastErr != nil {
						debugLog(fmt.Sprintf("%s:%s (%s) -> Failed: %v", p.IP, p.Port, p.Protocol, lastErr))
					}
				}
			}
		}()
	}

	go func() {
		for _, p := range proxies {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		if debugFile != nil {
			debugFile.Close()
		}
		checkDone <- true
	}()
}

func waitForCheckEvent() tea.Cmd {
	return func() tea.Msg {
		select {
		case p := <-checkResults:
			return validProxyMsg(p)
		case <-checkProgress:
			return int(1) // progress tick
		case <-checkDone:
			return finishedCheckingMsg{}
		}
	}
}

func main() {
	flag.Parse()

	m := initialModel()
	p := tea.NewProgram(m)

	go func() {
		logger := func(msg string) {
			p.Send(logMsg(msg))
		}

		onProxies := func(proxies []models.Proxy) {
			p.Send(newProxiesMsg(proxies))
		}

		fetchers := []fetcher.Fetcher{
			&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/iplocate/free-proxy-list/refs/heads/main/all-proxies.txt", Protocol: models.HTTP, Source: "iplocate"},
			&fetcher.TextFetcher{URL: "https://api.proxyscrape.com/v4/free-proxy-list/get?request=get_proxies&skip=0&proxy_format=protocolipport&format=text&limit=1000000&timeout=200000", Protocol: models.HTTP, Source: "proxyscrape"},
			&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks5.txt", Protocol: models.SOCKS5, Source: "TheSpeedX-SOCKS5"},
			&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks4.txt", Protocol: models.SOCKS4, Source: "TheSpeedX-SOCKS4"},
			&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt", Protocol: models.HTTP, Source: "TheSpeedX-HTTP"},
			&fetcher.GeonodeFetcher{
				BaseURL: "https://proxylist.geonode.com/api/proxy-list?limit=500&sort_by=lastChecked&sort_type=desc",
				Limit:   500,
				Pages:   10,
			},
			&fetcher.HTMLFetcher{
				URL:    "https://free-proxy-list.net/ru/",
				Source: "free-proxy-list.net",
			},
			&fetcher.ProxyDBFetcher{
				BaseURL: "https://proxydb.net/",
				Source:  "proxydb.net",
			},
		}

		var wg sync.WaitGroup
		for _, f := range fetchers {
			wg.Add(1)
			go func(f fetcher.Fetcher) {
				defer wg.Done()
				if err := f.Fetch(logger, onProxies); err != nil {
					p.Send(logMsg(fmt.Sprintf("Error: %v", err)))
				}
			}(f)
		}
		wg.Wait()
		p.Send(finishedFetchingMsg{})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
