package app

import (
	"context"
	"os/exec"
	"strings"
)

const applicationEnvironmentPrefix = "HERDR_WEB_"

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
	if strings.HasPrefix(name, applicationEnvironmentPrefix) {
		return true
	}
	if name == "SSLKEYLOGFILE" || name == "NSS_KEYLOGFILE" {
		return true
	}
	return name == "NODE_OPTIONS" && strings.Contains(strings.ToLower(value), "--tls-keylog")
}
