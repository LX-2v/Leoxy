package watcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

func Notify(client *http.Client, target string, report Report) error {
	client = noWebhookRedirects(client)
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	var notifyErr error
	if response.StatusCode >= http.StatusMultipleChoices {
		notifyErr = fmt.Errorf("webhook returned http %d", response.StatusCode)
	}
	return errors.Join(notifyErr, response.Body.Close())
}

func noWebhookRedirects(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &copy
}

func webhookClient(client *http.Client) *http.Client {
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		transport, _ = http.DefaultTransport.(*http.Transport)
	}
	transport = transport.Clone()
	transport.Proxy = nil
	transport.DialTLSContext = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !publicWebhookIP(ip) {
				return nil, fmt.Errorf("webhook destination resolves to a non-public address")
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("webhook destination did not resolve")
		}
		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
	copy := *client
	copy.Transport = transport
	return noWebhookRedirects(&copy)
}

func publicWebhookIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsMulticast() && !ip.IsUnspecified()
}
