package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// SafeControl checks if the resolved IP address is a private, loopback, unspecified, or link-local address.
func SafeControl(network, address string, c syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return fmt.Errorf("SSRF blocked: private/loopback IP: %s", ip)
	}
	return nil
}

// SafeTransport takes an http.RoundTripper and returns one with an SSRF-safe DialContext via net.Dialer.Control.
func SafeTransport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		return nil
	}

	if t, ok := rt.(*http.Transport); ok {
		tc := t.Clone()

		tc.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				Control:   SafeControl,
			}
			return d.DialContext(ctx, network, addr)
		}

		return tc
	}

	return rt
}
