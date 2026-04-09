package utils

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
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
		d := &net.Dialer{
			Control: SafeControl,
		}
		tc.DialContext = d.DialContext
		return tc
	}

	return rt
}
