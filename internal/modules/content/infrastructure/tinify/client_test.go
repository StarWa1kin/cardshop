package tinify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/content/contract"
)

const testKey = "mock-key"

func checkAuth(t *testing.T, r *http.Request) {
	t.Helper()
	user, pass, ok := r.BasicAuth()
	if !ok || user != "api" || pass != testKey {
		t.Errorf("basic auth mismatch: ok=%v user=%q pass=%q", ok, user, pass)
	}
}

func newTestClient(t *testing.T, server *httptest.Server, timeout time.Duration) *Client {
	t.Helper()
	client, err := New(Options{Endpoint: server.URL + "/shrink", Timeout: timeout})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func TestCompressUploadsAndDownloadsResult(t *testing.T) {
	var uploaded string
	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/shrink", func(w http.ResponseWriter, r *http.Request) {
		checkAuth(t, r)
		if r.Method != http.MethodPost {
			t.Errorf("shrink method %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		uploaded = string(body)
		w.Header().Set("Location", server.URL+"/output/abc")
		w.Header().Set("Compression-Count", "42")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"input":{"size":9,"type":"image/png"},"output":{"size":5,"type":"image/png","url":"`+server.URL+`/output/abc"}}`)
	})
	mux.HandleFunc("/output/abc", func(w http.ResponseWriter, r *http.Request) {
		checkAuth(t, r)
		_, _ = io.WriteString(w, "small")
	})
	server = httptest.NewServer(mux)
	defer server.Close()

	result, err := newTestClient(t, server, time.Second).Compress(context.Background(), testKey, []byte("big-image"))
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if uploaded != "big-image" {
		t.Fatalf("uploaded body mismatch: %q", uploaded)
	}
	if string(result.Data) != "small" || result.MimeType != "image/png" || result.CompressionCount != 42 {
		t.Fatalf("result mismatch: %#v", result)
	}
}

func TestCompressMapsHTTPErrors(t *testing.T) {
	tests := []struct {
		status int
		body   string
		want   error
	}{
		{http.StatusUnauthorized, `{"error":"Unauthorized","message":"Credentials are invalid."}`, contract.ErrCompressUnauthorized},
		{http.StatusTooManyRequests, `{"error":"TooManyRequests","message":"Your monthly limit has been exceeded"}`, contract.ErrCompressQuotaExceeded},
		{http.StatusUnsupportedMediaType, `{"error":"Unsupported media type","message":"File type is not supported."}`, contract.ErrCompressUnsupportedFormat},
		{http.StatusServiceUnavailable, `not json`, contract.ErrCompressFailed},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			_, err := newTestClient(t, server, time.Second).Compress(context.Background(), testKey, []byte("x"))
			if !errors.Is(err, test.want) {
				t.Fatalf("want %v got %v", test.want, err)
			}
		})
	}
}

func TestCompressTimesOut(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	_, err := newTestClient(t, server, 50*time.Millisecond).Compress(context.Background(), testKey, []byte("x"))
	if !errors.Is(err, contract.ErrCompressTimeout) {
		t.Fatalf("want timeout got %v", err)
	}
}

func TestCompressRefusesToSendKeyToForeignOutputHost(t *testing.T) {
	foreignHit := false
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignHit = true
	}))
	defer foreign.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", foreign.URL+"/output/x")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"output":{"type":"image/png"}}`)
	}))
	defer server.Close()

	_, err := newTestClient(t, server, time.Second).Compress(context.Background(), testKey, []byte("x"))
	if !errors.Is(err, contract.ErrCompressFailed) || !strings.Contains(err.Error(), "unexpected output host") {
		t.Fatalf("want foreign host rejection got %v", err)
	}
	if foreignHit {
		t.Fatal("client must not contact a foreign host")
	}
}

func TestCompressRejectsEmptyKeyWithoutCallingAPI(t *testing.T) {
	hit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer server.Close()
	if _, err := newTestClient(t, server, time.Second).Compress(context.Background(), " ", []byte("x")); !errors.Is(err, contract.ErrCompressKeyMissing) {
		t.Fatalf("want key missing got %v", err)
	}
	if hit {
		t.Fatal("API must not be called without a key")
	}
}
