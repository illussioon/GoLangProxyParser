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

func (f *GeonodeFetcher) Fetch() ([]models.Proxy, error) {
	var allProxies []models.Proxy
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	for page := 1; page <= f.Pages; page++ {
		url := fmt.Sprintf("%s&page=%d", f.BaseURL, page)
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("Error fetching Geonode page %d: %v\n", page, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Geonode page %d returned status: %s\n", page, resp.Status)
			resp.Body.Close()
			continue
		}

		var result geonodeResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			fmt.Printf("Error decoding Geonode page %d: %v\n", page, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if len(result.Data) == 0 {
			break
		}

		for _, item := range result.Data {
			// Map protocols
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

				allProxies = append(allProxies, models.Proxy{
					IP:       item.IP,
					Port:     item.Port,
					Protocol: protocol,
					Source:   "Geonode",
				})
			}
		}
	}

	return allProxies, nil
}
