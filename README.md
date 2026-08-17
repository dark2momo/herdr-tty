# Herdr Web

A small web entry point for [Herdr](https://herdr.dev), powered by
[ttyd](https://github.com/tsl0922/ttyd).

The primary design rule is to stay lightweight: one small Go program, the Go
standard library, and the existing ttyd/Herdr binaries. No Node.js runtime,
frontend framework, database, or separate control plane.

This first version intentionally stays minimal: it starts ttyd with writable
terminal access, ttyd's native Basic Authentication, same-origin WebSocket
checks, and a configurable client limit.

## Requirements

- Herdr
- ttyd 1.7+
- Go 1.23+ (when building from source)

## Run

```bash
go build -o herdr-web ./cmd/herdr-web

export HERDR_WEB_USERNAME='your-name'
read -rsp 'Password: ' HERDR_WEB_PASSWORD
export HERDR_WEB_PASSWORD

./herdr-web --listen 127.0.0.1:7681
```

Open `http://127.0.0.1:7681`. Extra arguments after `--` are passed to Herdr:

```bash
./herdr-web --listen 127.0.0.1:7681 -- --session work
```

Options:

```text
--listen        address to listen on (default 127.0.0.1:7681)
--ttyd          ttyd executable (default ttyd)
--herdr         Herdr executable (default herdr)
--cwd           working directory exposed to Herdr (default current directory)
--max-clients   maximum concurrent ttyd clients (default 3)
```

## Security

The default bind address is loopback. Exposing a writable web terminal grants
shell access with your user privileges; put it behind a network boundary you
trust.

ttyd's native authentication receives `user:password` as a process argument.
This is kept as the minimal baseline and may expose the credential to local
process inspection. Do not reuse an important password.

## Scope

Herdr Web serves the browser-to-Herdr application path. VPNs, tunnels, port
forwarding, DNS, firewall rules, and reverse proxies are outside this initial
stage.

## License

MIT
