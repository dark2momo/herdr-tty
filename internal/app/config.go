package app

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	usernameEnv = "HERDR_WEB_USERNAME"
	passwordEnv = "HERDR_WEB_PASSWORD"
)

type Config struct {
	Listen     string
	Ttyd       string
	Herdr      string
	CWD        string
	MaxClients int
	Username   string
	Password   string
	HerdrArgs  []string
}

type getenvFunc func(string) string
type getwdFunc func() (string, error)

func ParseConfig(args []string, getenv getenvFunc, getwd getwdFunc) (Config, error) {
	workingDirectory, err := getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}

	config := Config{}
	flags := flag.NewFlagSet("herdr-web", flag.ContinueOnError)
	flags.SetOutput(new(strings.Builder))
	flags.StringVar(&config.Listen, "listen", "127.0.0.1:7681", "address to listen on")
	flags.StringVar(&config.Ttyd, "ttyd", "ttyd", "ttyd executable")
	flags.StringVar(&config.Herdr, "herdr", "herdr", "Herdr executable")
	flags.StringVar(&config.CWD, "cwd", workingDirectory, "working directory")
	flags.IntVar(&config.MaxClients, "max-clients", 3, "maximum concurrent clients")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	config.HerdrArgs = flags.Args()
	config.Username = getenv(usernameEnv)
	config.Password = getenv(passwordEnv)

	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	host, portText, err := net.SplitHostPort(config.Listen)
	if err != nil {
		return fmt.Errorf("invalid --listen %q: %w", config.Listen, err)
	}
	if host == "" {
		return errors.New("--listen must include an explicit host")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port in --listen %q", config.Listen)
	}
	if config.MaxClients < 1 {
		return errors.New("--max-clients must be at least 1")
	}
	if config.Ttyd == "" || config.Herdr == "" {
		return errors.New("--ttyd and --herdr cannot be empty")
	}
	if config.CWD == "" {
		return errors.New("--cwd cannot be empty")
	}
	if config.Username == "" || config.Password == "" {
		return fmt.Errorf("%s and %s are required", usernameEnv, passwordEnv)
	}
	if strings.ContainsAny(config.Username, ":\r\n") {
		return fmt.Errorf("%s cannot contain a colon or newline", usernameEnv)
	}
	if strings.ContainsAny(config.Password, "\r\n") {
		return fmt.Errorf("%s cannot contain a newline", passwordEnv)
	}
	return nil
}

func (config Config) NativeArgs() []string {
	host, port, _ := net.SplitHostPort(config.Listen)
	args := []string{
		"--debug", "3",
		"--interface", host,
		"--port", port,
		"--writable",
		"--check-origin",
		"--max-clients", strconv.Itoa(config.MaxClients),
		"--credential", config.Username + ":" + config.Password,
		"--cwd", config.CWD,
		"--terminal-type", "xterm-256color",
		config.Herdr,
	}
	return append(args, config.HerdrArgs...)
}
