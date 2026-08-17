.PHONY: build test

build:
	go build -o bin/herdr-web ./cmd/herdr-web

test:
	go test ./...

