package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		name          string
		origin        string
		forwardedHost string
		fetchSite     string
		want          bool
	}{
		{name: "missing origin", want: true},
		{name: "direct", origin: "http://terminal.test", want: true},
		{name: "forwarded host", origin: "https://terminal.example", forwardedHost: "terminal.example", want: true},
		{name: "forwarded host chain", origin: "https://terminal.example", forwardedHost: "terminal.example, proxy.internal", want: true},
		{name: "browser same origin through proxy", origin: "https://terminal.example", fetchSite: "same-origin", want: true},
		{name: "cross site", origin: "https://evil.test", fetchSite: "cross-site", want: false},
		{name: "opaque origin", origin: "null", want: false},
		{name: "untrusted forwarded host", origin: "https://evil.test", forwardedHost: "terminal.example", want: false},
		{name: "malformed origin", origin: "http://%", fetchSite: "same-origin", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://terminal.test/_herdr/login", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.forwardedHost != "" {
				request.Header.Set("X-Forwarded-Host", test.forwardedHost)
			}
			if test.fetchSite != "" {
				request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			}
			if got := sameOrigin(request); got != test.want {
				t.Fatalf("sameOrigin() = %v, want %v", got, test.want)
			}
		})
	}
}
