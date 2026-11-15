package proxy

import (
	"net/http"
	"strings"
	"time"
)

func ExtendStreaming(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.Header.Get("Upgrade") != "" {
			controller := http.NewResponseController(w)
			_ = controller.SetWriteDeadline(time.Time{})
			_ = controller.SetReadDeadline(time.Time{})
		}
		next.ServeHTTP(w, r)
	})
}
