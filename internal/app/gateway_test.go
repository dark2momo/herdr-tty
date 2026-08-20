package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testAuthenticator(t *testing.T) *authenticator {
	t.Helper()
	auth, err := newAuthenticator(Config{
		Username:   "alice",
		Password:   "secret",
		SessionTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return auth
}

func TestGatewayLoginFlow(t *testing.T) {
	auth := testAuthenticator(t)
	upstream := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("proxied"))
	})
	handler := newGatewayHandler(auth, upstream)

	protectedRequest := httptest.NewRequest(http.MethodGet, "http://terminal.test/", nil)
	protectedResponse := httptest.NewRecorder()
	handler.ServeHTTP(protectedResponse, protectedRequest)
	if protectedResponse.Code != http.StatusSeeOther || protectedResponse.Header().Get("Location") != loginPath {
		t.Fatalf("unexpected protected response: %d %#v", protectedResponse.Code, protectedResponse.Header())
	}
	if got := protectedResponse.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("Referrer-Policy = %q", got)
	}

	form := url.Values{"username": {"alice"}, "password": {"secret"}}
	loginRequest := httptest.NewRequest(http.MethodPost, "http://terminal.test"+loginPath, strings.NewReader(form.Encode()))
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRequest.Header.Set("Origin", "http://terminal.test")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", loginResponse.Code)
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "http://terminal.test/", nil)
	authenticatedRequest.AddCookie(cookies[0])
	authenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(authenticatedResponse, authenticatedRequest)
	if authenticatedResponse.Code != http.StatusOK || authenticatedResponse.Body.String() != "proxied" {
		t.Fatalf("unexpected authenticated response: %d %q", authenticatedResponse.Code, authenticatedResponse.Body.String())
	}
}

func TestLoginPageSettlesSafariKeyboardBeforeSubmit(t *testing.T) {
	handler := newGatewayHandler(testAuthenticator(t), http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "http://terminal.test"+loginPath, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d", response.Code)
	}
	body := response.Body.String()
	for _, expected := range []string{
		`id="login-form"`,
		`document.activeElement?.blur()`,
		`window.scrollTo(0, 0)`,
		`HTMLFormElement.prototype.submit.call(form)`,
		`window.setTimeout`,
		`CriOS`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("login page is missing %q", expected)
		}
	}
}

func TestGatewayLocalModeSkipsLoginAndStillChecksWebSocketOrigin(t *testing.T) {
	upstreamCalls := 0
	handler := newGatewayHandler(nil, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalls++
		if isWebSocket(request) {
			writer.WriteHeader(http.StatusSwitchingProtocols)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "http://terminal.test/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || upstreamCalls != 1 {
		t.Fatalf("local root status=%d upstreamCalls=%d", response.Code, upstreamCalls)
	}

	request = httptest.NewRequest(http.MethodGet, "http://terminal.test"+loginPath, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("local login status=%d location=%q", response.Code, response.Header().Get("Location"))
	}

	request = httptest.NewRequest(http.MethodGet, "http://terminal.test/ws", nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Origin", "http://evil.test")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || upstreamCalls != 1 {
		t.Fatalf("cross-origin status=%d upstreamCalls=%d", response.Code, upstreamCalls)
	}
}

func TestGatewayRejectsInvalidLogin(t *testing.T) {
	handler := newGatewayHandler(testAuthenticator(t), http.NotFoundHandler())
	form := url.Values{"username": {"alice"}, "password": {"wrong"}}
	request := httptest.NewRequest(http.MethodPost, "http://terminal.test"+loginPath, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("invalid login set a cookie")
	}
}

func TestGatewayChecksWebSocketOrigin(t *testing.T) {
	auth := testAuthenticator(t)
	upstreamCalls := 0
	handler := newGatewayHandler(auth, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		writer.WriteHeader(http.StatusSwitchingProtocols)
	}))

	request := httptest.NewRequest(http.MethodGet, "http://terminal.test/ws", nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Origin", "http://evil.test")
	request.AddCookie(&http.Cookie{Name: cookieName, Value: auth.token()})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || upstreamCalls != 0 {
		t.Fatalf("cross-origin status=%d upstreamCalls=%d", response.Code, upstreamCalls)
	}

	request = httptest.NewRequest(http.MethodGet, "http://terminal.test/ws", nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Origin", "http://terminal.test")
	request.AddCookie(&http.Cookie{Name: cookieName, Value: auth.token()})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSwitchingProtocols || upstreamCalls != 1 {
		t.Fatalf("same-origin status=%d upstreamCalls=%d", response.Code, upstreamCalls)
	}
}

func TestExpiredToken(t *testing.T) {
	auth := testAuthenticator(t)
	now := time.Unix(1_700_000_000, 0)
	auth.now = func() time.Time { return now }
	token := auth.token()
	auth.now = func() time.Time { return now.Add(2 * time.Hour) }
	if auth.validToken(token) {
		t.Fatal("expired token was accepted")
	}
}
