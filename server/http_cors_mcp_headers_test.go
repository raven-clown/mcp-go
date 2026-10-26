package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamableHTTP_CORS_DefaultHeadersAllowMCPProtocolHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
	}{
		{name: "legacy after initialize", requested: "content-type, mcp-session-id, mcp-protocol-version"},
		{name: "legacy resume", requested: "content-type, mcp-session-id, mcp-protocol-version, last-event-id"},
		{name: "2026-07-28 tools/call", requested: "content-type, mcp-protocol-version, mcp-method, mcp-name"},
		{name: "2026-07-28 tools/list", requested: "content-type, mcp-protocol-version, mcp-method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := NewStreamableHTTPServer(NewMCPServer("test", "1.0.0"),
				WithStreamableHTTPCORS(WithCORSAllowedOrigins("https://example.com")),
			)
			ts := httptest.NewServer(srv)
			t.Cleanup(ts.Close)

			req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, ts.URL, nil)
			require.NoError(t, err)
			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			req.Header.Set("Access-Control-Request-Headers", tt.requested)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			t.Cleanup(func() { _ = resp.Body.Close() })

			require.Equal(t, http.StatusNoContent, resp.StatusCode)
			allowed := map[string]bool{}
			for h := range strings.SplitSeq(resp.Header.Get("Access-Control-Allow-Headers"), ",") {
				allowed[strings.ToLower(strings.TrimSpace(h))] = true
			}
			for h := range strings.SplitSeq(tt.requested, ",") {
				assert.True(t, allowed[strings.TrimSpace(h)], "preflight does not allow %q", strings.TrimSpace(h))
			}
		})
	}
}

func TestStreamableHTTP_CORS_ConfiguredHeadersReplaceDefaults(t *testing.T) {
	t.Parallel()

	srv := NewStreamableHTTPServer(NewMCPServer("test", "1.0.0"),
		WithStreamableHTTPCORS(
			WithCORSAllowedOrigins("https://example.com"),
			WithCORSAllowedHeaders("Content-Type", "Mcp-Param-Region"),
		),
	)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, ts.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, "Content-Type, Mcp-Param-Region", resp.Header.Get("Access-Control-Allow-Headers"))
}
