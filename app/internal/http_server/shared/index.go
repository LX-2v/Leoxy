package shared

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

func IsClosedError(err error) bool {
	return errors.Is(err, net.ErrClosed)
}

var ProxyTransport = NewProxyTransport(10*time.Second, 60*time.Second)

func NewProxyTransport(connectTimeout, responseHeaderTimeout time.Duration) *http.Transport {
	return &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   connectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		DisableCompression:    true,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          1024,
		MaxIdleConnsPerHost:   256,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func RequestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func RetryableMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
