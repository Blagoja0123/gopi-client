# webkit2_41 is needed on distros that ship only webkit2gtk-4.1 (Fedora 40+,
# Ubuntu 24.04+). Override with `make build TAGS=` elsewhere.
TAGS ?= webkit2_41
WAILS ?= $(shell command -v wails || echo $(HOME)/go/bin/wails)

.PHONY: dev build test test-go test-frontend check package-linux

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

# Build .deb/.rpm locally the same way the release workflow does
# (needs nfpm: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest).
package-linux: build
	mkdir -p dist
	VERSION=$${VERSION:-0.0.0-dev} GOARCH=$${GOARCH:-amd64} nfpm package -f build/linux/nfpm.yaml -p deb -t dist/gopi-client_linux_amd64.deb
	VERSION=$${VERSION:-0.0.0-dev} GOARCH=$${GOARCH:-amd64} nfpm package -f build/linux/nfpm.yaml -p rpm -t dist/gopi-client_linux_amd64.rpm
