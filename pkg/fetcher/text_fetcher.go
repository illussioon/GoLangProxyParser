package fetcher

import (
	"fmt"
	"strings"

	"ProxyParserGO/pkg/models"
)

type TextFetcher struct {
	URL      string
	Protocol models.Protocol
	Source   string
}

func (f *TextFetcher) Fetch(logger Logger, onProxies ProxyCallback) error {
	if logger != nil {
		logger(fmt.Sprintf("Fetching text list from %s...", f.Source))
	}

	lines, err := fetchURL(f.URL)
	if err != nil {
		if logger != nil {
			logger(fmt.Sprintf("Error fetching %s: %v", f.Source, err))
		}
		return err
	}

	var proxies []models.Proxy
	for _, line := range lines {
		// Clean line
		line = strings.TrimSpace(line)

		// Strip protocol prefix if present
		if idx := strings.Index(line, "://"); idx != -1 {
			line = line[idx+3:]
		}

		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			proxies = append(proxies, models.Proxy{
				IP:       parts[0],
				Port:     parts[1],
				Protocol: f.Protocol,
				Source:   f.Source,
			})
		}
	}

	if len(proxies) > 0 && onProxies != nil {
		onProxies(proxies)
	}

	if logger != nil {
		logger(fmt.Sprintf("Fetched %d proxies from %s", len(proxies), f.Source))
	}
	return nil
}
