package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	clearEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":3001" || cfg.MaxConcurrent != 2 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.OpenAPIPath != "api/openapi.yaml" {
		t.Fatalf("unexpected OpenAPI path: %s", cfg.OpenAPIPath)
	}
	if cfg.MaxRequestBytes != 2<<20 || cfg.MaxPDFBytes != 20<<20 {
		t.Fatalf("unexpected limits: %+v", cfg)
	}
	if cfg.AllowJavaScript || cfg.ChromeNoSandbox {
		t.Fatalf("unsafe defaults: %+v", cfg)
	}
}

func TestLoadCustomValues(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("PORT", "8080")
	t.Setenv("MAX_CONCURRENT_RENDERS", "4")
	t.Setenv("QUEUE_TIMEOUT", "2s")
	t.Setenv("OPENAPI_PATH", "/etc/go-pdf-renderer/openapi.yaml")
	t.Setenv("ALLOWED_ASSET_HOSTS", "CDN.EXAMPLE.COM, cdn.example.com, fonts.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":8080" || cfg.MaxConcurrent != 4 || cfg.QueueTimeout != 2*time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.OpenAPIPath != "/etc/go-pdf-renderer/openapi.yaml" {
		t.Fatalf("unexpected OpenAPI path: %s", cfg.OpenAPIPath)
	}
	if len(cfg.AllowedAssetHosts) != 2 || cfg.AllowedAssetHosts[0] != "cdn.example.com" {
		t.Fatalf("unexpected hosts: %v", cfg.AllowedAssetHosts)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "ALLOW_JAVASCRIPT", value: "sometimes"},
		{name: "MAX_CONCURRENT_RENDERS", value: "0"},
		{name: "QUEUE_TIMEOUT", value: "never"},
		{name: "MAX_REQUEST_BYTES", value: "12"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearEnvironment(t)
			t.Setenv(test.name, test.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadRejectsShortWriteTimeout(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("RENDER_TIMEOUT", "30s")
	t.Setenv("WRITE_TIMEOUT", "30s")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func clearEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"PORT", "ADDRESS", "CHROME_PATH", "CHROME_NO_SANDBOX", "ALLOW_JAVASCRIPT",
		"ALLOWED_ASSET_HOSTS", "ALLOWED_ORIGINS", "API_KEY", "MAX_CONCURRENT_RENDERS",
		"MAX_REQUEST_BYTES", "MAX_PDF_BYTES", "QUEUE_TIMEOUT", "RENDER_TIMEOUT",
		"READY_TIMEOUT", "READ_HEADER_TIMEOUT", "READ_TIMEOUT", "WRITE_TIMEOUT",
		"IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT", "OPENAPI_PATH",
	} {
		t.Setenv(name, "")
	}
}
