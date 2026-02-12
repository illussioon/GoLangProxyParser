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

	"github.com/schollz/progressbar/v3"
)

func main() {
	helpFlag := flag.Bool("h", false, "Show help")
	proxyLimit := flag.Int("proxy", 0, "Number of valid proxies to find (0 = no limit)")
	proxyType := flag.String("type", "", "Type of proxy: http, socks4, socks5")
	validations := flag.Int("validations", 1, "Number of times to validate each proxy")
	outputFile := flag.String("file", "valid_proxies.txt", "File to save valid proxies in real-time")
	threads := flag.Int("threads", 10, "Number of threads (workers)")
	flag.Parse()

	if *helpFlag {
		fmt.Println("Usage of ProxyParserGO:")
		flag.PrintDefaults()
		return
	}

	startTime := time.Now()
	fmt.Println("Starting Proxy Parser & Checker...")

	// Open output file
	f, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer f.Close()

	fetchers := []fetcher.Fetcher{
		&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/iplocate/free-proxy-list/refs/heads/main/all-proxies.txt", Protocol: models.HTTP, Source: "iplocate"},
		&fetcher.TextFetcher{URL: "https://api.proxyscrape.com/v4/free-proxy-list/get?request=get_proxies&skip=0&proxy_format=protocolipport&format=txt&limit=1000000&timeout=200000", Protocol: models.HTTP, Source: "proxyscrape"},
		&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks5.txt", Protocol: models.SOCKS5, Source: "TheSpeedX-SOCKS5"},
		&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks4.txt", Protocol: models.SOCKS4, Source: "TheSpeedX-SOCKS4"},
		&fetcher.TextFetcher{URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt", Protocol: models.HTTP, Source: "TheSpeedX-HTTP"},
		&fetcher.GeonodeFetcher{
			BaseURL: "https://proxylist.geonode.com/api/proxy-list?limit=500&sort_by=lastChecked&sort_type=desc",
			Limit:   500,
			Pages:   10,
		},
	}

	var allProxies []models.Proxy
	var mu sync.Mutex
	var wg sync.WaitGroup

	fmt.Println("Fetching proxies from sources...")

	for _, f := range fetchers {
		wg.Add(1)
		go func(f fetcher.Fetcher) {
			defer wg.Done()
			proxies, err := f.Fetch()
			if err != nil {
				fmt.Printf("Error fetching: %v\n", err)
				return
			}
			mu.Lock()
			allProxies = append(allProxies, proxies...)
			mu.Unlock()
		}(f)
	}
	wg.Wait()

	fmt.Printf("Total fetched: %d. Deduplicating and Filtering...\n", len(allProxies))

	uniqueProxies := make(map[string]models.Proxy)
	filterType := strings.ToLower(*proxyType)

	for _, p := range allProxies {
		if filterType != "" && string(p.Protocol) != filterType {
			continue
		}
		key := fmt.Sprintf("%s:%s", p.IP, p.Port)
		uniqueProxies[key] = p
	}

	fmt.Printf("Unique proxies to check: %d. Starting Checker with %d workers...\n", len(uniqueProxies), *threads)
	if *validations > 1 {
		fmt.Printf("Each proxy will be validated %d times.\n", *validations)
	}

	jobs := make(chan models.Proxy, len(uniqueProxies))
	results := make(chan models.Proxy)

	// Progress Bar
	bar := progressbar.Default(int64(len(uniqueProxies)))
	var validCount int32 = 0

	var workerWg sync.WaitGroup
	for i := 0; i < *threads; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for p := range jobs {
				isValid := true
				for v := 0; v < *validations; v++ {
					if !checker.Check(p) {
						isValid = false
						break
					}
				}

				// Update progress bar
				bar.Add(1)

				if isValid {
					results <- p
				}
			}
		}()
	}

	go func() {
		for _, p := range uniqueProxies {
			jobs <- p
		}
		close(jobs)
	}()

	go func() {
		workerWg.Wait()
		close(results)
	}()

	limit := *proxyLimit

	for p := range results {
		newCount := atomic.AddInt32(&validCount, 1)
		bar.Describe(fmt.Sprintf("Valid: %d", newCount))

		if _, err := f.WriteString(fmt.Sprintf("%s\n", p.String())); err != nil {
			// Ignore error or print to stderr
		}

		if limit > 0 && int(newCount) >= limit {
			fmt.Println("\nLimit reached! Stopping...")
			break
		}
	}
	bar.Finish()

	elapsed := time.Since(startTime)
	fmt.Printf("\nDone! Saved %d valid proxies to %s. Took %s\n", validCount, *outputFile, elapsed)
}
