# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors

BINARY      := siafu
BIN_DIR     := bin
MODULE      := github.com/gac982/siafu
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)
GOFLAGS_BUILD := -trimpath
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build build-all install test race lint fmt coverage check vuln logo demo demo-build e2e tui-smoke preflight stress stress-open web-smoke clean

all: check build

logo:
	python3 scripts/render-logo.py --crop 1125x1451+461+304 --width 44 --rows 28 --no-snap --out internal/tui/assets/logo.ans
	python3 scripts/render-logo.py --crop 420x480+810+480 --width 24 --rows 12 --no-snap --out internal/tui/assets/mark.ans

build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS_BUILD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/siafu

build-all:
	@mkdir -p $(BIN_DIR)
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(GOFLAGS_BUILD) -ldflags "$(LDFLAGS)" \
			-o $(BIN_DIR)/$(BINARY)-$$os-$$arch ./cmd/siafu; \
	done

install:
	CGO_ENABLED=0 go install $(GOFLAGS_BUILD) -ldflags "$(LDFLAGS)" ./cmd/siafu

test:
	go test ./...

race:
	go test -race ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }

fmt:
	gofmt -w .

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

check: lint race

GOVULNCHECK ?= $(shell command -v govulncheck 2>/dev/null || echo $(shell go env GOPATH)/bin/govulncheck)

vuln: build
	$(GOVULNCHECK) ./...
	$(GOVULNCHECK) -mode binary ./$(BIN_DIR)/$(BINARY)

demo-build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/siafu-demo ./examples/demo

demo: demo-build
	./$(BIN_DIR)/siafu-demo

e2e:
	bash scripts/e2e.sh

tui-smoke: build demo-build
	python3 scripts/tui-smoke.py

web-smoke: build demo-build
	bash scripts/web-smoke.sh

preflight:
	bash scripts/preflight.sh

stress: build demo-build
	bash scripts/preflight.sh
	bash scripts/stress-15k.sh

stress-open: build demo-build
	bash scripts/preflight.sh
	bash scripts/stress-open.sh

clean:
	rm -rf $(BIN_DIR) coverage.out
