package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPChecker_UpOn2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := HTTPChecker(context.Background(), srv.URL, time.Second); err != nil {
		t.Fatalf("expected no error for 200 response, got %v", err)
	}
}

func TestHTTPChecker_DownOn5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := HTTPChecker(context.Background(), srv.URL, time.Second); err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestHTTPChecker_UpOn404(t *testing.T) {
	// A 404 means the server answered -- that's "up", just a missing route.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := HTTPChecker(context.Background(), srv.URL, time.Second); err != nil {
		t.Fatalf("expected no error for 404 response, got %v", err)
	}
}

func TestHTTPChecker_DownOnUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // now nothing is listening

	if err := HTTPChecker(context.Background(), url, time.Second); err == nil {
		t.Fatal("expected an error for an unreachable server, got nil")
	}
}
