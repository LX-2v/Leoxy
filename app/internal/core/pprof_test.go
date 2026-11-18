package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterPprofRestrictsRemoteClients(t *testing.T) {
	mux := http.NewServeMux()
	registerPprof(mux, nil)

	paths := []string{
		"/debug/pprof/",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.RemoteAddr = "203.0.113.10:12345"
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("loopback status = %d, want %d", res.Code, http.StatusOK)
	}
}
