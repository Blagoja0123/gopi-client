# webkit2_41 is needed on distros that ship only webkit2gtk-4.1 (Fedora 40+,
# Ubuntu 24.04+). Override with `make build TAGS=` elsewhere.
TAGS ?= webkit2_41
WAILS ?= $(shell command -v wails || echo $(HOME)/go/bin/wails)

.PHONY: dev build test test-go test-frontend check

dev:
	$(WAILS) dev -tags "$(TAGS)"

build:
	$(WAILS) build -tags "$(TAGS)"

test: test-go test-frontend

test-go:
	go test -race ./internal/...

test-frontend:
	cd frontend && npm test

check:
	go vet ./...
	cd frontend && npm run check
