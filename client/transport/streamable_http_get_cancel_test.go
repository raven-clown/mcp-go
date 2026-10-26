package transport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ctxBoundBody struct {
	data      io.Reader
	reqCtx    context.Context
	blockRead bool
}

func (b *ctxBoundBody) Read(p []byte) (int, error) {
	n, err := b.data.Read(p)
	if n > 0 || !b.blockRead {
		return n, err
	}
	<-b.reqCtx.Done()
	return 0, b.reqCtx.Err()
}

func (b *ctxBoundBody) Close() error {
	<-b.reqCtx.Done()
	return nil
}

type getErrorTransport struct {
	blockRead bool
}

func (rt getErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": {"text/plain"}},
		Body: &ctxBoundBody{
			data:      strings.NewReader("boom"),
			reqCtx:    req.Context(),
			blockRead: rt.blockRead,
		},
		Request: req,
	}, nil
}

func startGETConnection(t *testing.T, rt http.RoundTripper) (*StreamableHTTP, <-chan error) {
	t.Helper()
	tr, err := NewStreamableHTTP("http://mock-get-server/mcp",
		WithHTTPBasicClient(&http.Client{Transport: rt}),
	)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := tr.createGETConnectionToServer(t.Context())
		done <- err
	}()
	return tr, done
}

func TestStreamableHTTP_GETErrorCancelsBeforeBodyClose(t *testing.T) {
	tr, done := startGETConnection(t, getErrorTransport{})
	t.Cleanup(func() { _ = tr.Close() })

	select {
	case err := <-done:
		require.ErrorContains(t, err, "request failed with status 500: boom")
	case <-time.After(2 * time.Second):
		t.Fatal("createGETConnectionToServer stalled closing the GET error response body")
	}
}

func TestStreamableHTTP_GETErrorBodyUnblocksOnClientClose(t *testing.T) {
	tr, done := startGETConnection(t, getErrorTransport{blockRead: true})

	select {
	case err := <-done:
		t.Fatalf("createGETConnectionToServer returned before Close: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	closed := make(chan struct{})
	go func() {
		_ = tr.Close()
		close(closed)
	}()

	select {
	case err := <-done:
		require.ErrorContains(t, err, "request failed with status 500")
	case <-time.After(2 * time.Second):
		t.Fatal("createGETConnectionToServer did not return after the client was closed")
	}

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return")
	}
}
