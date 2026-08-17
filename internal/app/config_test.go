package app

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	environment := map[string]string{
		usernameEnv: "alice",
		passwordEnv: "correct horse battery staple",
	}
	config, err := ParseConfig(
		[]string{"--listen", "127.0.0.1:9000", "--max-clients", "4", "--", "--session", "work"},
		func(key string) string { return environment[key] },
		func() (string, error) { return "/workspace", nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if config.Listen != "127.0.0.1:9000" || config.MaxClients != 4 {
		t.Fatalf("unexpected config: %#v", config)
	}
	if !reflect.DeepEqual(config.HerdrArgs, []string{"--session", "work"}) {
		t.Fatalf("unexpected Herdr args: %#v", config.HerdrArgs)
	}
}

func TestParseConfigRequiresCredentials(t *testing.T) {
	_, err := ParseConfig(nil, func(string) string { return "" }, func() (string, error) {
		return "/workspace", nil
	})
	if err == nil || !strings.Contains(err.Error(), usernameEnv) {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

func TestNativeArgs(t *testing.T) {
	config := Config{
		Listen:     "127.0.0.1:7681",
		Ttyd:       "ttyd",
		Herdr:      "herdr",
		CWD:        "/workspace",
		MaxClients: 3,
		Username:   "alice",
		Password:   "secret",
		HerdrArgs:  []string{"--session", "work"},
	}
	want := []string{
		"--debug", "3",
		"--interface", "127.0.0.1",
		"--port", "7681",
		"--writable",
		"--check-origin",
		"--max-clients", "3",
		"--credential", "alice:secret",
		"--cwd", "/workspace",
		"--terminal-type", "xterm-256color",
		"herdr", "--session", "work",
	}
	if got := config.NativeArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("NativeArgs() = %#v, want %#v", got, want)
	}
}
