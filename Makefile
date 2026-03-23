-include buildconfig.mk

# Anchor to Makefile directory so VERSION is found even when make is invoked
# from a subdirectory (e.g. go generate, IDE integrations).
REPO_ROOT   := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Version — read from VERSION file (SSOT). Do not hardcode here.
# Format: vX.Y.Z  (v prefix is required; matches git tag convention)
VERSION     := $(shell cat "$(REPO_ROOT)VERSION" 2>/dev/null | tr -d '[:space:]' || echo "dev")
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
NAME        ?= dimlox
MAIN        ?= ./cmd/dimlox

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.buildTime=$(BUILD_TIME) \
	-X main.gitCommit=$(GIT_COMMIT)

GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
EXT     :=
ifeq ($(GOOS),windows)
EXT := .exe
endif

INSTALL_PREFIX  ?= $(HOME)
ifeq ($(GOOS),windows)
INSTALL_BINDIR  ?= $(if $(APPDATA),$(APPDATA)/dimlox/bin,$(INSTALL_PREFIX)/AppData/Roaming/dimlox/bin)
else
INSTALL_BINDIR  ?= $(INSTALL_PREFIX)/.local/bin
endif
INSTALL_TARGET  ?= $(INSTALL_BINDIR)/$(NAME)$(EXT)
BUILD_ARTIFACT  := bin/$(NAME)_$(GOOS)_$(GOARCH)$(EXT)

# Repo-local tool bin
BIN_DIR := $(CURDIR)/bin

# Pinned tool versions (existing installs at or above are kept as-is)
SFETCH_VERSION  ?= v0.4.5
GONEAT_VERSION  ?= v0.5.8

# Tool paths — prefer repo-local, fall back to PATH
SFETCH  = $(shell [ -x "$(BIN_DIR)/sfetch" ]  && echo "$(BIN_DIR)/sfetch"  || command -v sfetch  2>/dev/null)
GONEAT  = $(shell [ -x "$(BIN_DIR)/goneat" ]  && echo "$(BIN_DIR)/goneat"  || command -v goneat  2>/dev/null)

.PHONY: all help build build-all build-windows test check fmt vet lint assess \
        install clean version version-check version-set \
        version-patch version-minor version-major \
        precommit prepush bootstrap tools

all: build

help: ## Show this help
	@echo "dimlox — dimension table manager and large-file cloud tool"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Current version: $(VERSION)"

# -----------------------------------------------------------------------------
# Build
# -----------------------------------------------------------------------------

build: ## Build for current platform
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags="$(LDFLAGS)" \
		-trimpath \
		-o $(BUILD_ARTIFACT) $(MAIN)
	@echo "[ok] Built $(BUILD_ARTIFACT)"

build-all: ## Build for linux/darwin/windows × amd64/arm64
	@mkdir -p dist/release
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-darwin-amd64  $(MAIN)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-darwin-arm64  $(MAIN)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-linux-amd64   $(MAIN)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-linux-arm64   $(MAIN)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-windows-amd64.exe $(MAIN)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-windows-arm64.exe $(MAIN)
	@echo "[ok] Built all targets to dist/release/"

build-windows: ## Cross-compile for Windows amd64 and arm64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" $(MAIN)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" $(MAIN)
	@echo "[ok] Windows cross-compiles passed"

# -----------------------------------------------------------------------------
# Quality (the four gates referenced in AGENTS.md)
# -----------------------------------------------------------------------------

fmt: ## Format Go source
	go fmt ./...

vet: ## Run go vet
	go vet ./...

RACE := $(shell go env CGO_ENABLED 2>/dev/null)
ifeq ($(RACE),1)
RACE_FLAG := -race
else
RACE_FLAG :=
endif

test: ## Run tests (race detector when CGO_ENABLED=1)
ifeq ($(GOOS),windows)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test-windows.ps1 "$(RACE_FLAG)"
else
	go test -v $(RACE_FLAG) ./...
endif

check: fmt vet test ## fmt + vet + test (full local quality gate)
	@echo "[ok] All checks passed"

# Lint via goneat if available, fall back to vet
lint: ## Run linters (goneat assess or go vet fallback)
	@if [ -n "$(GONEAT)" ]; then \
		$(GONEAT) assess --categories lint --check; \
	else \
		echo "[!!] goneat not found, falling back to go vet"; \
		go vet ./...; \
	fi

# Full assess (format + lint + security) — requires goneat
assess: ## Run goneat assess (format, lint, security)
	@if [ -z "$(GONEAT)" ]; then echo "[!!] goneat not found (run 'make bootstrap')"; exit 1; fi
	$(GONEAT) assess --categories format,lint,security --format concise

# -----------------------------------------------------------------------------
# Pre-commit / Pre-push
# -----------------------------------------------------------------------------

precommit: fmt lint vet test build ## fmt + lint + vet + test + build; goneat assess at critical
	@if [ -n "$(GONEAT)" ]; then \
		$(GONEAT) assess --categories format,lint,security --fail-on critical; \
	else \
		echo "[!!] goneat not found — skipping assess (run 'make bootstrap')"; \
	fi
	@echo "[ok] Pre-commit checks passed"

prepush: precommit build-all ## precommit + build-all + full assess at high; requires clean tree
	@if [ -z "$(GONEAT)" ]; then echo "[!!] goneat not found (run 'make bootstrap')"; exit 1; fi
	$(GONEAT) assess --categories format,lint,security --fail-on high
	@echo "[ok] Pre-push checks passed"

# -----------------------------------------------------------------------------
# Bootstrap (goneat + optional sfetch)
# -----------------------------------------------------------------------------

bootstrap: ## Install dev tools via trust chain: curl → sfetch → goneat → tools
	@echo "Bootstrapping dimlox development environment..."
	@echo ""
	@# Step 0: curl is the trust anchor
	@if ! command -v curl >/dev/null 2>&1; then \
		echo "[!!] curl not found (required for bootstrap)"; \
		echo "  macOS:  brew install curl"; \
		echo "  Ubuntu: sudo apt install curl"; \
		exit 1; \
	fi
	@echo "[ok] curl found"
	@echo ""
	@# Step 1: Install sfetch via curl
	@mkdir -p "$(BIN_DIR)"
	@if [ ! -x "$(BIN_DIR)/sfetch" ] && ! command -v sfetch >/dev/null 2>&1; then \
		echo "[..] Installing sfetch $(SFETCH_VERSION) via curl..."; \
		curl -fsSL https://github.com/3leaps/sfetch/releases/download/$(SFETCH_VERSION)/install-sfetch.sh | bash -s -- --dest "$(BIN_DIR)"; \
	else \
		echo "[ok] sfetch already installed"; \
	fi
	@SFETCH_BIN=""; \
	if [ -x "$(BIN_DIR)/sfetch" ]; then SFETCH_BIN="$(BIN_DIR)/sfetch"; \
	elif command -v sfetch >/dev/null 2>&1; then SFETCH_BIN="$$(command -v sfetch)"; fi; \
	if [ -z "$$SFETCH_BIN" ]; then echo "[!!] sfetch installation failed"; exit 1; fi; \
	echo "[ok] sfetch: $$SFETCH_BIN"
	@echo ""
	@# Step 2: Install goneat via sfetch
	@SFETCH_BIN=""; \
	if [ -x "$(BIN_DIR)/sfetch" ]; then SFETCH_BIN="$(BIN_DIR)/sfetch"; \
	elif command -v sfetch >/dev/null 2>&1; then SFETCH_BIN="$$(command -v sfetch)"; fi; \
	if [ ! -x "$(BIN_DIR)/goneat" ] && ! command -v goneat >/dev/null 2>&1; then \
		echo "[..] Installing goneat $(GONEAT_VERSION) via sfetch..."; \
		$$SFETCH_BIN --repo fulmenhq/goneat --tag $(GONEAT_VERSION) --dest-dir "$(BIN_DIR)"; \
	else \
		echo "[ok] goneat already installed"; \
	fi
	@GONEAT_BIN=""; \
	if [ -x "$(BIN_DIR)/goneat" ]; then GONEAT_BIN="$(BIN_DIR)/goneat"; \
	elif command -v goneat >/dev/null 2>&1; then GONEAT_BIN="$$(command -v goneat)"; fi; \
	if [ -z "$$GONEAT_BIN" ]; then echo "[!!] goneat installation failed"; exit 1; fi; \
	echo "[ok] goneat: $$($$GONEAT_BIN version 2>&1 | head -n1)"
	@echo ""
	@# Step 3: Install foundation + go + security tools via goneat
	@echo "[..] Installing tools via goneat..."
	@GONEAT_BIN=""; \
	if [ -x "$(BIN_DIR)/goneat" ]; then GONEAT_BIN="$(BIN_DIR)/goneat"; \
	elif command -v goneat >/dev/null 2>&1; then GONEAT_BIN="$$(command -v goneat)"; fi; \
	$$GONEAT_BIN doctor tools --scope foundation --install --yes --no-cooling 2>/dev/null || \
		echo "[!!] Some foundation tools may need manual installation (see goneat doctor tools)"; \
	$$GONEAT_BIN doctor tools --scope go --install --yes --no-cooling 2>/dev/null || \
		echo "[!!] Some go tools may need manual installation"; \
	$$GONEAT_BIN doctor tools --scope security --install --yes --no-cooling 2>/dev/null || \
		echo "[!!] Some security tools may need manual installation"
	@echo ""
	@echo "[ok] Bootstrap complete"

tools: ## Check required tools
	@if command -v go >/dev/null 2>&1; then echo "[ok] go: $$(go version | cut -d' ' -f3)"; else echo "[!!] go not found"; fi
	@if [ -n "$(GONEAT)" ]; then echo "[ok] goneat: $$($(GONEAT) version 2>&1 | head -n1)"; else echo "[--] goneat: not found (optional, run 'make bootstrap')"; fi
	@if command -v git >/dev/null 2>&1; then echo "[ok] git: $$(git --version | cut -d' ' -f3)"; else echo "[!!] git not found"; fi

# -----------------------------------------------------------------------------
# Install / Clean
# -----------------------------------------------------------------------------

install: build ## Install dimlox to INSTALL_BINDIR
	@mkdir -p "$(INSTALL_BINDIR)"
	cp "$(BUILD_ARTIFACT)" "$(INSTALL_TARGET)"
	chmod 755 "$(INSTALL_TARGET)"
	@echo "[ok] Installed to $(INSTALL_TARGET)"

clean: ## Remove build artifacts and Go build cache
	rm -rf bin/ dist/ coverage.out
	go clean -cache
	@echo "[ok] Cleaned"

# -----------------------------------------------------------------------------
# Version management (VERSION file is SSOT — do not edit directly)
# -----------------------------------------------------------------------------

version: ## Print current version
	@echo "$(VERSION)"

version-check: ## Print current version (verbose)
	@echo "Current version: $(VERSION)"
	@echo "Git commit:      $(GIT_COMMIT)"
	@echo "Build time:      $(BUILD_TIME)"

version-set: ## Set version (usage: make version-set V=vX.Y.Z)
	@if [ -z "$(V)" ]; then echo "usage: make version-set V=vX.Y.Z" >&2; exit 1; fi
	@printf '%s\n' "$(V)" > "$(REPO_ROOT)VERSION"
	@echo "[ok] Version set to $(V)"

version-patch: ## Bump patch version (v0.1.0 → v0.1.1)
	@current=$$(cat "$(REPO_ROOT)VERSION" | tr -d '[:space:]'); \
	major=$$(echo $$current | cut -d. -f1); \
	minor=$$(echo $$current | cut -d. -f2); \
	patch=$$(echo $$current | cut -d. -f3); \
	newver="$$major.$$minor.$$((patch + 1))"; \
	printf '%s\n' "$$newver" > "$(REPO_ROOT)VERSION"; \
	echo "[ok] $$current → $$newver"

version-minor: ## Bump minor version (v0.1.0 → v0.2.0)
	@current=$$(cat "$(REPO_ROOT)VERSION" | tr -d '[:space:]'); \
	major=$$(echo $$current | cut -d. -f1); \
	minor=$$(echo $$current | cut -d. -f2); \
	newver="$$major.$$((minor + 1)).0"; \
	printf '%s\n' "$$newver" > "$(REPO_ROOT)VERSION"; \
	echo "[ok] $$current → $$newver"

version-major: ## Bump major version (v0.1.0 → v1.0.0)
	@current=$$(cat "$(REPO_ROOT)VERSION" | tr -d '[:space:]'); \
	prefix=$$(echo $$current | cut -d. -f1 | grep -o '^v'); \
	majornum=$$(echo $$current | cut -d. -f1 | tr -d 'v'); \
	newver="$${prefix}$$((majornum + 1)).0.0"; \
	printf '%s\n' "$$newver" > "$(REPO_ROOT)VERSION"; \
	echo "[ok] $$current → $$newver"
