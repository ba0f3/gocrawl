package crawler

import (
	"context"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"

	"gocrawl/internal/utils"
)

// newChromeHTTPTransport returns an *http.Transport that uses uTLS with a Chrome ClientHello
// (HelloChrome_Auto) for HTTPS, HTTP/2 via ALPN, and copies proxy/connection pool defaults from
// the standard library transport.
func newChromeHTTPTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	t := base.Clone()

	t.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := net.Dialer{
			Timeout: 30 * time.Second,
			Control: utils.SafeControl,
		}
		rawConn, err := d.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		host, _, splitErr := net.SplitHostPort(addr)
		if splitErr != nil {
			host = addr
		}
		cfg := &utls.Config{
			ServerName:         host,
			InsecureSkipVerify: t.TLSClientConfig != nil && t.TLSClientConfig.InsecureSkipVerify,
		}
		if t.TLSClientConfig != nil && t.TLSClientConfig.RootCAs != nil {
			cfg.RootCAs = t.TLSClientConfig.RootCAs
		}
		uconn := utls.UClient(rawConn, cfg, utls.HelloChrome_Auto)
		handshakeTimeout := t.TLSHandshakeTimeout
		if handshakeTimeout <= 0 {
			handshakeTimeout = 10 * time.Second
		}
		handshakeCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
		defer cancel()
		if err := uconn.HandshakeContext(handshakeCtx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}
		return uconn, nil
	}

	// Ensure we do not run the default TLS stack on top of uTLS.
	t.TLSClientConfig = nil

	if err := http2.ConfigureTransport(t); err != nil {
		// Extremely unlikely; keep HTTP/1.1-only if configuration fails.
		_ = err
	}
	return t
}
