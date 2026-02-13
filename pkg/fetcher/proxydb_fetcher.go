package fetcher

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"ProxyParserGO/pkg/models"

	"github.com/PuerkitoBio/goquery"
)

type ProxyDBFetcher struct {
	BaseURL string
	Source  string
}

func (f *ProxyDBFetcher) Fetch(logger Logger, onProxies ProxyCallback) error {
	// Open log file for persistent logging
	logFile, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil && logger != nil {
		logger(fmt.Sprintf("Error opening log file: %v", err))
	} else if logFile != nil {
		defer logFile.Close()
	}

	// Helper to log to both file and TUI
	log := func(msg string) {
		// Log to file
		if logFile != nil {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			logFile.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
		}
		// Log to TUI
		if logger != nil {
			logger(fmt.Sprintf("[%s] %s", f.Source, msg))
		}
	}

	client := http.Client{Timeout: 30 * time.Second}
	offset := 0
	step := 30
	retries := 0
	totalFetched := 0

	log("Starting fetch...")

	for {
		url := fmt.Sprintf("%s?offset=%d", f.BaseURL, offset)
		log(fmt.Sprintf("Fetching offset %d...", offset))

		resp, err := client.Get(url)
		if err != nil {
			log(fmt.Sprintf("Error fetching %s: %v", url, err))
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retries++
			if retries > 10 {
				log("Max retries reached for 429. Stopping.")
				break
			}
			delay := time.Duration(retries*5) * time.Second
			log(fmt.Sprintf("Rate limited (429). Sleeping %s...", delay))
			time.Sleep(delay)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log(fmt.Sprintf("Status %s for %s", resp.Status, url))
			resp.Body.Close()
			break
		}

		retries = 0

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			log(fmt.Sprintf("Error parsing HTML: %v", err))
			break
		}

		noProxies := false
		doc.Find("td").Each(func(i int, s *goquery.Selection) {
			if strings.Contains(s.Text(), "No Proxies found") {
				noProxies = true
			}
		})

		if noProxies {
			log("No more proxies found. Stopping.")
			break
		}

		rows := doc.Find("div.table-responsive table tbody tr")
		if rows.Length() == 0 {
			log("No rows found. Stopping.")
			break
		}

		count := 0
		var pageProxies []models.Proxy
		rows.Each(func(i int, s *goquery.Selection) {
			ipLink := s.Find("td:nth-child(1) a")
			ip := strings.TrimSpace(ipLink.Text())

			portLink := s.Find("td:nth-child(2) a")
			port := strings.TrimSpace(portLink.Text())

			protocolStr := strings.TrimSpace(s.Find("td:nth-child(3)").Text())

			if ip == "" || port == "" {
				return
			}

			var protocol models.Protocol
			protocolStrLower := strings.ToLower(protocolStr)
			if strings.Contains(protocolStrLower, "socks5") {
				protocol = models.SOCKS5
			} else if strings.Contains(protocolStrLower, "socks4") {
				protocol = models.SOCKS4
			} else {
				protocol = models.HTTP
			}

			p := models.Proxy{
				IP:       ip,
				Port:     port,
				Protocol: protocol,
				Source:   f.Source,
			}
			pageProxies = append(pageProxies, p)
			count++
		})

		if len(pageProxies) > 0 && onProxies != nil {
			onProxies(pageProxies)
			totalFetched += count
		}

		log(fmt.Sprintf("Fetched %d proxies from offset %d", count, offset))

		if count == 0 {
			break
		}

		offset += step
		time.Sleep(500 * time.Millisecond)
	}

	log(fmt.Sprintf("Finished fetching. Total: %d", totalFetched))
	return nil
}
