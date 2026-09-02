# Installing HerdrTTY with a coding agent

This guide is an operational contract for a coding agent installing or
upgrading HerdrTTY on a user's machine.

## Guardrails

- Inspect before changing anything. Preserve unrelated files and existing
  configuration.
- Keep HerdrTTY lightweight: use the repository's standard-library Go code and
  native JavaScript/CSS as-is. Do not add runtime dependencies.
- Do not configure VPNs, tunnels, DNS, firewall rules, reverse proxies, WSL
  lifecycle, or other network/platform infrastructure.
- Default to loopback-only access. Do not expose a non-loopback listener unless
  the user explicitly requests it and supplies the authentication decision.
- Never invent, replace, print, or commit credentials. Existing credentials
  must remain unchanged unless the user explicitly asks otherwise.
- Do not discard a dirty worktree. Do not use destructive Git commands.
- Build and install without `sudo` unless the user explicitly requests a
  system-wide installation.

## 1. Preflight

Run these read-only checks:

```bash
command -v git
command -v go
command -v node
command -v herdr
command -v ttyd
go version
node --version
herdr --version
ttyd --version
```

HerdrTTY requires Herdr, ttyd 1.7+, and Go 1.23+ for a source build. If a
requirement is missing, report it and ask before changing the host package
manager or installing unrelated software.

HerdrTTY must be launched from a regular shell or service, not from inside an
existing Herdr pane where `HERDR_ENV` is set.

## 2. Obtain the source

For a fresh checkout:

```bash
git clone https://github.com/dark2momo/herdr-tty.git
cd herdr-tty
```

For an existing checkout, first inspect it:

```bash
git status --short --branch
git remote -v
```

Preserve local changes. Only update a clean checkout whose remote is the
expected repository:

```bash
git pull --ff-only
```

## 3. Verify and build

Use the repository targets:

```bash
make check
make build
```

`make check` runs the Go race tests, vet, a build, JavaScript syntax checking,
and the dependency-free Node behavior tests. Node.js is needed for this check,
but not at runtime.

## 4. Install for the current user

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 bin/herdr-tty "$HOME/.local/bin/herdr-tty"
```

Confirm the installed file matches the build:

```bash
cmp bin/herdr-tty "$HOME/.local/bin/herdr-tty"
```

If `~/.local/bin` is not on `PATH`, report that fact. Do not rewrite shell
startup files without permission.

## 5. Start safely

The safe default is a loopback-only foreground session:

```bash
herdr-tty --no-open
```

It listens on `127.0.0.1:7681` and uses local authentication mode. To attach a
named persistent Herdr session:

```bash
herdr-tty --session work --no-open
```

If the user explicitly requests LAN exposure, use form authentication and let
the user provide the credentials without echoing them:

```bash
export HERDR_TTY_USERNAME='user-supplied-name'
read -rsp 'HerdrTTY password: ' HERDR_TTY_PASSWORD
export HERDR_TTY_PASSWORD
herdr-tty --listen 0.0.0.0:7681 --auth form --no-open
```

This command changes only HerdrTTY's listener. Network reachability and policy
remain outside this project.

For an existing service, inspect its unit, executable path, arguments, and
environment source first. Replace only the HerdrTTY binary, retain the existing
listener and credentials, and use that service's normal restart mechanism.

## 6. Verify and hand off

Verify the configured URL responds and that the launched command still uses the
expected executable, listener, working directory, authentication mode, and
client limit. Do not display credential values during verification.

Report:

- the installed binary path and source commit;
- whether `make check` passed;
- the listener and authentication mode;
- whether an existing service was restarted successfully;
- any step not performed and why.

Do not claim mobile behavior is verified without a real-device check.
