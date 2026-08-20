package app

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type browserInvocation struct {
	program string
	args    []string
}

func (config Config) browserURL() string {
	host, port, _ := net.SplitHostPort(config.Listen)
	if host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func openBrowserURL(rawURL string) error {
	wsl := runtime.GOOS == "linux" && (os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "")
	candidates := browserInvocations(runtime.GOOS, wsl, rawURL)
	tried := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		tried = append(tried, candidate.program)
		path, err := exec.LookPath(candidate.program)
		if err != nil {
			continue
		}
		command := exec.Command(path, candidate.args...)
		command.Env = childEnvironment(os.Environ())
		command.Stdin = nil
		command.Stdout = io.Discard
		command.Stderr = io.Discard
		if err := command.Start(); err != nil {
			return fmt.Errorf("start %s: %w", candidate.program, err)
		}
		go func() { _ = command.Wait() }()
		return nil
	}
	if len(tried) == 0 {
		return fmt.Errorf("browser opening is unsupported on %s", runtime.GOOS)
	}
	return fmt.Errorf("no browser opener found (tried %s)", strings.Join(tried, ", "))
}

func browserInvocations(goos string, wsl bool, rawURL string) []browserInvocation {
	switch goos {
	case "darwin":
		return []browserInvocation{{program: "open", args: []string{rawURL}}}
	case "windows":
		return []browserInvocation{{program: "rundll32.exe", args: []string{"url.dll,FileProtocolHandler", rawURL}}}
	case "linux":
		if wsl {
			return []browserInvocation{
				{program: "rundll32.exe", args: []string{"url.dll,FileProtocolHandler", rawURL}},
				{program: "cmd.exe", args: []string{"/c", "start", "", rawURL}},
				{program: "wslview", args: []string{rawURL}},
				{program: "xdg-open", args: []string{rawURL}},
			}
		}
		return []browserInvocation{{program: "xdg-open", args: []string{rawURL}}}
	default:
		return nil
	}
}
