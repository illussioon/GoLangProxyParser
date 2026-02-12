package fetcher

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ProxyParserGO/pkg/models"
)

type Fetcher interface {
	Fetch() ([]models.Proxy, error)
}

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
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
