package app

import (
	"reflect"
	"testing"
)

func TestBrowserURLUsesReachableLoopbackForWildcardListener(t *testing.T) {
	config := Config{Listen: "0.0.0.0:7681"}
	if got, want := config.browserURL(), "http://127.0.0.1:7681"; got != want {
		t.Fatalf("browserURL() = %q, want %q", got, want)
	}
}

func TestBrowserInvocations(t *testing.T) {
	rawURL := "http://127.0.0.1:7681"
	tests := []struct {
		name string
		goos string
		wsl  bool
		want []browserInvocation
	}{
		{
			name: "linux",
			goos: "linux",
			want: []browserInvocation{{program: "xdg-open", args: []string{rawURL}}},
		},
		{
			name: "wsl",
			goos: "linux",
			wsl:  true,
			want: []browserInvocation{
				{program: "rundll32.exe", args: []string{"url.dll,FileProtocolHandler", rawURL}},
				{program: "cmd.exe", args: []string{"/c", "start", "", rawURL}},
				{program: "wslview", args: []string{rawURL}},
				{program: "xdg-open", args: []string{rawURL}},
			},
		},
		{
			name: "macOS",
			goos: "darwin",
			want: []browserInvocation{{program: "open", args: []string{rawURL}}},
		},
		{
			name: "Windows",
			goos: "windows",
			want: []browserInvocation{{program: "rundll32.exe", args: []string{"url.dll,FileProtocolHandler", rawURL}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := browserInvocations(test.goos, test.wsl, rawURL); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("browserInvocations() = %#v, want %#v", got, test.want)
			}
		})
	}
}
