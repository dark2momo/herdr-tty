package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
	if config.AuthMode != "form" || config.SessionTTL != 7*24*time.Hour || config.OpenBrowser {
		t.Fatalf("unexpected auth defaults: %#v", config)
	}
	if !reflect.DeepEqual(config.HerdrArgs, []string{"--session", "work"}) {
		t.Fatalf("unexpected Herdr args: %#v", config.HerdrArgs)
	}
}

func TestBackendArgsDoNotContainCredentials(t *testing.T) {
	config := Config{
		Herdr:      "herdr",
		CWD:        "/workspace",
		MaxClients: 3,
		Username:   "alice",
		Password:   "secret",
	}
	joined := strings.Join(config.BackendArgs(17682), " ")
	if strings.Contains(joined, config.Username) || strings.Contains(joined, config.Password) {
		t.Fatalf("backend args contain credentials: %q", joined)
	}
}

func TestParseConfigDefaultsToLocalAndOpensBrowser(t *testing.T) {
	config, err := ParseConfig(nil, func(string) string { return "" }, func() (string, error) {
		return "/workspace", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.AuthMode != "local" || !config.OpenBrowser || config.Listen != "127.0.0.1:7681" {
		t.Fatalf("unexpected local defaults: %#v", config)
	}
}

func TestParseConfigRequiresCredentialsForFormAuth(t *testing.T) {
	_, err := ParseConfig([]string{"--auth", "form"}, func(string) string { return "" }, func() (string, error) {
		return "/workspace", nil
	})
	if err == nil || !strings.Contains(err.Error(), usernameEnv) {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

func TestParseConfigLocalAuthIgnoresCredentialEnvironment(t *testing.T) {
	config, err := ParseConfig(
		[]string{"--auth", "local", "--no-open"},
		func(key string) string {
			if key == usernameEnv {
				return "left-over-value"
			}
			return ""
		},
		func() (string, error) { return "/workspace", nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if config.AuthMode != "local" || config.OpenBrowser || config.Username != "" || config.Password != "" {
		t.Fatalf("unexpected local config: %#v", config)
	}
}

func TestParseConfigRejectsLocalAuthOnNonLoopback(t *testing.T) {
	_, err := ParseConfig([]string{"--listen", "0.0.0.0:7681"}, func(string) string { return "" }, func() (string, error) {
		return "/workspace", nil
	})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback error, got %v", err)
	}
}

func TestParseConfigFileAndCLIOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "herdr-web.json")
	content := `{
  "listen": "127.0.0.1:9000",
  "cwd": "/configured",
  "max_clients": 2,
  "auth": "local",
  "session_ttl": "24h",
  "open_browser": true,
  "herdr_args": ["--session", "configured"]
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(
		[]string{"--config", path, "--listen", "127.0.0.1:9001", "--no-open", "--", "--session", "cli"},
		func(string) string { return "" },
		func() (string, error) { return "/workspace", nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if config.Listen != "127.0.0.1:9001" || config.CWD != "/configured" || config.MaxClients != 2 {
		t.Fatalf("unexpected file config: %#v", config)
	}
	if config.AuthMode != "local" || config.SessionTTL != 24*time.Hour || config.OpenBrowser {
		t.Fatalf("unexpected file auth config: %#v", config)
	}
	if !reflect.DeepEqual(config.HerdrArgs, []string{"--session", "cli"}) {
		t.Fatalf("HerdrArgs = %#v", config.HerdrArgs)
	}
}

func TestParseConfigFileRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "herdr-web.json")
	if err := os.WriteFile(path, []byte(`{"unknown": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ParseConfig([]string{"--config", path}, func(string) string { return "" }, func() (string, error) {
		return "/workspace", nil
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestParseConfigKeepsFileHerdrArgsWithoutCLIArgs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "herdr-web.json")
	if err := os.WriteFile(path, []byte(`{"herdr_args":["--session","configured"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := ParseConfig([]string{"--config", path, "--no-open"}, func(string) string { return "" }, func() (string, error) {
		return "/workspace", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config.HerdrArgs, []string{"--session", "configured"}) {
		t.Fatalf("HerdrArgs = %#v", config.HerdrArgs)
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
