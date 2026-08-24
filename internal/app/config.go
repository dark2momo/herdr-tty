package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	usernameEnv       = "HERDR_TTY_USERNAME"
	passwordEnv       = "HERDR_TTY_PASSWORD"
	legacyUsernameEnv = "HERDR_WEB_USERNAME"
	legacyPasswordEnv = "HERDR_WEB_PASSWORD"
	herdrEnv          = "HERDR_ENV"
)

type Config struct {
	Listen      string
	Ttyd        string
	Herdr       string
	CWD         string
	MaxClients  int
	AuthMode    string
	Session     string
	SessionTTL  time.Duration
	Username    string
	Password    string
	HerdrArgs   []string
	OpenBrowser bool
}

type fileConfig struct {
	Listen      *string  `json:"listen"`
	Ttyd        *string  `json:"ttyd"`
	Herdr       *string  `json:"herdr"`
	CWD         *string  `json:"cwd"`
	MaxClients  *int     `json:"max_clients"`
	AuthMode    *string  `json:"auth"`
	Session     *string  `json:"session"`
	SessionTTL  *string  `json:"session_ttl"`
	OpenBrowser *bool    `json:"open_browser"`
	HerdrArgs   []string `json:"herdr_args"`
}

type getenvFunc func(string) string
type getwdFunc func() (string, error)

func ParseConfig(args []string, getenv getenvFunc, getwd getwdFunc) (Config, error) {
	if getenv(herdrEnv) != "" {
		return Config{}, errors.New("refusing to start inside Herdr: HERDR_ENV is set; launch herdr-tty from a regular shell or service")
	}

	workingDirectory, err := getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}

	config := Config{
		Listen:     "127.0.0.1:7681",
		Ttyd:       "ttyd",
		Herdr:      "herdr",
		CWD:        workingDirectory,
		MaxClients: 3,
		AuthMode:   "auto",
		SessionTTL: 7 * 24 * time.Hour,
	}
	configPath, err := configPathFromArgs(args)
	if err != nil {
		return Config{}, err
	}
	loaded := fileConfig{}
	if configPath != "" {
		loaded, err = loadConfigFile(configPath)
		if err != nil {
			return Config{}, err
		}
		if err := applyFileConfig(&config, loaded); err != nil {
			return Config{}, fmt.Errorf("load config %s: %w", configPath, err)
		}
	}

	flags := flag.NewFlagSet("herdr-tty", flag.ContinueOnError)
	flags.SetOutput(new(strings.Builder))
	parsedConfigPath := configPath
	openBrowser := false
	noOpenBrowser := false
	flags.StringVar(&parsedConfigPath, "config", configPath, "JSON configuration file")
	flags.StringVar(&config.Listen, "listen", config.Listen, "address to listen on")
	flags.StringVar(&config.Ttyd, "ttyd", config.Ttyd, "ttyd executable")
	flags.StringVar(&config.Herdr, "herdr", config.Herdr, "Herdr executable")
	flags.StringVar(&config.CWD, "cwd", config.CWD, "working directory")
	flags.IntVar(&config.MaxClients, "max-clients", config.MaxClients, "maximum concurrent clients")
	flags.StringVar(&config.AuthMode, "auth", config.AuthMode, "authentication mode: auto, local, form, or native")
	flags.StringVar(&config.Session, "session", config.Session, "named Herdr session")
	flags.DurationVar(&config.SessionTTL, "session-ttl", config.SessionTTL, "login session lifetime")
	flags.BoolVar(&openBrowser, "open", false, "open the web terminal in a browser")
	flags.BoolVar(&noOpenBrowser, "no-open", false, "do not open a browser")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if parsedConfigPath != configPath {
		return Config{}, errors.New("--config must be resolved before other options")
	}
	if parsedArgs := flags.Args(); len(parsedArgs) > 0 {
		config.HerdrArgs = parsedArgs
	}
	sessionSet := loaded.Session != nil
	flags.Visit(func(current *flag.Flag) {
		if current.Name == "session" {
			sessionSet = true
		}
	})
	if sessionSet && config.Session == "" {
		return Config{}, errors.New("--session cannot be empty")
	}
	if config.Session != "" && hasHerdrSessionArgument(config.HerdrArgs) {
		return Config{}, errors.New("--session cannot be combined with a Herdr --session argument after --")
	}
	if openBrowser && noOpenBrowser {
		return Config{}, errors.New("--open and --no-open cannot be used together")
	}
	config.Username, config.Password = credentialsFromEnvironment(getenv)
	if err := config.resolveAuthMode(); err != nil {
		return Config{}, err
	}
	switch {
	case openBrowser:
		config.OpenBrowser = true
	case noOpenBrowser:
		config.OpenBrowser = false
	case loaded.OpenBrowser != nil:
		config.OpenBrowser = *loaded.OpenBrowser
	default:
		config.OpenBrowser = config.AuthMode == "local"
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func credentialsFromEnvironment(getenv getenvFunc) (string, string) {
	username := getenv(usernameEnv)
	password := getenv(passwordEnv)
	if username != "" || password != "" {
		return username, password
	}
	return getenv(legacyUsernameEnv), getenv(legacyPasswordEnv)
}

func configPathFromArgs(args []string) (string, error) {
	path := ""
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			break
		}
		value := ""
		switch {
		case argument == "--config":
			index++
			if index >= len(args) {
				return "", errors.New("missing value for --config")
			}
			value = args[index]
		case strings.HasPrefix(argument, "--config="):
			value = strings.TrimPrefix(argument, "--config=")
		default:
			continue
		}
		if value == "" {
			return "", errors.New("--config cannot be empty")
		}
		if path != "" {
			return "", errors.New("--config can only be specified once")
		}
		path = value
	}
	return path, nil
}

func loadConfigFile(path string) (fileConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return fileConfig{}, fmt.Errorf("open config %s: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	config := fileConfig{}
	if err := decoder.Decode(&config); err != nil {
		return fileConfig{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return fileConfig{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	return config, nil
}

func applyFileConfig(config *Config, loaded fileConfig) error {
	if loaded.Listen != nil {
		config.Listen = *loaded.Listen
	}
	if loaded.Ttyd != nil {
		config.Ttyd = *loaded.Ttyd
	}
	if loaded.Herdr != nil {
		config.Herdr = *loaded.Herdr
	}
	if loaded.CWD != nil {
		config.CWD = *loaded.CWD
	}
	if loaded.MaxClients != nil {
		config.MaxClients = *loaded.MaxClients
	}
	if loaded.AuthMode != nil {
		config.AuthMode = *loaded.AuthMode
	}
	if loaded.Session != nil {
		config.Session = *loaded.Session
	}
	if loaded.SessionTTL != nil {
		ttl, err := time.ParseDuration(*loaded.SessionTTL)
		if err != nil {
			return fmt.Errorf("invalid session_ttl %q: %w", *loaded.SessionTTL, err)
		}
		config.SessionTTL = ttl
	}
	if loaded.HerdrArgs != nil {
		config.HerdrArgs = append([]string(nil), loaded.HerdrArgs...)
	}
	return nil
}

func hasHerdrSessionArgument(args []string) bool {
	for _, argument := range args {
		if argument == "--session" || strings.HasPrefix(argument, "--session=") {
			return true
		}
	}
	return false
}

func (config *Config) resolveAuthMode() error {
	hasUsername := config.Username != ""
	hasPassword := config.Password != ""

	switch config.AuthMode {
	case "auto":
		if hasUsername != hasPassword {
			return fmt.Errorf("%s and %s must be set together", usernameEnv, passwordEnv)
		}
		if hasUsername {
			config.AuthMode = "form"
		} else {
			config.AuthMode = "local"
		}
	case "local":
	case "form", "native":
		if hasUsername != hasPassword {
			return fmt.Errorf("%s and %s must be set together", usernameEnv, passwordEnv)
		}
	default:
		return errors.New("--auth must be auto, local, form, or native")
	}
	if config.AuthMode == "local" {
		host, _, err := net.SplitHostPort(config.Listen)
		if err == nil && !isLoopbackHost(host) {
			return errors.New("--auth local requires a loopback --listen address")
		}
		config.Username = ""
		config.Password = ""
		return nil
	}
	if !hasUsername {
		return fmt.Errorf("%s and %s are required for --auth %s", usernameEnv, passwordEnv, config.AuthMode)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if beforeZone, _, found := strings.Cut(host, "%"); found {
		host = beforeZone
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
	if config.AuthMode != "local" && config.AuthMode != "form" && config.AuthMode != "native" {
		return errors.New("--auth must resolve to local, form, or native")
	}
	if config.SessionTTL <= 0 {
		return errors.New("--session-ttl must be positive")
	}
	if config.Ttyd == "" || config.Herdr == "" {
		return errors.New("--ttyd and --herdr cannot be empty")
	}
	if config.CWD == "" {
		return errors.New("--cwd cannot be empty")
	}
	if config.AuthMode != "local" && strings.ContainsAny(config.Username, ":\r\n") {
		return fmt.Errorf("%s cannot contain a colon or newline", usernameEnv)
	}
	if config.AuthMode != "local" && strings.ContainsAny(config.Password, "\r\n") {
		return fmt.Errorf("%s cannot contain a newline", passwordEnv)
	}
	return nil
}

func (config Config) BackendArgs(port int) []string {
	args := []string{
		"--debug", "3",
		"--interface", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--writable",
		"--max-clients", strconv.Itoa(config.MaxClients),
		"--cwd", config.CWD,
		"--terminal-type", "xterm-256color",
	}
	return config.appendHerdrCommand(args)
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
	}
	return config.appendHerdrCommand(args)
}

func (config Config) appendHerdrCommand(args []string) []string {
	args = append(args, config.Herdr)
	if config.Session != "" {
		args = append(args, "--session", config.Session)
	}
	return append(args, config.HerdrArgs...)
}
