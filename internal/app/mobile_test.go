package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestInjectMobileAssets(t *testing.T) {
	html := "<!doctype html><html><head><title>ttyd</title></head><body></body></html>"
	response := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"text/html; charset=utf-8"}, "ETag": {"old"}},
		Body:          io.NopCloser(strings.NewReader(html)),
		ContentLength: int64(len(html)),
	}
	if err := injectMobileAssets(response); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{"data-herdr-web-mobile", mobileCSSPath, mobileJavaScriptPath} {
		if !strings.Contains(text, expected) {
			t.Fatalf("injected HTML does not contain %q: %s", expected, text)
		}
	}
	if response.ContentLength != int64(len(body)) || response.Header.Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Fatalf("wrong content length: %d %q", response.ContentLength, response.Header.Get("Content-Length"))
	}
	if response.Header.Get("ETag") != "" {
		t.Fatal("stale ETag was retained")
	}
}

func TestMobileAssetsRequireAuthentication(t *testing.T) {
	auth := testAuthenticator(t)
	handler := newGatewayHandler(auth, http.NotFoundHandler())

	request := httptest.NewRequest(http.MethodGet, "http://terminal.test"+mobileJavaScriptPath, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "http://terminal.test"+mobileJavaScriptPath, nil)
	request.AddCookie(&http.Cookie{Name: cookieName, Value: auth.token(), Expires: time.Now().Add(time.Hour)})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "WheelEvent") {
		t.Fatalf("authenticated asset status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestMobileScriptDoesNotInterceptKeyboard(t *testing.T) {
	script := string(mobileJavaScript)
	for _, event := range []string{"keydown", "keyup", "keypress"} {
		if strings.Contains(script, event) {
			t.Fatalf("mobile script unexpectedly contains %q", event)
		}
	}
	for _, feature := range []string{"touchmove", "WheelEvent", "visualViewport", "offsetTop", "contextmenu"} {
		if !strings.Contains(script, feature) {
			t.Fatalf("mobile script is missing %q", feature)
		}
	}
	for _, feature := range []string{"--herdr-web-viewport-top", "translate3d"} {
		if !strings.Contains(string(mobileCSS), feature) {
			t.Fatalf("mobile CSS is missing %q", feature)
		}
	}
}
