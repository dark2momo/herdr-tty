.PHONY: build check test

GOWORK ?= off
export GOWORK

build:
	mkdir -p bin
	go build -trimpath -ldflags="-s -w" -o bin/herdr-web ./cmd/herdr-web

test:
	go test ./...

check:
	go test -race ./...
	go vet ./...
	go build ./...
