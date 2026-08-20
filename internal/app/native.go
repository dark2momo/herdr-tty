package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func Run(ctx context.Context, config Config, stdin io.Reader, stdout, stderr io.Writer) error {
	if config.AuthMode == "native" {
		return RunNative(ctx, config, stdin, stdout, stderr)
	}
	return RunGateway(ctx, config, stdin, stdout, stderr)
}

func RunNative(ctx context.Context, config Config, stdin io.Reader, stdout, stderr io.Writer) error {
	ttydPath, err := exec.LookPath(config.Ttyd)
	if err != nil {
		return fmt.Errorf("find ttyd: %w", err)
	}
	if _, err := exec.LookPath(config.Herdr); err != nil {
		return fmt.Errorf("find Herdr: %w", err)
	}

	command := newTtydCommand(ctx, ttydPath, config.NativeArgs(), os.Environ())
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if !config.OpenBrowser {
		if err := command.Run(); err != nil {
			return fmt.Errorf("run ttyd: %w", err)
		}
		return nil
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("start ttyd: %w", err)
	}
	childExit := make(chan error, 1)
	go func() { childExit <- command.Wait() }()
	if err := waitForBackend(ctx, config.Listen, childExit); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	browserURL := config.browserURL()
	fmt.Fprintf(stdout, "Herdr Web listening on %s\n", browserURL)
	if err := openBrowserURL(browserURL); err != nil {
		fmt.Fprintf(stderr, "open browser: %v\n", err)
	}
	select {
	case <-ctx.Done():
		<-childExit
		return nil
	case err := <-childExit:
		if err == nil {
			return nil
		}
		return fmt.Errorf("run ttyd: %w", err)
	}
}
