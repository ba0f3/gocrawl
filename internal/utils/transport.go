package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// SafeTransport returns an http.RoundTripper that blocks connections to internal IPs (SSRF protection).
func SafeTransport() http.RoundTripper {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		Proxy:                 nil, // disable proxying from env to avoid SSRF bypass
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, err := dialer.Resolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}

			var safeIPs []net.IP
			for _, ip := range ips {
				if !isPrivateIP(ip) {
					safeIPs = append(safeIPs, ip)
				}
			}

			if len(safeIPs) == 0 {
				return nil, fmt.Errorf("SSRF protection: blocked connection to private IP for %s", host)
			}

			// Dial the first safe IP
			for _, ip := range safeIPs {
				safeAddr := net.JoinHostPort(ip.String(), port)
				conn, err := dialer.DialContext(ctx, network, safeAddr)
				if err == nil {
					return conn, nil
				}
			}

			return nil, fmt.Errorf("failed to dial any safe IPs for %s", host)
		},
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
	} else {
		// IPv6 specific checks
		// Unique Local Addresses (ULA) fc00::/7
		if ip[0]&0xfe == 0xfc {
			return true
		}
	}

	return false
}
