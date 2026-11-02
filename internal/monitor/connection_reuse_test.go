package monitor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type drainTransport func(*http.Request) (*http.Response, error)

func (f drainTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type observedBody struct {
	io.Reader
	bytes  int
	closed bool
}

func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.bytes += n
	return n, err
}
func (b *observedBody) Close() error { b.closed = true; return nil }
func TestCheckerDrainsSmallResponsesAndBoundsLargeOnes(t *testing.T) {
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	for _, size := range []int{20, 100000} {
		body := &observedBody{Reader: strings.NewReader(strings.Repeat("x", size))}
		http.DefaultTransport = drainTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header), Request: r}, nil
		})
		if err := HTTPChecker(context.Background(), "http://health.test", time.Second); err != nil {
			t.Fatal(err)
		}
		want := min(size, 64*1024)
		if body.bytes != want || !body.closed {
			t.Fatalf("size=%d read=%d closed=%v", size, body.bytes, body.closed)
		}
	}
}
