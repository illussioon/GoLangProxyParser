package fetcher

import (
	"strings"

	"ProxyParserGO/pkg/models"
)

type TextFetcher struct {
	URL      string
	Protocol models.Protocol
	Source   string
}

func (f *TextFetcher) Fetch() ([]models.Proxy, error) {
	lines, err := fetchURL(f.URL)
	if err != nil {
		return nil, err
	}

	var proxies []models.Proxy
	for _, line := range lines {
		line = strings.TrimSpace(line)
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
	return proxies, nil
}
