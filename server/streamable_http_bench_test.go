package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkStreamableHTTPToolCall(b *testing.B) {
	for _, size := range []int{16, 64 << 10, 1 << 20} {
		h := NewStreamableHTTPServer(benchServer(), WithStateLess(true), WithDisableLocalhostProtection(true))
		body := []byte(callMsg(size))
		b.Run(fmt.Sprintf("args=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					b.Fatalf("status %d: %s", rec.Code, rec.Body.String())
				}
				_, _ = io.Copy(io.Discard, rec.Body)
			}
		})
	}
}
