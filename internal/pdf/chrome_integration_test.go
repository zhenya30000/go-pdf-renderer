package pdf

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestChromeRendererIntegration(t *testing.T) {
	if os.Getenv("PDF_GENERATOR_INTEGRATION") != "1" {
		t.Skip("set PDF_GENERATOR_INTEGRATION=1 to run")
	}
	for _, test := range []struct {
		name         string
		allowedHosts []string
	}{
		{name: "blocked remote assets"},
		{name: "allowlisted remote assets", allowedHosts: []string{"cdn.example.com"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			renderer, err := NewChromeRenderer(context.Background(), ChromeConfig{
				ExecutablePath:    os.Getenv("CHROME_PATH"),
				NoSandbox:         os.Getenv("CHROME_NO_SANDBOX") == "true",
				AllowedAssetHosts: test.allowedHosts,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer renderer.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			output, err := renderer.Render(ctx, Request{
				HTML: `<html><head><meta charset="utf-8"><style>@page { size: A4 }</style></head><body><h1>Привет, PDF</h1></body></html>`,
				Options: Options{
					WaitUntil: "none",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(output) < 1000 || !strings.HasPrefix(string(output), "%PDF-") {
				t.Fatalf("unexpected output: %d bytes", len(output))
			}
		})
	}
}
