# HerdrTTY

[![check](https://github.com/dark2momo/herdr-tty/actions/workflows/check.yml/badge.svg)](https://github.com/dark2momo/herdr-tty/actions/workflows/check.yml)

Herdr in your browser: a lightweight, mobile-friendly web terminal for
[Herdr](https://herdr.dev), powered by
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

## Quick start

With `herdr` and `ttyd` on `PATH`, run:

```bash
herdr-tty
```

HerdrTTY listens only on `127.0.0.1:7681`, skips login for that loopback-only
session, and opens the terminal in the local browser. It runs in the foreground;
press Ctrl+C to stop it. Use `--no-open` when launching from a service or a
headless shell.

Launch HerdrTTY from a regular shell or service, not from an existing Herdr
pane. It refuses to start when `HERDR_ENV` is set instead of silently creating
or altering a nested session.

To build from source first:

```bash
GOWORK=off go build -o herdr-tty ./cmd/herdr-tty

./herdr-tty
```

Select a named persistent Herdr session directly:

```bash
herdr-tty --session work
```

Other arguments after `--` are still passed to Herdr.

## Configuration

Pass an explicit JSON configuration file for repeatable launches:

```bash
herdr-tty --config ~/.config/herdr-tty/config.json
```

```json
{
  "listen": "127.0.0.1:7681",
  "cwd": "/home/me/projects",
  "max_clients": 3,
  "auth": "local",
  "session": "work",
  "open_browser": true,
  "session_ttl": "168h",
  "herdr_args": []
}
```

Command-line options override the file. Login credentials are intentionally not
accepted in JSON; continue to provide them through the environment:

```bash
export HERDR_TTY_USERNAME='your-name'
read -rsp 'Password: ' HERDR_TTY_PASSWORD
export HERDR_TTY_PASSWORD

herdr-tty --listen 0.0.0.0:7681 --no-open
```

The legacy `HERDR_WEB_USERNAME` and `HERDR_WEB_PASSWORD` names remain accepted
for existing installations. New configuration should use `HERDR_TTY_*`.

When both credential variables exist, `--auth auto` selects form login. Without
credentials it selects `local`, which is rejected unless `--listen` is a
loopback address.

Options:

```text
--config        JSON configuration file
--listen        address to listen on (default 127.0.0.1:7681)
--ttyd          ttyd executable (default ttyd)
--herdr         Herdr executable (default herdr)
--cwd           working directory exposed to Herdr (default current directory)
--max-clients   maximum concurrent ttyd clients (default 3)
--auth          authentication mode: auto, local, form, or native (default auto)
--session       named persistent Herdr session
--session-ttl   login lifetime (default 168h)
--open          open the browser
--no-open       do not open the browser
```

## Mobile behavior

- One-finger drag scrolls the active Herdr view with light inertia.
- Long-press and drag selects terminal text; a temporary Copy button writes the
  xterm selection through Clipboard API or an HTTP-compatible copy event. If
  WebKit rejects both, the selected text is presented in a native copy field.
- Focusing the terminal shows an expandable paste input with stacked Esc and
  Input controls above the virtual keyboard. Input pastes non-empty text and
  then sends a terminal return; with empty text, it sends only the return. The
  virtual keyboard's Return key remains the native way to add a line break.
- At ttyd's reconnect prompt, Input reconnects through ttyd's native Enter-key
  path without clearing the draft. Both actions pause while reconnecting, and
  the preserved draft requires a second Input tap after the connection returns.
- A two-finger tap sends a right mouse click to Herdr.
- The terminal follows `visualViewport` when a mobile keyboard changes the
  visible area, including iOS viewport offsets.
- On iPad Chrome, a small stale focus inset is ignored after the keyboard
  closes, while an open keyboard still resizes the terminal above it.
- iOS virtual Chinese keyboards forward punctuation through a narrow
  `beforeinput`/`input` fallback into ttyd's public xterm instance; ordinary
  text, active composition, desktop keyboards, and Herdr shortcuts keep their
  native paths.
- The browser context menu is suppressed inside the web app.
- No global `keydown`, `keyup`, or `keypress` handler is installed, so Herdr
  keyboard shortcuts continue through ttyd unchanged. A synthetic Enter key is
  dispatched only to ttyd's hidden input when its reconnect prompt is visible.

## Security

The default bind address is loopback, and password-free `local` mode is accepted
only on a loopback listener. Exposing a writable web terminal grants shell
access with your user privileges; use form login and put it behind a network
boundary you trust.

Form login uses an HTTP-only, same-site Cookie signed by an
in-memory random key. Restarting HerdrTTY invalidates existing sessions.
Credentials stay in HerdrTTY and are not passed to ttyd: all `HERDR_TTY_*` and
legacy `HERDR_WEB_*` variables are removed from the environment inherited by
ttyd and Herdr.
TLS key logging through `SSLKEYLOGFILE`, `NSS_KEYLOGFILE`, or Node's
`--tls-keylog` option is not inherited by ttyd, Herdr, or their descendants.
API keys, proxy settings, and other ordinary user environment remain intact.

The original ttyd Basic Authentication mode remains available with
`--auth native`. Native mode receives `user:password` as a ttyd process
argument and may expose the credential to local process inspection. Do not
reuse an important password with that mode.

## Scope

HerdrTTY serves the browser-to-Herdr application path. VPNs, tunnels, port
forwarding, DNS, firewall rules, and reverse proxies are outside this initial
stage.

## License

MIT
