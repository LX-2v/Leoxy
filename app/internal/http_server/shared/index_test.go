package shared

import (
	"testing"
	"time"
)

func TestNewProxyTransportUsesResponseHeaderTimeout(t *testing.T) {
	transport := NewProxyTransport(3*time.Second, 30*time.Second)
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %s, want 30s", transport.ResponseHeaderTimeout)
	}
	if !transport.DisableCompression {
		t.Fatal("DisableCompression = false, want true")
	}
}
