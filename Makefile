# Go runs in Docker unless you have Go 1.27 or newer and pass GO=go
# (`make test GO=go`). See docs/DEVELOPMENT.md.

GO_IMAGE ?= golang:1.27.3-bookworm
DOCKER   ?= docker run --rm -v "$(CURDIR)":/src -w /src \
	-v minideck-gocache:/root/.cache -v minideck-gomod:/go/pkg/mod
GO       ?= $(DOCKER) $(GO_IMAGE) go
ifeq ($(GO),go)
RUN      :=
GOFMT    := gofmt
else
RUN      := $(DOCKER) $(GO_IMAGE)
GOFMT    := $(DOCKER) $(GO_IMAGE) gofmt
endif
DIRS     := ui tnet secrets cache appdir atomicfile procs qr fb version tools

.PHONY: help test vet fmt lint check icons bundle clean

help: ## list the targets
	@awk -F':.*## ' '/^[a-z-]+:.*## /{printf "  %-8s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## unit tests
	$(GO) test ./...

vet: ## go vet, for this computer and for the device (ARMv7)
	$(GO) vet ./...
	$(RUN) env GOOS=linux GOARCH=arm GOARM=7 go vet ./...

fmt: ## gofmt every Go file
	$(GOFMT) -w $(DIRS)

lint: ## staticcheck
	$(GO) run honnef.co/go/tools/cmd/staticcheck@2026.2.2 ./...

check: ## what CI checks
	@test -z "$$($(GOFMT) -l $(DIRS))" || { echo "gofmt needed: make fmt"; exit 1; }
	$(MAKE) --no-print-directory vet test

icons: ## draw the app icons into icons/
	python3 scripts/make-icons.py

bundle: ## the all-apps release from the apps' latest releases, into dist/
	$(GO) run ./tools/bundle -o dist

clean:
	rm -rf build dist
