# Checks for both modules. The lint configuration and pinned tool versions
# follow berad-sdk's strict policy: zero findings, no baseline.

GO ?= go
MODULES := engine editor

TOOLS_DIR := $(CURDIR)/.cache/tools
GOLANGCI_LINT_VERSION := 2.15.0
GOIMPORTS_VERSION := v0.37.0
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint
GOIMPORTS := $(TOOLS_DIR)/goimports

EDITOR_BIN := .cache/bin/berad-editor
DEMO_PROJECT ?= .cache/demo

# local.mk, never committed, holds settings for this computer alone, such as
# GAME_PROJECT, the project make game opens.
-include local.mk

.PHONY: help tools tools-check format format-check lint vet test check editor blank demo game gui-test latency

help:
	@echo "make tools         install the pinned golangci-lint and goimports (network)"
	@echo "make format        format all Go source"
	@echo "make format-check  fail on formatting or import differences"
	@echo "make lint          strict lint, zero findings"
	@echo "make vet           go vet"
	@echo "make test          tests with the race detector"
	@echo "make check         all of the above except formatting changes"
	@echo "make editor        build berad-editor into .cache/bin"
	@echo "make blank         open a new, empty project, a fresh one every time"
	@echo "make demo          open the sample project, kept between runs in $(DEMO_PROJECT)"
	@echo "make game          open your game's project, GAME_PROJECT in local.mk"
	@echo "make gui-test      run the tests that open the real editor window"
	@echo "make latency       measure strict-mode command-to-event time"

tools:
	GOBIN=$(TOOLS_DIR) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION)
	GOBIN=$(TOOLS_DIR) $(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

tools-check:
	@test -x $(GOLANGCI_LINT) || { echo "missing golangci-lint: run make tools"; exit 1; }
	@test -x $(GOIMPORTS) || { echo "missing goimports: run make tools"; exit 1; }
	@$(GOLANGCI_LINT) version | grep -q "version $(GOLANGCI_LINT_VERSION)" || { echo "golangci-lint is not $(GOLANGCI_LINT_VERSION): run make tools"; exit 1; }

format: tools-check
	$(GOIMPORTS) -w $(MODULES)

format-check: tools-check
	@out=$$($(GOIMPORTS) -l $(MODULES)); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

lint: tools-check
	@for m in $(MODULES); do (cd $$m && $(GOLANGCI_LINT) run --config ../.golangci.yml ./...) || exit 1; done

vet:
	@for m in $(MODULES); do $(GO) -C $$m vet ./... || exit 1; done

test:
	@for m in $(MODULES); do $(GO) -C $$m test -race ./... || exit 1; done

check: format-check lint vet test

editor:
	$(GO) -C editor build -o ../$(EDITOR_BIN) ./cmd/berad-editor

blank: editor
	@dir=$$(mktemp -d -t berad-blank); echo "Blank project in $$dir/project"; \
		./$(EDITOR_BIN) open --project $$dir/project

demo: editor
	./$(EDITOR_BIN) open --project $(DEMO_PROJECT) --sample

game: editor
	@test -n "$(GAME_PROJECT)" || { echo "Set GAME_PROJECT in local.mk, for example: GAME_PROJECT = ../my-game/project"; exit 1; }
	./$(EDITOR_BIN) open --project $(GAME_PROJECT)

gui-test:
	BERAD_GUI_TESTS=1 $(GO) -C editor test -count=1 -v ./cmd/berad-editor/

latency:
	$(GO) -C editor test -count=1 -run StrictModeLatency -v ./orchestrator/
