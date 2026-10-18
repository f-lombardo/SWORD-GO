package web

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppAbsoluteURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		request func() *http.Request
		path    string
		want    string
	}{
		{
			name:    "uses configured base URL and preserves query parameters",
			baseURL: "https://console.example.test/sword",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "http://internal.example.test/servers", nil)
			},
			path: "/public/servers/42/scripts/provision?token=secret",
			want: "https://console.example.test/sword/public/servers/42/scripts/provision?token=secret",
		},
		{
			name: "uses forwarded public URL",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://internal.example.test/servers", nil)
				req.Header.Set("X-Forwarded-Proto", "https")
				req.Header.Set("X-Forwarded-Host", "console.example.test")
				return req
			},
			path: "/public/sites/4/scripts/install?token=secret",
			want: "https://console.example.test/public/sites/4/scripts/install?token=secret",
		},
		{
			name: "uses HTTPS for a TLS request",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://console.example.test/servers", nil)
				req.TLS = &tls.ConnectionState{}
				return req
			},
			path: "/public/servers/8/callbacks/provision?signature=value",
			want: "https://console.example.test/public/servers/8/callbacks/provision?signature=value",
		},
		{
			name: "uses HTTP and request host by default",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "http://localhost:8088/servers", nil)
			},
			path: "/public/servers/8/scripts/provision",
			want: "http://localhost:8088/public/servers/8/scripts/provision",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &App{baseURL: test.baseURL}
			if got := app.absoluteURL(test.request(), test.path); got != test.want {
				t.Errorf("absoluteURL() = %q, want %q", got, test.want)
			}
		})
	}
}
