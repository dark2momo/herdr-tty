package app

import (
	"context"
	"os/exec"
	"strings"
)

var applicationEnvironmentPrefixes = [...]string{"HERDR_TTY_", "HERDR_WEB_"}

func newTtydCommand(ctx context.Context, path string, args, environment []string) *exec.Cmd {
	command := exec.CommandContext(ctx, path, args...)
	command.Env = childEnvironment(environment)
	return command
}

func childEnvironment(environment []string) []string {
	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if found && excludeChildEnvironment(strings.ToUpper(name), value) {
			continue
		}
		clean = append(clean, entry)
	}
	return clean
}

func excludeChildEnvironment(name, value string) bool {
	for _, prefix := range applicationEnvironmentPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	if name == "SSLKEYLOGFILE" || name == "NSS_KEYLOGFILE" {
		return true
	}
	return name == "NODE_OPTIONS" && strings.Contains(strings.ToLower(value), "--tls-keylog")
}
