package pdf

import "testing"

func TestAllowURL(t *testing.T) {
	renderer := &ChromeRenderer{allowedHosts: map[string]struct{}{
		"cdn.example.com": {},
		"8.8.8.8":         {},
		"127.0.0.1":       {},
		"10.0.0.1":        {},
		"192.168.1.10":    {},
		"169.254.169.254": {},
	}}
	tests := []struct {
		url     string
		allowed bool
	}{
		{url: "about:blank", allowed: true},
		{url: "data:image/png;base64,AA==", allowed: true},
		{url: "https://cdn.example.com/font.woff2", allowed: true},
		{url: "https://CDN.EXAMPLE.COM/image.png", allowed: true},
		{url: "https://example.com/image.png", allowed: false},
		{url: "http://127.0.0.1/private", allowed: false},
		{url: "http://10.0.0.1/private", allowed: false},
		{url: "http://192.168.1.10/private", allowed: false},
		{url: "http://169.254.169.254/latest/meta-data", allowed: false},
		{url: "file:///etc/passwd", allowed: false},
		{url: "ftp://cdn.example.com/file", allowed: false},
	}
	for _, test := range tests {
		if actual := renderer.allowURL(test.url); actual != test.allowed {
			t.Fatalf("%s: got %t, want %t", test.url, actual, test.allowed)
		}
	}
}
