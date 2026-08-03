package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zhenya3000/pdf-generator/internal/pdf"
)

type fakeRenderer struct {
	render func(context.Context, pdf.Request) ([]byte, error)
	ready  func(context.Context) error
}

func (f fakeRenderer) Render(ctx context.Context, request pdf.Request) ([]byte, error) {
	if f.render == nil {
		return []byte("%PDF-1.4\n%%EOF"), nil
	}
	return f.render(ctx, request)
}

func (f fakeRenderer) Ready(ctx context.Context) error {
	if f.ready == nil {
		return nil
	}
	return f.ready(ctx)
}

func TestGeneratePDF(t *testing.T) {
	var received pdf.Request
	handler := testHandler(fakeRenderer{render: func(_ context.Context, request pdf.Request) ([]byte, error) {
		received = request
		return []byte("%PDF-1.4\n%%EOF"), nil
	}})

	recorder := perform(handler, http.MethodPost, "/v1/pdf", `{"html":"<h1>Hello</h1>","filename":"report","options":{"format":"Letter","landscape":true}}`, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, body %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("unexpected content type: %s", recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "report.pdf") {
		t.Fatalf("unexpected disposition: %s", recorder.Header().Get("Content-Disposition"))
	}
	if received.Options.Format != "LETTER" || !received.Options.Landscape {
		t.Fatalf("unexpected request: %+v", received)
	}
}

func TestGenerateRejectsInvalidRequests(t *testing.T) {
	handler := testHandler(fakeRenderer{})
	tests := []struct {
		name        string
		body        string
		contentType string
		status      int
	}{
		{name: "content type", body: `{}`, contentType: "text/plain", status: http.StatusUnsupportedMediaType},
		{name: "malformed", body: `{`, status: http.StatusBadRequest},
		{name: "trailing", body: `{"html":"ok"}{"extra":true}`, status: http.StatusBadRequest},
		{name: "unknown", body: `{"html":"ok","unknown":true}`, status: http.StatusBadRequest},
		{name: "empty", body: `{"html":"  "}`, status: http.StatusBadRequest},
		{name: "filename", body: `{"html":"ok","filename":"../secret"}`, status: http.StatusBadRequest},
		{name: "options", body: `{"html":"ok","options":{"scale":3}}`, status: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headers := map[string]string{}
			if test.contentType != "" {
				headers["Content-Type"] = test.contentType
			}
			recorder := perform(handler, http.MethodPost, "/v1/pdf", test.body, headers)
			if recorder.Code != test.status {
				t.Fatalf("status: got %d, want %d, body %s", recorder.Code, test.status, recorder.Body.String())
			}
			if recorder.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("unexpected content type: %s", recorder.Header().Get("Content-Type"))
			}
		})
	}
}

func TestGenerateRejectsLargeBody(t *testing.T) {
	handler := New(testConfig(fakeRenderer{}, func(cfg *Config) {
		cfg.MaxRequestBytes = 64
	}))
	body := `{"html":"` + strings.Repeat("x", 100) + `"}`
	recorder := perform(handler, http.MethodPost, "/v1/pdf", body, nil)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: got %d, body %s", recorder.Code, recorder.Body.String())
	}
}

func TestGenerateRequiresAPIKey(t *testing.T) {
	handler := New(testConfig(fakeRenderer{}, func(cfg *Config) {
		cfg.APIKey = "secret"
	}))
	unauthorized := perform(handler, http.MethodPost, "/v1/pdf", `{"html":"ok"}`, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", unauthorized.Code)
	}
	authorized := perform(handler, http.MethodPost, "/v1/pdf", `{"html":"ok"}`, map[string]string{"X-API-Key": "secret"})
	if authorized.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", authorized.Code)
	}
}

func TestGenerateTimesOut(t *testing.T) {
	handler := New(testConfig(fakeRenderer{render: func(ctx context.Context, _ pdf.Request) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}, func(cfg *Config) {
		cfg.RenderTimeout = 20 * time.Millisecond
	}))
	recorder := perform(handler, http.MethodPost, "/v1/pdf", `{"html":"ok"}`, nil)
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status: got %d, body %s", recorder.Code, recorder.Body.String())
	}
}

func TestGenerateQueueTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	handler := New(testConfig(fakeRenderer{render: func(ctx context.Context, _ pdf.Request) ([]byte, error) {
		once.Do(func() { close(started) })
		select {
		case <-release:
			return []byte("%PDF-1.4\n%%EOF"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}, func(cfg *Config) {
		cfg.MaxConcurrent = 1
		cfg.QueueTimeout = 20 * time.Millisecond
	}))

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstDone <- perform(handler, http.MethodPost, "/v1/pdf", `{"html":"first"}`, nil)
	}()
	<-started
	second := perform(handler, http.MethodPost, "/v1/pdf", `{"html":"second"}`, nil)
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, body %s", second.Code, second.Body.String())
	}
	close(release)
	if first := <-firstDone; first.Code != http.StatusOK {
		t.Fatalf("first status: got %d, body %s", first.Code, first.Body.String())
	}
}

func TestSystemEndpoints(t *testing.T) {
	handler := New(testConfig(fakeRenderer{}, func(cfg *Config) {
		cfg.Version = "1.2.3"
		cfg.Commit = "abc"
		cfg.BuildDate = "today"
	}))
	for _, path := range []string{"/healthz", "/readyz", "/version", "/metrics"} {
		recorder := perform(handler, http.MethodGet, path, "", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: got %d, body %s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestReadyFailure(t *testing.T) {
	handler := testHandler(fakeRenderer{ready: func(context.Context) error {
		return errors.New("offline")
	}})
	recorder := perform(handler, http.MethodGet, "/readyz", "", nil)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, body %s", recorder.Code, recorder.Body.String())
	}
}

func TestCORS(t *testing.T) {
	handler := New(testConfig(fakeRenderer{}, func(cfg *Config) {
		cfg.AllowedOrigins = []string{"https://app.example.com"}
	}))
	recorder := perform(handler, http.MethodOptions, "/v1/pdf", "", map[string]string{
		"Origin":                        "https://app.example.com",
		"Access-Control-Request-Method": "POST",
	})
	if recorder.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("unexpected CORS header: %s", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if recorder.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("credentials must not be enabled")
	}
}

func testHandler(renderer pdf.Renderer) http.Handler {
	return New(testConfig(renderer, nil))
}

func testConfig(renderer pdf.Renderer, change func(*Config)) Config {
	cfg := Config{
		Renderer:        renderer,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		MaxConcurrent:   2,
		QueueTimeout:    time.Second,
		RenderTimeout:   time.Second,
		ReadyTimeout:    time.Second,
		MaxRequestBytes: 1 << 20,
		MaxPDFBytes:     1 << 20,
		Version:         "dev",
		Commit:          "none",
		BuildDate:       "unknown",
	}
	if change != nil {
		change(&cfg)
	}
	return cfg
}

func perform(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
