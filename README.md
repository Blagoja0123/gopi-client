# API Client

A desktop API client built with Go and [Wails](https://wails.io) (Svelte + TypeScript frontend). It supports:

- **REST/HTTP** requests: params, headers, JSON/text/XML/form/multipart bodies, bearer/basic/API-key auth, timeouts, redirects, cookies, and cURL import/export.
- **gRPC** calls: unary, server, client and bidi streaming, with methods discovered via server reflection or local `.proto` files. No codegen needed.
- **WebSockets**: an interactive session with a message log and saved messages.
- **Webhooks**: a local receiver/inspector, replay to any URL, saving captures as requests, and GitHub/Stripe/HMAC signing and verification.
- **Environments** with `{{variables}}` and **secrets that never get committed**.
- **OpenAPI 3 / Swagger 2 import**.
- **Quick (scratch) requests** that need no workspace or collection.

## Install

**Windows** (PowerShell). This installs per user, with no admin prompt, and sets up WebView2 if needed:

```powershell
irm https://raw.githubusercontent.com/Blagoja0123/gopi-client/master/scripts/install.ps1 | iex
```

**macOS and Linux**:

```sh
curl -fsSL https://raw.githubusercontent.com/Blagoja0123/gopi-client/master/scripts/install.sh | sh
```

On Linux, the script installs the `.deb` (apt) or `.rpm` (dnf/zypper) package, which pulls in WebKitGTK automatically. On other distros, or with `INSTALL_METHOD=user`, it installs into `~/.local` without sudo. Set `VERSION=v1.2.3` to pin a release.

Every download is checked against the release's `SHA256SUMS`.

You can also download an installer directly from the [latest release](https://github.com/Blagoja0123/gopi-client/releases/latest):

| OS | File |
|---|---|
| Windows | `gopi-client_windows_amd64_setup.exe` (or `_portable.zip`) |
| macOS (Intel + Apple Silicon) | `gopi-client_macos_universal.dmg` |
| Debian / Ubuntu | `sudo apt install ./gopi-client_linux_amd64.deb` (also `arm64`) |
| Fedora / openSUSE | `sudo dnf install ./gopi-client_linux_amd64.rpm` (also `arm64`) |
| Other Linux | `gopi-client_linux_amd64.tar.gz` (needs WebKitGTK 4.1) |

> Until the app is code-signed, a browser-downloaded `.dmg` will be blocked by Gatekeeper the first time it is opened. Right-click the app and choose **Open**, or allow it in **System Settings → Privacy & Security**. The install script avoids this. On Windows, SmartScreen may show "unknown publisher"; choose **More info → Run anyway**.

## Workspaces are plain, git-friendly files

Any folder (typically a git repo) can be a workspace:

```
my-api/
  apiclient.yaml              # name + workspace-wide variables (committed)
  .gitignore                  # gets *.secret.yaml and .apiclient/ added
  environments/
    dev.yaml                  # variables; secret ones listed by name only
    dev.secret.yaml           # secret values (gitignored, local only)
  requests/
    users/
      _folder.yaml            # headers/auth inherited by everything below
      get-user.yaml           # one request per file
```

A request file looks like this:

```yaml
name: Get user
kind: http
method: GET
url: '{{baseUrl}}/users/{{id}}'
headers:
  - key: Accept
    value: application/json
auth:
  type: inherit
```

The files are designed to diff cleanly:

- One request per file.
- Key order is fixed.
- Re-saving an unchanged request produces byte-identical output.
- No timestamps or UI state are written.

Changes on disk (for example after `git pull`) are picked up live.

Variables resolve in this order: environment (including its secrets), then workspace variables. Built-in variables are `{{$uuid}}`, `{{$timestamp}}`, `{{$isoTimestamp}}` and `{{$randomInt}}`.

Scratch requests and app settings live in your user config directory (`~/.config/apiclient`), not in any repo.

## Development

Prerequisites: Go 1.25+, Node 20+, and the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`). On Linux you also need the webkit2gtk 4.1 development package.

```sh
make dev     # hot-reloading app; the UI is also served at http://localhost:34115
make test    # Go tests (with -race) + frontend unit tests (vitest)
make build   # produces build/bin/apiclient
```

Code layout:

| Path | What |
|---|---|
| `internal/model` | On-disk schema |
| `internal/store` | Workspace files, environments + secrets, folder inheritance |
| `internal/vars` | `{{var}}` resolution and secret masking |
| `internal/runner/{httprunner,grpcrunner,wsrunner}` | Protocol runners |
| `internal/webhook` | Receiver, replay, signatures |
| `internal/importer/{openapi,curl}` | Importers |
| `internal/scratch` | One-off requests |
| `app*.go` | Wails bindings |
| `frontend/src` | UI (`lib/` holds unit-tested helpers) |

## Releasing

```sh
git tag v1.2.3 && git push origin v1.2.3     # or v1.3.0-rc.1 for a pre-release
```

[`.github/workflows/release.yml`](.github/workflows/release.yml) then:

1. Runs CI.
2. Builds on native runners: Linux amd64/arm64, Windows and macOS universal.
3. Packages each platform:
   - Linux: `.deb`, `.rpm` and `.tar.gz` via [nfpm](https://nfpm.goreleaser.com), configured in `build/linux/nfpm.yaml`.
   - Windows: an NSIS installer and a portable `.zip`.
   - macOS: a `.dmg` and a `.zip`.
4. Publishes a GitHub release with `SHA256SUMS`.
5. Installs the published release on Ubuntu, Fedora, macOS and Windows with the install scripts, as a smoke test.

Asset names carry no version, so `releases/latest/download/<file>` links always work. Use `make package-linux` to build the Linux packages locally.

**macOS signing (optional).** Set these repository secrets and releases are signed and notarized automatically:

- `MACOS_CERTIFICATE`: a base64-encoded Developer ID Application `.p12`
- `MACOS_CERTIFICATE_PASSWORD`
- `MACOS_SIGN_IDENTITY`, e.g. `Developer ID Application: Name (TEAMID)`
- `APPLE_ID`
- `APPLE_TEAM_ID`
- `APPLE_APP_PASSWORD`: an app-specific password
