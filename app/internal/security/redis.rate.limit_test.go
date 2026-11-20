package security

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRedisTokenBucketRateLimit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close fake Redis listener: %v", err)
		}
	})
	var requests atomic.Int64
	errors := make(chan error, 1)
	go serveFakeRedis(listener, &requests, errors)

	handler := RedisTokenBucketRateLimit(16, listener.Addr().String(), "", 0, 60, 1, "leoxy:test", false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	if got, _ := statusOf(handler, "192.0.2.1:1000"); got != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", got, http.StatusNoContent)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.1:1000"
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("second response = %d Retry-After=%q, want 429 and 60", response.Code, response.Header().Get("Retry-After"))
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("Redis calls = %d, want 2", got)
	}
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
	}
}

func TestRedisTLSConnection(t *testing.T) {
	certificateServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer certificateServer.Close()
	parsed, err := url.Parse(certificateServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close test Redis listener: %v", err)
		}
	}()
	serverTLS := tls.NewListener(listener, &tls.Config{Certificates: certificateServer.TLS.Certificates, MinVersion: tls.VersionTLS12})
	commands := make(chan []string, 2)
	serverDone := make(chan error, 1)
	go func() {
		conn, acceptErr := serverTLS.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer func() {
			serverDone <- conn.Close()
		}()
		reader := bufio.NewReader(conn)
		for i := 0; i < 2; i++ {
			args, readErr := readRedisCommand(reader)
			if readErr != nil {
				return
			}
			commands <- args
			if args[0] == "AUTH" {
				_, _ = io.WriteString(conn, "+OK\r\n")
			} else {
				_, _ = io.WriteString(conn, ":1\r\n")
			}
		}
	}()
	rootCert, err := x509.ParseCertificate(certificateServer.TLS.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(rootCert)
	limiter := newRedisLimiter(1, "localhost:"+strings.Split(listener.Addr().String(), ":")[1], "redis-secret", 0, true)
	limiter.tlsConfig = &tls.Config{RootCAs: roots, ServerName: parsed.Hostname()}
	if allowed, err := limiter.AllowTokenBucket("test", 60, 1); err != nil || !allowed {
		t.Fatalf("AllowTokenBucket() = %v, %v; want true, nil", allowed, err)
	}
	if got := <-commands; len(got) != 2 || got[0] != "AUTH" || got[1] != "redis-secret" {
		t.Fatalf("first Redis command = %q, want AUTH with configured password", got)
	}
	if got := <-commands; len(got) == 0 || got[0] != "EVAL" {
		t.Fatalf("second Redis command = %q, want EVAL", got)
	}
	if err := <-serverDone; err != nil {
		t.Errorf("close test Redis TLS connection: %v", err)
	}
}

func TestRedisTokenBucketFailOpenUsesLocalBucket(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	handler := RedisTokenBucketRateLimit(16, address, "", 0, 60, 1, "leoxy:test", true, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	if got, _ := statusOf(handler, "192.0.2.1:1000"); got != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", got, http.StatusNoContent)
	}
	if got, _ := statusOf(handler, "192.0.2.1:1000"); got != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func serveFakeRedis(listener net.Listener, requests *atomic.Int64, errors chan<- error) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer func() {
				if err := conn.Close(); err != nil {
					select {
					case errors <- fmt.Errorf("close fake Redis connection: %w", err):
					default:
					}
				}
			}()
			reader := bufio.NewReader(conn)
			for {
				args, err := readRedisCommand(reader)
				if err != nil {
					if err != io.EOF {
						select {
						case errors <- err:
						default:
						}
					}
					return
				}
				if len(args) != 6 || args[0] != "EVAL" || args[2] != "1" || args[3] != "leoxy:test:token-bucket" || args[4] != "60" || args[5] != "1" || !strings.Contains(args[1], "HMGET") {
					select {
					case errors <- fmt.Errorf("unexpected Redis command: %q", args):
					default:
					}
					_, _ = io.WriteString(conn, "-ERR unexpected command\r\n")
					return
				}
				result := int64(0)
				if requests.Add(1) == 1 {
					result = 1
				}
				if _, err := fmt.Fprintf(conn, ":%d\r\n", result); err != nil {
					return
				}
			}
		}()
	}
}

func readRedisCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 || line[0] != '*' {
		return nil, fmt.Errorf("invalid RESP command header %q", line)
	}
	count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, err
	}
	args := make([]string, count)
	for i := range args {
		line, err = reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if len(line) < 3 || line[0] != '$' {
			return nil, fmt.Errorf("invalid RESP bulk header %q", line)
		}
		size, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return nil, err
		}
		data := make([]byte, size+2)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		args[i] = string(data[:size])
	}
	return args, nil
}
