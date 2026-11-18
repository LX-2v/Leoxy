package core

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elrefai99/Leoxy/app/internal/file"
)

func newStreamTestServer(t *testing.T, upstream http.Handler, readTimeout, writeTimeout time.Duration) *httptest.Server {
	t.Helper()
	backend := httptest.NewServer(upstream)
	t.Cleanup(backend.Close)
	mux := http.NewServeMux()
	registerHTTP(mux, file.Security{}, file.Upstream{
		Name:      "stream-test",
		Path:      "/",
		ServerURL: backend.URL,
		Security:  file.Security{MaxBody: 1},
	}, nil)
	server := httptest.NewUnstartedServer(mux)
	server.Config.ReadTimeout = readTimeout
	server.Config.WriteTimeout = writeTimeout
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func TestRegisterHTTPIncompleteKnownLengthBodyHonorsReadTimeout(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
	}{
		{name: "plain"},
		{name: "arbitrary upgrade", headers: []string{"Upgrade: websocket"}},
		{name: "SSE accept", headers: []string{"Accept: text/event-stream"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newStreamTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(http.StatusOK)
			}), 150*time.Millisecond, time.Second)
			conn := dialStreamTestServer(t, server)
			defer conn.Close()
			request := "POST / HTTP/1.1\r\nHost: test\r\nContent-Length: 100\r\nConnection: close\r\n"
			request += strings.Join(test.headers, "\r\n")
			if len(test.headers) > 0 {
				request += "\r\n"
			}
			request += "\r\nA"
			if _, err := io.WriteString(conn, request); err != nil {
				t.Fatal(err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			_, err := bufio.NewReader(conn).ReadByte()
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				t.Fatalf("connection stayed open beyond read timeout: %v", err)
			}
		})
	}
}

func TestRegisterHTTPIncompleteChunkedBodyHonorsReadTimeout(t *testing.T) {
	server := newStreamTestServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("upstream received an incomplete chunked body")
	}), 150*time.Millisecond, time.Second)
	conn := dialStreamTestServer(t, server)
	defer conn.Close()
	_, err := io.WriteString(conn, "POST / HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\nAccept: text/event-stream\r\nConnection: close\r\n\r\n1\r\nA")
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, err = bufio.NewReader(conn).ReadByte()
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatalf("connection stayed open beyond read timeout: %v", err)
	}
}

func TestRegisterHTTPSSEContinuesBeyondWriteTimeout(t *testing.T) {
	server := newStreamTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(150 * time.Millisecond)
		_, _ = io.WriteString(w, "data: second\n\n")
		w.(http.Flusher).Flush()
	}), time.Second, 50*time.Millisecond)
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "text/event-stream")
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "data: second") {
		t.Fatalf("stream body = %q, want second event after write timeout", body)
	}
}

func TestRegisterHTTPSuccessfulWebSocketUpgradeOutlivesWriteTimeout(t *testing.T) {
	server := newStreamTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("backend response writer does not support hijacking")
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		key := r.Header.Get("Sec-WebSocket-Key")
		accept := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
		if key != "dGhlIHNhbXBsZSBub25jZQ==" {
			t.Errorf("WebSocket key = %q", key)
		}
		_, _ = fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
		if err := rw.Flush(); err != nil {
			t.Error(err)
			return
		}
		time.Sleep(150 * time.Millisecond)
		_, _ = conn.Write([]byte{0x81, 5, 'a', 'l', 'i', 'v', 'e'})
	}), time.Second, 50*time.Millisecond)
	conn := dialStreamTestServer(t, server)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "HTTP/1.1 101 Switching Protocols" {
		t.Fatalf("WebSocket status = %q", strings.TrimSpace(status))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	frame := make([]byte, 7)
	if _, err := io.ReadFull(reader, frame); err != nil {
		t.Fatal(err)
	}
	if string(frame[2:]) != "alive" {
		t.Fatalf("WebSocket frame payload = %q, want alive", frame[2:])
	}
}

func TestRegisterHTTPRejectedWebSocketUpgradeReturnsUpstreamResponse(t *testing.T) {
	server := newStreamTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upgrade rejected", http.StatusBadRequest)
	}), time.Second, time.Second)
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
}

func TestRegisterHTTPMaxBodyLimitWithStreamingHeaders(t *testing.T) {
	var upstreamRequests atomic.Int64
	server := newStreamTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}), time.Second, time.Second)
	request, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(strings.Repeat("x", 1024*1024+1)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "text/event-stream")
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusRequestEntityTooLarge)
	}
	if got := upstreamRequests.Load(); got != 0 {
		t.Fatalf("upstream received %d requests, want 0", got)
	}
}

func dialStreamTestServer(t *testing.T, server *httptest.Server) net.Conn {
	t.Helper()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", target.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
