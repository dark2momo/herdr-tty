# Herdr Web

[![check](https://github.com/dark2momo/herdr-web/actions/workflows/check.yml/badge.svg)](https://github.com/dark2momo/herdr-web/actions/workflows/check.yml)

A small web entry point for [Herdr](https://herdr.dev), powered by
[ttyd](https://github.com/tsl0922/ttyd).

The primary design rule is to stay lightweight: one small Go program, the Go
standard library, and the existing ttyd/Herdr binaries. No Node.js runtime,
frontend framework, database, or separate control plane.

On touch devices, a small vanilla JavaScript/CSS layer adds drag and inertial
scrolling while leaving keyboard events untouched. It emits standard wheel
events, so xterm.js and Herdr retain their native scroll and mouse semantics.

It starts ttyd as a loopback-only backend and puts a small Cookie-authenticated
Go gateway in front. The gateway keeps credentials out of ttyd's process
arguments, checks WebSocket origins, and enforces a configurable client limit.

## Requirements

- Herdr
- ttyd 1.7+
- Go 1.23+ (when building from source)
- Node.js 20+ (only for JavaScript checks; not needed at runtime)

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
--auth          authentication mode: form or native (default form)
--session-ttl   login lifetime (default 168h)
```

## Mobile behavior

- One-finger drag scrolls the active Herdr view with light inertia.
- Long-press and drag selects terminal text; a temporary Copy button appears
  after the gesture.
- A two-finger tap sends a right mouse click to Herdr.
- The terminal follows `visualViewport` when a mobile keyboard changes the
  visible area, including iOS viewport offsets.
- On iPad Chrome, a small stale focus inset is ignored after the keyboard
  closes, while an open keyboard still resizes the terminal above it.
- iOS virtual Chinese keyboards forward punctuation through a narrow
  `beforeinput` fallback for xterm.js; ordinary text, active composition,
  desktop keyboards, and Herdr shortcuts keep their native paths.
- The browser context menu is suppressed inside the web app.
- No `keydown`, `keyup`, or `keypress` handler is installed, so Herdr keyboard
  shortcuts continue through ttyd unchanged.

## Security

The default bind address is loopback. Exposing a writable web terminal grants
shell access with your user privileges; put it behind a network boundary you
trust.

The default form login uses an HTTP-only, same-site Cookie signed by an
in-memory random key. Restarting Herdr Web invalidates existing sessions.
Credentials stay in Herdr Web and are not passed to ttyd: all `HERDR_WEB_*`
variables are removed from the environment inherited by ttyd and Herdr.

The original ttyd Basic Authentication mode remains available with
`--auth native`. Native mode receives `user:password` as a ttyd process
argument and may expose the credential to local process inspection. Do not
reuse an important password with that mode.

## Scope

Herdr Web serves the browser-to-Herdr application path. VPNs, tunnels, port
forwarding, DNS, firewall rules, and reverse proxies are outside this initial
stage.

## License

MIT
