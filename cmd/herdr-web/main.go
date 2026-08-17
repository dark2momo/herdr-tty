package main

import (
	"fmt"
	"os"

	"github.com/dark2momo/herdr-web/internal/app"
)

func main() {
	config, err := app.ParseConfig(os.Args[1:], os.Getenv, os.Getwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if err := app.RunNative(config, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
