package app

import (
	"context"
	"reflect"
	"testing"
)

func TestChildEnvironmentRemovesCredentialsAndTLSKeyLogs(t *testing.T) {
	environment := []string{
		"PATH=/usr/bin",
		"HERDR_TTY_USERNAME=alice",
		"HERDR_TTY_PASSWORD=secret",
		"HERDR_TTY_FUTURE_SECRET=future",
		"herdr_tty_mixed_case=hidden",
		"HERDR_WEB_USERNAME=legacy-alice",
		"HERDR_WEB_PASSWORD=legacy-secret",
		"herdr_web_mixed_case=legacy-hidden",
		"HERDR_ENV=1",
		"HERDR_BIN_PATH=/usr/local/bin/herdr",
		"HERDR_WORKSPACE_ID=workspace-1",
		"HERDR_TAB_ID=tab-1",
		"HERDR_PANE_ID=pane-1",
		"HERDR_PANE_RUNTIME_ID=runtime-1",
		"HERDR_ACTIVE_WORKSPACE_ID=workspace-1",
		"HERDR_PLUGIN_ID=plugin-1",
		"HERDR_STARTUP_CWD=/outer",
		"HERDR_REATTACH_COMMAND=herdr --remote host",
		"SSLKEYLOGFILE=/tmp/tls.keys",
		"nss_keylogfile=/tmp/nss.keys",
		"NODE_OPTIONS=--trace-warnings --tls-keylog=/tmp/node.keys",
		"HERDR_SESSION=work",
		"HERDR_SOCKET_PATH=/tmp/herdr.sock",
		"HERDR_CONFIG_PATH=/home/alice/.config/herdr/config.toml",
		"OPENAI_API_KEY=test-key",
		"HTTPS_PROXY=http://proxy.test",
		"TERM=xterm-256color",
		"MALFORMED",
	}
	want := []string{
		"PATH=/usr/bin",
		"HERDR_ENV=1",
		"HERDR_BIN_PATH=/usr/local/bin/herdr",
		"HERDR_WORKSPACE_ID=workspace-1",
		"HERDR_TAB_ID=tab-1",
		"HERDR_PANE_ID=pane-1",
		"HERDR_PANE_RUNTIME_ID=runtime-1",
		"HERDR_ACTIVE_WORKSPACE_ID=workspace-1",
		"HERDR_PLUGIN_ID=plugin-1",
		"HERDR_STARTUP_CWD=/outer",
		"HERDR_REATTACH_COMMAND=herdr --remote host",
		"HERDR_SESSION=work",
		"HERDR_SOCKET_PATH=/tmp/herdr.sock",
		"HERDR_CONFIG_PATH=/home/alice/.config/herdr/config.toml",
		"OPENAI_API_KEY=test-key",
		"HTTPS_PROXY=http://proxy.test",
		"TERM=xterm-256color",
		"MALFORMED",
	}
	if got := childEnvironment(environment); !reflect.DeepEqual(got, want) {
		t.Fatalf("childEnvironment() = %#v, want %#v", got, want)
	}
}

func TestChildEnvironmentKeepsOrdinaryNodeOptions(t *testing.T) {
	environment := []string{"NODE_OPTIONS=--max-old-space-size=4096"}
	if got := childEnvironment(environment); !reflect.DeepEqual(got, environment) {
		t.Fatalf("childEnvironment() = %#v, want %#v", got, environment)
	}
}

func TestTtydCommandUsesCleanEnvironment(t *testing.T) {
	command := newTtydCommand(
		context.Background(),
		"ttyd",
		[]string{"--version"},
		[]string{"PATH=/usr/bin", "HERDR_TTY_PASSWORD=secret", "HERDR_WEB_PASSWORD=legacy-secret"},
	)
	if !reflect.DeepEqual(command.Env, []string{"PATH=/usr/bin"}) {
		t.Fatalf("command.Env = %#v", command.Env)
	}
}
