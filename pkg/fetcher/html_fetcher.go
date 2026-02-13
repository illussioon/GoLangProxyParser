package fetcher

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"ProxyParserGO/pkg/models"

	"github.com/PuerkitoBio/goquery"
)

type HTMLFetcher struct {
	URL    string
	Source string
}

func (f *HTMLFetcher) Fetch(logger Logger, onProxies ProxyCallback) error {
	if logger != nil {
		logger(fmt.Sprintf("Fetching HTML from %s...", f.Source))
	}

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(f.URL)
	if err != nil {
		if logger != nil {
			logger(fmt.Sprintf("Error fetching %s: %v", f.Source, err))
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if logger != nil {
			logger(fmt.Sprintf("Bad status %s for %s", resp.Status, f.Source))
		}
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		if logger != nil {
			logger(fmt.Sprintf("Error parsing HTML for %s: %v", f.Source, err))
		}
		return err
	}

	var proxies []models.Proxy
	count := 0
	doc.Find(".fpl-list table tbody tr").Each(func(i int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() >= 7 {
			ip := strings.TrimSpace(tds.Eq(0).Text())
			port := strings.TrimSpace(tds.Eq(1).Text())
			https := strings.TrimSpace(strings.ToLower(tds.Eq(6).Text()))

			if ip == "" {
				return
			}

			protocol := models.HTTP
			if https == "yes" {
				protocol = models.HTTP
			}

			proxies = append(proxies, models.Proxy{
				IP:       ip,
				Port:     port,
				Protocol: protocol,
				Source:   f.Source,
			})
			count++
		}
	})

	if len(proxies) > 0 && onProxies != nil {
		onProxies(proxies)
	}

	if logger != nil {
		logger(fmt.Sprintf("Fetched %d proxies from %s", count, f.Source))
	}
	return nil
}
