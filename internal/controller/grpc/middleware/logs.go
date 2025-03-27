package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		startTime := time.Now()
		next.ServeHTTP(lrw, r)
		duration := time.Since(startTime)
		log.Printf("%-6s %-20s %-3d       (%s)", r.Method, r.RequestURI, lrw.statusCode, formatDuration(duration))
	})
}

func formatDuration(d time.Duration) string {
	ms := float64(d.Nanoseconds()) / 1e6
	if ms < 1000 {
		return fmt.Sprintf("%.1fms", ms)
	}
	sec := ms / 1000
	return fmt.Sprintf("%.1fs", sec)
}
