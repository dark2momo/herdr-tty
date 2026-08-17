package app

import (
	"context"
	"reflect"
	"testing"
)

func TestChildEnvironmentRemovesHerdrWebValues(t *testing.T) {
	environment := []string{
		"PATH=/usr/bin",
		"HERDR_WEB_USERNAME=alice",
		"HERDR_WEB_PASSWORD=secret",
		"HERDR_WEB_FUTURE_SECRET=future",
		"herdr_web_mixed_case=hidden",
		"TERM=xterm-256color",
		"MALFORMED",
	}
	want := []string{"PATH=/usr/bin", "TERM=xterm-256color", "MALFORMED"}
	if got := childEnvironment(environment); !reflect.DeepEqual(got, want) {
		t.Fatalf("childEnvironment() = %#v, want %#v", got, want)
	}
}

func TestTtydCommandUsesCleanEnvironment(t *testing.T) {
	command := newTtydCommand(
		context.Background(),
		"ttyd",
		[]string{"--version"},
		[]string{"PATH=/usr/bin", "HERDR_WEB_PASSWORD=secret"},
	)
	if !reflect.DeepEqual(command.Env, []string{"PATH=/usr/bin"}) {
		t.Fatalf("command.Env = %#v", command.Env)
	}
}
