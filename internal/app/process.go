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
		name, _, found := strings.Cut(entry, "=")
		if found && strings.HasPrefix(strings.ToUpper(name), applicationEnvironmentPrefix) {
			continue
		}
		clean = append(clean, entry)
	}
	return clean
}
