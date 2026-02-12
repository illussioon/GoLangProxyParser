package models

import "fmt"

type Protocol string

const (
	HTTP   Protocol = "http"
	SOCKS4 Protocol = "socks4"
	SOCKS5 Protocol = "socks5"
)

type Proxy struct {
	IP       string
	Port     string
	Protocol Protocol
	Source   string
}

func (p Proxy) String() string {
	return fmt.Sprintf("%s://%s:%s", p.Protocol, p.IP, p.Port)
}

func (p Proxy) Address() string {
	return fmt.Sprintf("%s:%s", p.IP, p.Port)
}
