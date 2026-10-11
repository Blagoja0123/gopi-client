# API Client

A desktop API client built with Go and [Wails](https://wails.io) (Svelte + TypeScript frontend). It supports:

- **REST/HTTP** requests: params, headers, JSON/text/XML/form/multipart bodies, bearer/basic/API-key auth, timeouts, redirects, cookies, and cURL import/export.
- **gRPC** calls: unary, server, client and bidi streaming, with methods discovered via server reflection or local `.proto` files. No codegen needed.
- **WebSockets**: an interactive session with a message log and saved messages.
- **Webhooks**: a local receiver/inspector, replay to any URL, saving captures as requests, and GitHub/Stripe/HMAC signing and verification.
- **Environments** with `{{variables}}` and **secrets that never get committed**.
- **OpenAPI 3 / Swagger 2 import**.
- **Quick (scratch) requests** that need no workspace or collection.

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
