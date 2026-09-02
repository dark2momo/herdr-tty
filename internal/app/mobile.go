package app

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	mobileJavaScriptPath = "/_herdr/mobile.js"
	mobileCSSPath        = "/_herdr/mobile.css"
	maxHTMLSize          = 4 << 20
	mobileHead           = `<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,viewport-fit=cover" data-herdr-tty-mobile><meta name="color-scheme" content="dark"><meta name="theme-color" content="#000000"><link rel="stylesheet" href="/_herdr/mobile.css"><script defer src="/_herdr/mobile.js"></script>`
)

//go:embed web/mobile.js
var mobileJavaScript []byte

//go:embed web/mobile.css
var mobileCSS []byte

func serveMobileAsset(writer http.ResponseWriter, request *http.Request) bool {
	var content []byte
	switch request.URL.Path {
	case mobileJavaScriptPath:
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		content = mobileJavaScript
	case mobileCSSPath:
		writer.Header().Set("Content-Type", "text/css; charset=utf-8")
		content = mobileCSS
	default:
		return false
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Content-Length", strconv.Itoa(len(content)))
	if request.Method != http.MethodHead {
		_, _ = writer.Write(content)
	}
	return true
}

func injectMobileAssets(response *http.Response) error {
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/html") {
		return nil
	}
	if response.Header.Get("Content-Encoding") != "" {
		return fmt.Errorf("cannot inject mobile assets into encoded HTML")
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxHTMLSize+1))
	if err != nil {
		return fmt.Errorf("read ttyd HTML: %w", err)
	}
	_ = response.Body.Close()
	if len(body) > maxHTMLSize {
		return fmt.Errorf("ttyd HTML exceeds %d bytes", maxHTMLSize)
	}
	if bytes.Contains(body, []byte("data-herdr-tty-mobile")) {
		response.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	marker := []byte("</head>")
	position := bytes.LastIndex(bytes.ToLower(body), marker)
	if position < 0 {
		response.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}

	injected := make([]byte, 0, len(body)+len(mobileHead))
	injected = append(injected, body[:position]...)
	injected = append(injected, mobileHead...)
	injected = append(injected, body[position:]...)
	response.Body = io.NopCloser(bytes.NewReader(injected))
	response.ContentLength = int64(len(injected))
	response.Header.Set("Content-Length", strconv.Itoa(len(injected)))
	response.Header.Del("ETag")
	response.Header.Del("Last-Modified")
	return nil
}
