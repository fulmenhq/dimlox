-include buildconfig.mk

# Anchor to Makefile directory so VERSION is found even when make is invoked
# from a subdirectory (e.g. go generate, IDE integrations).
REPO_ROOT   := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
WINDOWS_HOST := $(filter Windows_NT,$(OS))
ifeq ($(WINDOWS_HOST),Windows_NT)
SHELL := powershell.exe
.SHELLFLAGS := -NoProfile -ExecutionPolicy Bypass -Command
endif

# Version — read from VERSION file (SSOT). Do not hardcode here.
# Format: vX.Y.Z  (v prefix is required; matches git tag convention)
ifeq ($(WINDOWS_HOST),Windows_NT)
VERSION     := $(shell powershell -NoProfile -ExecutionPolicy Bypass -File "$(REPO_ROOT)scripts/make-windows.ps1" print-version "$(REPO_ROOT)")
BUILD_TIME  := $(shell powershell -NoProfile -ExecutionPolicy Bypass -File "$(REPO_ROOT)scripts/make-windows.ps1" print-build-time)
GIT_COMMIT  := $(shell powershell -NoProfile -ExecutionPolicy Bypass -File "$(REPO_ROOT)scripts/make-windows.ps1" print-git-commit "$(REPO_ROOT)")
else
VERSION     := $(shell cat "$(REPO_ROOT)VERSION" 2>/dev/null | tr -d '[:space:]' || echo "dev")
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
endif
NAME        ?= dimlox
MAIN        ?= ./cmd/dimlox

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.buildTime=$(BUILD_TIME) \
	-X main.gitCommit=$(GIT_COMMIT)

ifeq ($(WINDOWS_HOST),Windows_NT)
GOOS    ?= $(shell powershell -NoProfile -ExecutionPolicy Bypass -File "$(REPO_ROOT)scripts/make-windows.ps1" go-env GOOS)
GOARCH  ?= $(shell powershell -NoProfile -ExecutionPolicy Bypass -File "$(REPO_ROOT)scripts/make-windows.ps1" go-env GOARCH)
else
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
endif
EXT     :=
ifeq ($(GOOS),windows)
EXT := .exe
endif

ifeq ($(WINDOWS_HOST),Windows_NT)
INSTALL_PREFIX  ?= $(if $(USERPROFILE),$(USERPROFILE),$(HOME))
else
INSTALL_PREFIX  ?= $(HOME)
endif
ifeq ($(GOOS),windows)
INSTALL_BINDIR  ?= $(if $(LOCALAPPDATA),$(LOCALAPPDATA)/Programs/dimlox/bin,$(INSTALL_PREFIX)/AppData/Local/Programs/dimlox/bin)
else
INSTALL_BINDIR  ?= $(INSTALL_PREFIX)/.local/bin
endif
INSTALL_TARGET  ?= $(INSTALL_BINDIR)/$(NAME)$(EXT)
BUILD_ARTIFACT  := bin/$(NAME)_$(GOOS)_$(GOARCH)$(EXT)

# Repo-local tool bin
BIN_DIR := $(CURDIR)/bin
DIST_RELEASE ?= dist/release
RELEASE_TAG ?= $(or $(DIMLOX_RELEASE_TAG),$(VERSION))
SIGNING_ENV_PREFIX ?= DIMLOX
SIGNING_APP_NAME ?= $(NAME)

# Pinned tool versions (existing installs at or above are kept as-is)
SFETCH_VERSION  ?= v0.4.5
GONEAT_VERSION  ?= v0.5.8

# Tool paths — prefer repo-local, fall back to PATH
ifeq ($(WINDOWS_HOST),Windows_NT)
SFETCH  = $(shell powershell -NoProfile -ExecutionPolicy Bypass -File "$(REPO_ROOT)scripts/make-windows.ps1" find-tool "$(REPO_ROOT)" "sfetch")
GONEAT  = $(shell powershell -NoProfile -ExecutionPolicy Bypass -File "$(REPO_ROOT)scripts/make-windows.ps1" find-tool "$(REPO_ROOT)" "goneat")
else
SFETCH  = $(shell [ -x "$(BIN_DIR)/sfetch" ]  && echo "$(BIN_DIR)/sfetch"  || command -v sfetch  2>/dev/null)
GONEAT  = $(shell [ -x "$(BIN_DIR)/goneat" ]  && echo "$(BIN_DIR)/goneat"  || command -v goneat  2>/dev/null)
endif

.PHONY: all help build build-all build-windows test test-short check fmt vet lint assess \
        install install-path clean version version-check version-set \
        version-patch version-minor version-major \
        precommit prepush bootstrap tools \
        release-clean release-build release-checksums release-sign release-download \
        release-export-keys release-verify-keys release-verify-checksums release-notes \
        release-upload release-upload-provenance release-upload-all \
        update-homebrew-formula update-scoop-manifest

all: build

help: ## Show this help
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 help "$(REPO_ROOT)Makefile" "$(VERSION)"
else
	@echo "dimlox — dimension table manager and large-file cloud tool"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Current version: $(VERSION)"
endif

# -----------------------------------------------------------------------------
# Build
# -----------------------------------------------------------------------------

build: ## Build for current platform
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 build "$(GOOS)" "$(GOARCH)" "$(LDFLAGS)" "$(MAIN)" "$(BUILD_ARTIFACT)"
else
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags="$(LDFLAGS)" \
		-trimpath \
		-o $(BUILD_ARTIFACT) $(MAIN)
	@echo "[ok] Built $(BUILD_ARTIFACT)"
endif

build-all: ## Build for linux/darwin/windows × amd64/arm64
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 build-all "$(NAME)" "$(LDFLAGS)" "$(MAIN)"
else
	@mkdir -p dist/release
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-darwin-amd64  $(MAIN)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-darwin-arm64  $(MAIN)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-linux-amd64   $(MAIN)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-linux-arm64   $(MAIN)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-windows-amd64.exe $(MAIN)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/release/$(NAME)-windows-arm64.exe $(MAIN)
	@echo "[ok] Built all targets to dist/release/"
endif

build-windows: ## Cross-compile for Windows amd64 and arm64
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 build-windows "$(LDFLAGS)" "$(MAIN)"
else
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" $(MAIN)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" $(MAIN)
	@echo "[ok] Windows cross-compiles passed"
endif

# -----------------------------------------------------------------------------
# Quality (the four gates referenced in AGENTS.md)
# -----------------------------------------------------------------------------

fmt: ## Format Go source
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 go fmt ./...
else
	go fmt ./...
endif

vet: ## Run go vet
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 go vet ./...
else
	go vet ./...
endif

ifeq ($(WINDOWS_HOST),Windows_NT)
RACE_FLAG :=
else
RACE := $(shell go env CGO_ENABLED 2>/dev/null)
ifeq ($(RACE),1)
RACE_FLAG := -race
else
RACE_FLAG :=
endif
endif

test: ## Run tests (race detector when CGO_ENABLED=1)
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test-windows.ps1
else
	go test -v $(RACE_FLAG) ./...
endif

test-short: ## Run tests without live network (CI-safe)
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test-windows.ps1 -Short
else
	go test -v -short ./...
endif

check: fmt vet test ## fmt + vet + test (full local quality gate)
	@echo "[ok] All checks passed"

# Lint via goneat if available, fall back to vet
lint: ## Run linters (goneat assess or go vet fallback)
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 lint "$(GONEAT)"
else
	@if [ -n "$(GONEAT)" ]; then \
		$(GONEAT) assess --categories lint --check; \
	else \
		echo "[!!] goneat not found, falling back to go vet"; \
		go vet ./...; \
	fi
endif

# Full assess (format + lint + security) — requires goneat
assess: ## Run goneat assess (format, lint, security)
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 assess "$(GONEAT)"
else
	@if [ -z "$(GONEAT)" ]; then echo "[!!] goneat not found (run 'make bootstrap')"; exit 1; fi
	$(GONEAT) assess --categories format,lint,security --format concise
endif

# -----------------------------------------------------------------------------
# Pre-commit / Pre-push
# -----------------------------------------------------------------------------

precommit: fmt lint vet test build ## fmt + lint + vet + test + build; goneat assess at critical
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 precommit "$(GONEAT)"
else
	@if [ -n "$(GONEAT)" ]; then \
		$(GONEAT) assess --categories format,lint,security --fail-on critical; \
	else \
		echo "[!!] goneat not found — skipping assess (run 'make bootstrap')"; \
	fi
	@echo "[ok] Pre-commit checks passed"
endif

prepush: precommit build-all ## precommit + build-all + full assess at high; requires clean tree
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 prepush "$(GONEAT)"
else
	@if [ -z "$(GONEAT)" ]; then echo "[!!] goneat not found (run 'make bootstrap')"; exit 1; fi
	$(GONEAT) assess --categories format,lint,security --fail-on high
	@echo "[ok] Pre-push checks passed"
endif

# -----------------------------------------------------------------------------
# Bootstrap (goneat + optional sfetch)
# -----------------------------------------------------------------------------

bootstrap: ## Install dev tools via trust chain: curl → sfetch → goneat → tools
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 bootstrap "$(GONEAT)"
else
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
endif

tools: ## Check required tools
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 tools "$(GONEAT)"
else
	@if command -v go >/dev/null 2>&1; then echo "[ok] go: $$(go version | cut -d' ' -f3)"; else echo "[!!] go not found"; fi
	@if [ -n "$(GONEAT)" ]; then echo "[ok] goneat: $$($(GONEAT) version 2>&1 | head -n1)"; else echo "[--] goneat: not found (optional, run 'make bootstrap')"; fi
	@if command -v git >/dev/null 2>&1; then echo "[ok] git: $$(git --version | cut -d' ' -f3)"; else echo "[!!] git not found"; fi
endif

# -----------------------------------------------------------------------------
# Install / Clean
# -----------------------------------------------------------------------------

install: build ## Install dimlox to INSTALL_BINDIR
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 install "$(BUILD_ARTIFACT)" "$(INSTALL_TARGET)"
else
	@mkdir -p "$(INSTALL_BINDIR)"
	cp "$(BUILD_ARTIFACT)" "$(INSTALL_TARGET)"
	chmod 755 "$(INSTALL_TARGET)"
	@echo "[ok] Installed to $(INSTALL_TARGET)"
endif

install-path: ## Add INSTALL_BINDIR to user PATH on Windows
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 install-path "$(INSTALL_BINDIR)"
else
	@echo "[ok] $(INSTALL_BINDIR) is the standard install location on this platform"
endif

clean: ## Remove build artifacts and Go build cache
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 clean
else
	rm -rf bin/ dist/ coverage.out
	go clean -cache
	@echo "[ok] Cleaned"
endif

# -----------------------------------------------------------------------------
# Release
# -----------------------------------------------------------------------------

release-clean: ## Remove staged release artifacts
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	rm -rf "$(DIST_RELEASE)"
	mkdir -p "$(DIST_RELEASE)"
	@echo "[ok] Cleaned $(DIST_RELEASE)"
endif

release-build: release-clean build-all release-checksums ## Build release artifacts into dist/release
	@echo "[ok] Release artifacts staged in $(DIST_RELEASE)"

release-checksums: ## Generate checksum manifests in dist/release
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	./scripts/generate-checksums.sh "$(DIST_RELEASE)" "$(NAME)"
endif

release-download: ## Download GitHub release assets for RELEASE_TAG
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	./scripts/release-download.sh "$(RELEASE_TAG)" "$(DIST_RELEASE)"
endif

release-sign: ## Sign checksum manifests (minisign required; PGP optional)
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	SIGNING_ENV_PREFIX="$(SIGNING_ENV_PREFIX)" SIGNING_APP_NAME="$(SIGNING_APP_NAME)" ./scripts/sign-release-manifests.sh "$(RELEASE_TAG)" "$(DIST_RELEASE)"
endif

release-export-keys: ## Export public signing keys into dist/release
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	SIGNING_ENV_PREFIX="$(SIGNING_ENV_PREFIX)" SIGNING_APP_NAME="$(SIGNING_APP_NAME)" ./scripts/export-release-keys.sh "$(DIST_RELEASE)"
endif

release-verify-keys: ## Verify exported public keys are public-only
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	@if [ -f "$(DIST_RELEASE)/$(NAME)-minisign.pub" ]; then ./scripts/verify-minisign-public-key.sh "$(DIST_RELEASE)/$(NAME)-minisign.pub"; else echo "[--] No minisign public key found (skipping)"; fi
	@if [ -f "$(DIST_RELEASE)/fulmenhq-release-signing-key.asc" ]; then ./scripts/verify-public-key.sh "$(DIST_RELEASE)/fulmenhq-release-signing-key.asc"; else echo "[--] No PGP public key found (skipping)"; fi
endif

release-verify-checksums: ## Verify SHA256SUMS and SHA512SUMS against artifacts
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	./scripts/verify-checksums.sh "$(DIST_RELEASE)"
endif

release-notes: ## Copy docs/releases/RELEASE_TAG.md into dist/release
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	@notes_src="docs/releases/$(RELEASE_TAG).md"; notes_dst="$(DIST_RELEASE)/release-notes-$(RELEASE_TAG).md"; \
	if [ ! -f "$$notes_src" ]; then echo "[!!] Missing $$notes_src"; exit 1; fi; \
	mkdir -p "$(DIST_RELEASE)"; \
	cp "$$notes_src" "$$notes_dst"; echo "[ok] Copied $$notes_src -> $$notes_dst"
endif

release-upload: release-upload-provenance ## Upload provenance assets to GitHub release
	@:

release-upload-provenance: release-verify-checksums release-verify-keys ## Upload manifests, signatures, keys, and notes
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	./scripts/release-upload-provenance.sh "$(RELEASE_TAG)" "$(DIST_RELEASE)"
endif

release-upload-all: release-verify-checksums release-verify-keys ## Upload binaries and provenance assets to GitHub release
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Release helper targets are not supported on Windows hosts; use macOS or Linux for release staging."; exit 1
else
	./scripts/release-upload.sh "$(RELEASE_TAG)" "$(DIST_RELEASE)"
endif

update-homebrew-formula: ## Update Homebrew formula in ../homebrew-tap
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Homebrew formula updates are not supported on Windows hosts; use macOS or Linux."; exit 1
else
	@if [ ! -d "../homebrew-tap" ]; then \
		echo "[!!] ../homebrew-tap not found -- clone https://github.com/fulmenhq/homebrew-tap"; \
		exit 1; \
	fi
	@$(MAKE) -C ../homebrew-tap update APP=$(NAME) VERSION=$(VERSION)
	@echo "[ok] Homebrew formula updated -- review and push from ../homebrew-tap"
endif

update-scoop-manifest: ## Update Scoop manifest in ../scoop-bucket
ifeq ($(WINDOWS_HOST),Windows_NT)
	Write-Error "Scoop manifest updates are not supported from this Makefile on Windows hosts; use macOS or Linux."; exit 1
else
	@if [ ! -d "../scoop-bucket" ]; then \
		echo "[--] ../scoop-bucket not found -- skipping (clone https://github.com/fulmenhq/scoop-bucket)"; \
	else \
		$(MAKE) -C ../scoop-bucket update-dimlox VERSION=$(VERSION); \
		echo "[ok] Scoop manifest updated -- review and push from ../scoop-bucket"; \
	fi
endif

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
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 version-set "$(REPO_ROOT)" "$(V)"
else
	@if [ -z "$(V)" ]; then echo "usage: make version-set V=vX.Y.Z" >&2; exit 1; fi
	@printf '%s\n' "$(V)" > "$(REPO_ROOT)VERSION"
	@echo "[ok] Version set to $(V)"
endif

version-patch: ## Bump patch version (v0.1.0 → v0.1.1)
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 version-patch "$(REPO_ROOT)"
else
	@current=$$(cat "$(REPO_ROOT)VERSION" | tr -d '[:space:]'); \
	major=$$(echo $$current | cut -d. -f1); \
	minor=$$(echo $$current | cut -d. -f2); \
	patch=$$(echo $$current | cut -d. -f3); \
	newver="$$major.$$minor.$$((patch + 1))"; \
	printf '%s\n' "$$newver" > "$(REPO_ROOT)VERSION"; \
	echo "[ok] $$current → $$newver"
endif

version-minor: ## Bump minor version (v0.1.0 → v0.2.0)
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 version-minor "$(REPO_ROOT)"
else
	@current=$$(cat "$(REPO_ROOT)VERSION" | tr -d '[:space:]'); \
	major=$$(echo $$current | cut -d. -f1); \
	minor=$$(echo $$current | cut -d. -f2); \
	newver="$$major.$$((minor + 1)).0"; \
	printf '%s\n' "$$newver" > "$(REPO_ROOT)VERSION"; \
	echo "[ok] $$current → $$newver"
endif

version-major: ## Bump major version (v0.1.0 → v1.0.0)
ifeq ($(WINDOWS_HOST),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/make-windows.ps1 version-major "$(REPO_ROOT)"
else
	@current=$$(cat "$(REPO_ROOT)VERSION" | tr -d '[:space:]'); \
	prefix=$$(echo $$current | cut -d. -f1 | grep -o '^v'); \
	majornum=$$(echo $$current | cut -d. -f1 | tr -d 'v'); \
	newver="$${prefix}$$((majornum + 1)).0.0"; \
	printf '%s\n' "$$newver" > "$(REPO_ROOT)VERSION"; \
	echo "[ok] $$current → $$newver"
endif
