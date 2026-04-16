package utils

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrBlockedConnection is returned when SafeTransport prevents a connection to an internal IP.
var ErrBlockedConnection = errors.New("SSRF protection: blocked connection to private IP")

// SafeTransport returns an http.RoundTripper that blocks connections to internal IPs (SSRF protection).
func SafeTransport() http.RoundTripper {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				// Prevent bypass via IPv6 zone identifiers or invalid parsing
				return fmt.Errorf("%w: invalid IP format %s", ErrBlockedConnection, host)
			}
			if isPrivateIP(ip) {
				return fmt.Errorf("%w for %s", ErrBlockedConnection, host)
			}
			return nil
		},
	}

	return &http.Transport{
		Proxy:                 nil, // disable proxying from env to avoid SSRF bypass
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           dialer.DialContext,
	}
}

// isPrivateIP checks if an IP belongs to private or loopback ranges
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true // treat nil as unsafe/private
	}
	// Check standard private blocks and loopback
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	// Additional manual checks
	ip4 := ip.To4()
	if ip4 != nil {
		// 0.0.0.0/8 (Current network)
		if ip4[0] == 0 {
			return true
		}
	}

	return false
}
