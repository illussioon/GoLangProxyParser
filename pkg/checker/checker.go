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

const (
	TargetURL = "https://www.google.com"
	Timeout   = 10 * time.Second
)

func Check(p models.Proxy) bool {
	var client *http.Client

	switch p.Protocol {
	case models.HTTP:
		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s", p.IP, p.Port))
		if err != nil {
			return false
		}
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				DialContext: (&net.Dialer{
					Timeout:   Timeout,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
			Timeout: Timeout,
		}

	case models.SOCKS5:
		dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", p.IP, p.Port), nil, proxy.Direct)
		if err != nil {
			return false
		}
		client = &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
			Timeout: Timeout,
		}

	case models.SOCKS4:
		dialer := &SOCKS4Dialer{
			ProxyIP:   p.IP,
			ProxyPort: p.Port,
		}
		client = &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
			Timeout: Timeout,
		}

	default:
		return false
	}

	if client == nil {
		return false
	}

	resp, err := client.Get(TargetURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
