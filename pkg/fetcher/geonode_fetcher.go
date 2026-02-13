package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ProxyParserGO/pkg/models"
)

type GeonodeFetcher struct {
	BaseURL string
	Limit   int
	Pages   int
}

type geonodeResponse struct {
	Data []struct {
		IP        string   `json:"ip"`
		Port      string   `json:"port"`
		Protocols []string `json:"protocols"`
	} `json:"data"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

func (f *GeonodeFetcher) Fetch(logger Logger, onProxies ProxyCallback) error {
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	if logger != nil {
		logger("Starting fetch from Geonode...")
	}

	totalFetched := 0

	for page := 1; page <= f.Pages; page++ {
		url := fmt.Sprintf("%s&page=%d", f.BaseURL, page)
		if logger != nil {
			logger(fmt.Sprintf("Fetching Geonode page %d...", page))
		}

		resp, err := client.Get(url)
		if err != nil {
			if logger != nil {
				logger(fmt.Sprintf("Error fetching Geonode page %d: %v", page, err))
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if logger != nil {
				logger(fmt.Sprintf("Geonode page %d returned status: %s", page, resp.Status))
			}
			resp.Body.Close()
			continue
		}

		var result geonodeResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			if logger != nil {
				logger(fmt.Sprintf("Error decoding Geonode page %d: %v", page, err))
			}
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if len(result.Data) == 0 {
			break
		}

		var pageProxies []models.Proxy
		for _, item := range result.Data {
			for _, p := range item.Protocols {
				var protocol models.Protocol
				protocolStr := p
				switch protocolStr {
				case "socks4":
					protocol = models.SOCKS4
				case "socks5":
					protocol = models.SOCKS5
				case "http", "https":
					protocol = models.HTTP
				default:
					continue
				}

				pageProxies = append(pageProxies, models.Proxy{
					IP:       item.IP,
					Port:     item.Port,
					Protocol: protocol,
					Source:   "Geonode",
				})
			}
		}

		if len(pageProxies) > 0 && onProxies != nil {
			onProxies(pageProxies)
			totalFetched += len(pageProxies)
		}
	}

	if logger != nil {
		logger(fmt.Sprintf("Finished fetching from Geonode. Total: %d", totalFetched))
	}
	return nil
}
