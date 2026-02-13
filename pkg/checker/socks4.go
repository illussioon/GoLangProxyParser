package checker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

type SOCKS4Dialer struct {
	ProxyIP   string
	ProxyPort string
	Timeout   time.Duration
}

func (d *SOCKS4Dialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", d.ProxyIP, d.ProxyPort), d.Timeout)
	if err != nil {
		return nil, err
	}

	// Set deadline for the handshake
	conn.SetDeadline(time.Now().Add(d.Timeout))

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			conn.Close()
			return nil, fmt.Errorf("failed to resolve IP for %s", host)
		}
		ip = ips[0]
	}
	ip4 := ip.To4()
	if ip4 == nil {
		conn.Close()
		return nil, errors.New("SOCKS4 only supports IPv4")
	}

	req := make([]byte, 0, 9)
	req = append(req, 4)
	req = append(req, 1)

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	req = append(req, portBytes...)

	req = append(req, ip4...)
	req = append(req, 0)

	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}
	resp := make([]byte, 8)
	if _, err := conn.Read(resp); err != nil {
		conn.Close()
		return nil, err
	}

	if resp[1] != 0x5a {
		conn.Close()
		return nil, fmt.Errorf("SOCKS4 request failed with code: %d", resp[1])
	}

	// Clear deadline for subsequent use (or let client set it)
	conn.SetDeadline(time.Time{})
	return conn, nil
}
