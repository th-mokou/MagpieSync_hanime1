//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"

	"magpiesync/internal/netproxy"
)

// newSiteHTTPClient returns the read-only HTTP client used against the site.
// It uses the standard Go TLS stack with HTTP/2, which Cloudflare accepts with
// a valid cf_clearance session.
func newSiteHTTPClient(rawProxy string, timeout time.Duration) (*http.Client, error) {
	canonical, err := netproxy.Normalize(rawProxy)
	if err != nil {
		return nil, err
	}
	var proxyURL *url.URL
	if canonical != "" {
		proxyURL, err = url.Parse(canonical)
		if err != nil {
			return nil, fmt.Errorf("hanime1: parse proxy: %w", err)
		}
	}
	dialer := &siteDialer{
		proxy:  proxyURL,
		dialer: &net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second},
	}
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: timeout}, nil
}

type siteDialer struct {
	proxy  *url.URL
	dialer *net.Dialer
}

func (d *siteDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d.proxy == nil {
		return d.dialer.DialContext(ctx, network, addr)
	}
	switch d.proxy.Scheme {
	case "socks5", "socks5h":
		var auth *xproxy.Auth
		if d.proxy.User != nil {
			password, _ := d.proxy.User.Password()
			auth = &xproxy.Auth{User: d.proxy.User.Username(), Password: password}
		}
		dialer, err := xproxy.SOCKS5("tcp", d.proxy.Host, auth, xproxy.Direct)
		if err != nil {
			return nil, err
		}
		ctxDialer, ok := dialer.(xproxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("hanime1: SOCKS5 proxy does not support context")
		}
		return ctxDialer.DialContext(ctx, network, addr)
	case "http", "https":
		return d.dialHTTPProxy(ctx, addr)
	default:
		return nil, fmt.Errorf("hanime1: unsupported proxy scheme %q", d.proxy.Scheme)
	}
}

func (d *siteDialer) dialHTTPProxy(ctx context.Context, addr string) (net.Conn, error) {
	conn, err := d.dialer.DialContext(ctx, "tcp", d.proxy.Host)
	if err != nil {
		return nil, err
	}
	if d.proxy.Scheme == "https" {
		host, _, _ := net.SplitHostPort(d.proxy.Host)
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if d.proxy.User != nil {
		password, _ := d.proxy.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(d.proxy.User.Username() + ":" + password))
		fmt.Fprintf(&b, "Proxy-Authorization: Basic %s\r\n", token)
	}
	b.WriteString("Proxy-Connection: keep-alive\r\n\r\n")
	if _, err := conn.Write([]byte(b.String())); err != nil {
		_ = conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("hanime1: proxy CONNECT failed: HTTP %d", resp.StatusCode)
	}
	return conn, nil
}
