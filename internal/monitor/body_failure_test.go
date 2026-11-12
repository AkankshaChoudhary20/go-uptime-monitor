package monitor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type failedBody struct{ closed bool }

func (b *failedBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (b *failedBody) Close() error             { b.closed = true; return nil }

type bodyTransport struct{ body *failedBody }

func (t bodyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: t.body}, nil
}
func TestHTTPCheckerReportsBodyReadFailure(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	body := &failedBody{}
	http.DefaultTransport = bodyTransport{body}
	err := HTTPChecker(context.Background(), "http://example.test", time.Second)
	if err == nil || !strings.Contains(err.Error(), "reading response") {
		t.Fatalf("got %v", err)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}
