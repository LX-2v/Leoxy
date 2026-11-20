package watcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestNotifyPostsReport(t *testing.T) {
	var received Report
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := Notify(server.Client(), server.URL, Report{Project: "demo", MemoryBytes: 42}); err != nil {
		t.Fatal(err)
	}
	if received.Project != "demo" || received.MemoryBytes != 42 {
		t.Fatalf("unexpected payload: %+v", received)
	}
}

func TestNotifyReportsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := Notify(server.Client(), server.URL, Report{}); err == nil {
		t.Fatal("expected error for http 500")
	}
}

func TestNotifyDoesNotFollowRedirects(t *testing.T) {
	privateRequests := 0
	private := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		privateRequests++
	}))
	defer private.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, private.URL, http.StatusFound)
	}))
	defer redirect.Close()

	if err := Notify(redirect.Client(), redirect.URL, Report{}); err == nil {
		t.Fatal("expected error for redirect response")
	}
	if privateRequests != 0 {
		t.Fatal("redirect destination received a request")
	}
}

func TestWebhookClientRejectsPrivateDestination(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()

	if err := Notify(webhookClient(server.Client()), server.URL, Report{}); err == nil {
		t.Fatal("expected private destination to be rejected")
	}
	if requests != 0 {
		t.Fatal("private destination received a request")
	}
}

func TestPublicWebhookIP(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1"} {
		if publicWebhookIP(netip.MustParseAddr(address)) {
			t.Errorf("publicWebhookIP(%s) = true", address)
		}
	}
	if !publicWebhookIP(netip.MustParseAddr("8.8.8.8")) {
		t.Error("publicWebhookIP(8.8.8.8) = false")
	}
}
