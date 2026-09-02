package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	loginPath  = "/_herdr/login"
	logoutPath = "/_herdr/logout"
	cookieName = "herdr_tty_session"
)

var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
  <meta name="color-scheme" content="dark">
  <meta name="theme-color" content="#090b10">
  <title>HerdrTTY</title>
  <style>
    :root { color-scheme: dark; background: #090b10; font: 16px/1.5 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; }
    * { box-sizing: border-box; }
    body { min-height: 100dvh; margin: 0; display: grid; place-items: center; padding: 24px; background: #090b10; color: #e6e9ef; }
    main { width: min(100%, 360px); }
    h1 { margin: 0 0 6px; font-size: 24px; letter-spacing: -.02em; }
    p { margin: 0 0 22px; color: #9399a8; }
    label { display: block; margin: 14px 0 6px; font-size: 13px; color: #b5bac8; }
    input,button { width: 100%; min-height: 44px; border-radius: 8px; font: inherit; }
    input { border: 1px solid #303544; padding: 10px 12px; background: #11141c; color: inherit; outline: none; }
    input:focus { border-color: #89b4fa; box-shadow: 0 0 0 3px #89b4fa22; }
    button { margin-top: 20px; border: 0; background: #89b4fa; color: #0b1020; font-weight: 700; cursor: pointer; }
    .error { margin: 0 0 14px; padding: 10px 12px; border: 1px solid #f38ba855; border-radius: 8px; color: #f38ba8; background: #f38ba811; }
  </style>
</head>
<body>
  <main>
    <h1>HerdrTTY</h1>
    <p>Sign in to continue to your terminal.</p>
    {{if .Error}}<div class="error" role="alert">{{.Error}}</div>{{end}}
    <form id="login-form" method="post" action="/_herdr/login">
      <label for="username">Username</label>
      <input id="username" name="username" autocomplete="username" autocapitalize="none" required autofocus>
      <label for="password">Password</label>
      <input id="password" name="password" type="password" autocomplete="current-password" required>
      <button type="submit">Sign in</button>
    </form>
  </main>
  <script>
    (() => {
      const userAgent = navigator.userAgent;
      const isTouchApple =
        /\b(iPad|iPhone|iPod)\b/.test(userAgent) ||
        (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
      const isStandalone =
        navigator.standalone === true ||
        window.matchMedia?.("(display-mode: standalone)").matches === true;
      if (!isTouchApple || !isStandalone) return;

      const form = document.getElementById("login-form");
      form.addEventListener("submit", (event) => {
        if (form.dataset.submitting === "true") return;
        event.preventDefault();
        form.dataset.submitting = "true";
        form.querySelector('button[type="submit"]').disabled = true;
        document.activeElement?.blur();
        window.scrollTo(0, 0);
        window.setTimeout(() => HTMLFormElement.prototype.submit.call(form), 300);
      });
    })();
  </script>
</body>
</html>`))

type authenticator struct {
	username string
	password [sha256.Size]byte
	secret   []byte
	ttl      time.Duration
	now      func() time.Time
}

func newAuthenticator(config Config) (*authenticator, error) {
	secret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return nil, fmt.Errorf("generate session secret: %w", err)
	}
	return &authenticator{
		username: config.Username,
		password: sha256.Sum256([]byte(config.Password)),
		secret:   secret,
		ttl:      config.SessionTTL,
		now:      time.Now,
	}, nil
}

func (auth *authenticator) credentialsMatch(username, password string) bool {
	wantUser := sha256.Sum256([]byte(auth.username))
	gotUser := sha256.Sum256([]byte(username))
	gotPassword := sha256.Sum256([]byte(password))
	return subtle.ConstantTimeCompare(gotUser[:], wantUser[:]) == 1 &&
		subtle.ConstantTimeCompare(gotPassword[:], auth.password[:]) == 1
}

func (auth *authenticator) token() string {
	expires := auth.now().Add(auth.ttl).Unix()
	payload := strconv.FormatInt(expires, 10)
	mac := hmac.New(sha256.New, auth.secret)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (auth *authenticator) validToken(token string) bool {
	payload, encodedSignature, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	expires, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || !auth.now().Before(time.Unix(expires, 0)) {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, auth.secret)
	_, _ = mac.Write([]byte(payload))
	return hmac.Equal(signature, mac.Sum(nil))
}

func (auth *authenticator) authenticated(request *http.Request) bool {
	cookie, err := request.Cookie(cookieName)
	return err == nil && auth.validToken(cookie.Value)
}

func (auth *authenticator) setCookie(writer http.ResponseWriter, request *http.Request) {
	expires := auth.now().Add(auth.ttl)
	http.SetCookie(writer, &http.Cookie{
		Name:     cookieName,
		Value:    auth.token(),
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(auth.ttl.Seconds()),
		HttpOnly: true,
		Secure:   request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearCookie(writer http.ResponseWriter, request *http.Request) {
	http.SetCookie(writer, &http.Cookie{
		Name:     cookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode,
	})
}

type loginPageData struct {
	Error string
}

func serveLogin(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	_ = loginTemplate.Execute(writer, loginPageData{Error: message})
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(parsed.Host, request.Host) {
		return true
	}

	forwardedHost := request.Header.Get("X-Forwarded-Host")
	if first, _, found := strings.Cut(forwardedHost, ","); found {
		forwardedHost = first
	}
	if forwardedHost = strings.TrimSpace(forwardedHost); forwardedHost != "" && strings.EqualFold(parsed.Host, forwardedHost) {
		return true
	}

	return strings.EqualFold(strings.TrimSpace(request.Header.Get("Sec-Fetch-Site")), "same-origin")
}

func isWebSocket(request *http.Request) bool {
	return strings.EqualFold(request.Header.Get("Upgrade"), "websocket") &&
		headerContainsToken(request.Header.Get("Connection"), "upgrade")
}

func headerContainsToken(header, token string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
