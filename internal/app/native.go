package app

import (
	"fmt"
	"io"
	"os/exec"
)

func RunNative(config Config, stdin io.Reader, stdout, stderr io.Writer) error {
	ttydPath, err := exec.LookPath(config.Ttyd)
	if err != nil {
		return fmt.Errorf("find ttyd: %w", err)
	}
	if _, err := exec.LookPath(config.Herdr); err != nil {
		return fmt.Errorf("find Herdr: %w", err)
	}

	command := exec.Command(ttydPath, config.NativeArgs()...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run ttyd: %w", err)
	}
	return nil
}
