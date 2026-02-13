package checker

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"ProxyParserGO/pkg/models"

	"golang.org/x/net/proxy"
)

// Check validates a proxy against a target URL.
// Returns success status and any error encountered.
func Check(p models.Proxy, targetURL string, timeout time.Duration) (bool, error) {
	var client *http.Client

	if timeout == 0 {
		timeout = 10 * time.Second
	}
	// Default to Google if empty, but caller should provide it
	if targetURL == "" {
		targetURL = "https://www.google.com"
	}

	switch p.Protocol {
	case models.HTTP:
		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s", p.IP, p.Port))
		if err != nil {
			return false, fmt.Errorf("url parse error: %w", err)
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			DialContext: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			DisableKeepAlives: true,
		}

		client = &http.Client{
			Transport: transport,
			Timeout:   timeout,
		}

	case models.SOCKS5:
		dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", p.IP, p.Port), nil, proxy.Direct)
		if err != nil {
			return false, fmt.Errorf("socks5 dialer error: %w", err)
		}

		client = &http.Client{
			Transport: &http.Transport{
				Dial:              dialer.Dial,
				DisableKeepAlives: true,
			},
			Timeout: timeout,
		}

	case models.SOCKS4:
		dialer := &SOCKS4Dialer{
			ProxyIP:   p.IP,
			ProxyPort: p.Port,
			Timeout:   timeout,
		}

		client = &http.Client{
			Transport: &http.Transport{
				Dial:              dialer.Dial,
				DisableKeepAlives: true,
			},
			Timeout: timeout,
		}

	default:
		return false, fmt.Errorf("unknown protocol: %s", p.Protocol)
	}

	if client == nil {
		return false, fmt.Errorf("failed to create client")
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, fmt.Errorf("status code: %d", resp.StatusCode)
}
