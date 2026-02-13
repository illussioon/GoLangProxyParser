package fetcher

import (
	"bufio"
	"fmt"
	"net/http"
	"time"

	"ProxyParserGO/pkg/models"
)

// Logger is a function that accepts a string message
type Logger func(string)

// ProxyCallback determines how found proxies are delivered
type ProxyCallback func([]models.Proxy)

type Fetcher interface {
	Fetch(logger Logger, onProxies ProxyCallback) error
}

// fetchURL is a helper function to fetch raw text content from a URL
func fetchURL(url string) ([]string, error) {
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var lines []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}
