//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"net/http"
	"testing"
)

type idleTrackingTransport struct {
	closed bool
}

func (t *idleTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	panic("unexpected request")
}

func (t *idleTrackingTransport) CloseIdleConnections() {
	t.closed = true
}

func TestClientCloseClosesIdleConnections(t *testing.T) {
	transport := &idleTrackingTransport{}
	c := &Client{http: &http.Client{Transport: transport}}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if !transport.closed {
		t.Fatal("idle connections were not closed")
	}
}
