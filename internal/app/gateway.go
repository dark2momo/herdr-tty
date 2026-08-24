package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func RunGateway(ctx context.Context, config Config, stdin io.Reader, stdout, stderr io.Writer) error {
	ttydPath, err := exec.LookPath(config.Ttyd)
	if err != nil {
		return fmt.Errorf("find ttyd: %w", err)
	}
	if _, err := exec.LookPath(config.Herdr); err != nil {
		return fmt.Errorf("find Herdr: %w", err)
	}

	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", config.Listen, err)
	}
	defer listener.Close()

	backendPort, err := availablePort()
	if err != nil {
		return err
	}
	backendAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(backendPort))
	command := newTtydCommand(ctx, ttydPath, config.BackendArgs(backendPort), os.Environ())
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start ttyd: %w", err)
	}
	childExit := make(chan error, 1)
	go func() { childExit <- command.Wait() }()
	defer stopProcess(command.Process, childExit)

	if err := waitForBackend(ctx, backendAddress, childExit); err != nil {
		return err
	}

	var auth *authenticator
	if config.AuthMode == "form" {
		auth, err = newAuthenticator(config)
		if err != nil {
			return err
		}
	}
	target := &url.URL{Scheme: "http", Host: backendAddress}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		director(request)
		request.Header.Set("Accept-Encoding", "identity")
	}
	proxy.ModifyResponse = injectMobileAssets
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, proxyErr error) {
		http.Error(writer, "terminal backend unavailable", http.StatusBadGateway)
		fmt.Fprintf(stderr, "proxy ttyd: %v\n", proxyErr)
	}

	server := &http.Server{
		Handler:           newGatewayHandler(auth, proxy),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	serverExit := make(chan error, 1)
	go func() { serverExit <- server.Serve(listener) }()
	browserURL := config.browserURL()
	fmt.Fprintf(stdout, "HerdrTTY listening on %s\n", browserURL)
	if config.OpenBrowser {
		if err := openBrowserURL(browserURL); err != nil {
			fmt.Fprintf(stderr, "open browser: %v\n", err)
		}
	}

	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
		return nil
	case err := <-childExit:
		_ = server.Close()
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			return errors.New("ttyd exited unexpectedly")
		}
		return fmt.Errorf("ttyd exited: %w", err)
	case err := <-serverExit:
		if ctx.Err() != nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve gateway: %w", err)
	}
}

func newGatewayHandler(auth *authenticator, upstream http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "same-origin")

		if handleAuthentication(writer, request, auth) {
			return
		}
		if isWebSocket(request) && (!sameOrigin(request) || request.Header.Get("Origin") == "") {
			http.Error(writer, "cross-origin WebSocket denied", http.StatusForbidden)
			return
		}
		if serveMobileAsset(writer, request) {
			return
		}
		upstream.ServeHTTP(writer, request)
	})
}

func handleAuthentication(writer http.ResponseWriter, request *http.Request, auth *authenticator) bool {
	if auth == nil {
		if request.URL.Path == loginPath || request.URL.Path == logoutPath {
			http.Redirect(writer, request, "/", http.StatusSeeOther)
			return true
		}
		return false
	}

	switch request.URL.Path {
	case loginPath:
		if request.Method == http.MethodGet {
			if auth.authenticated(request) {
				http.Redirect(writer, request, "/", http.StatusSeeOther)
				return true
			}
			serveLogin(writer, http.StatusOK, "")
			return true
		}
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		if !sameOrigin(request) {
			http.Error(writer, "cross-origin request denied", http.StatusForbidden)
			return true
		}
		request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
		if err := request.ParseForm(); err != nil {
			serveLogin(writer, http.StatusBadRequest, "Invalid request")
			return true
		}
		if !auth.credentialsMatch(request.FormValue("username"), request.FormValue("password")) {
			serveLogin(writer, http.StatusUnauthorized, "Invalid username or password")
			return true
		}
		auth.setCookie(writer, request)
		http.Redirect(writer, request, "/", http.StatusSeeOther)
		return true

	case logoutPath:
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		if !sameOrigin(request) {
			http.Error(writer, "cross-origin request denied", http.StatusForbidden)
			return true
		}
		clearCookie(writer, request)
		http.Redirect(writer, request, loginPath, http.StatusSeeOther)
		return true
	}

	if auth.authenticated(request) {
		return false
	}
	if isWebSocket(request) {
		http.Error(writer, "authentication required", http.StatusUnauthorized)
	} else {
		http.Redirect(writer, request, loginPath, http.StatusSeeOther)
	}
	return true
}

func availablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve backend port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return 0, fmt.Errorf("release backend port: %w", err)
	}
	return port, nil
}

func waitForBackend(ctx context.Context, address string, childExit <-chan error) error {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-childExit:
			if err == nil {
				return errors.New("ttyd exited before becoming ready")
			}
			return fmt.Errorf("ttyd exited before becoming ready: %w", err)
		case <-timer.C:
			return errors.New("timed out waiting for ttyd")
		case <-ticker.C:
			connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err == nil {
				_ = connection.Close()
				return nil
			}
		}
	}
}

func stopProcess(process *os.Process, childExit <-chan error) {
	if process == nil {
		return
	}
	if err := process.Signal(os.Interrupt); errors.Is(err, os.ErrProcessDone) {
		return
	}
	select {
	case <-childExit:
		return
	case <-time.After(time.Second):
		_ = process.Kill()
		select {
		case <-childExit:
		case <-time.After(time.Second):
		}
	}
}
