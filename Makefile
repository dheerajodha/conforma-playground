PORT ?= 8090
URL ?= http://localhost:$(PORT)
WEB_DIR ?= web

# Works on Linux and macOS, and python3 is already needed to serve. Override
# with BROWSER='open' or BROWSER=xdg-open to use a platform opener directly.
BROWSER ?= python3 -m webbrowser

WASM_EXEC := $(shell go env GOROOT)/lib/wasm/wasm_exec.js

.PHONY: all build serve view test clean

all: build

## Build the WASM artifact and copy Go's wasm_exec.js runtime helper
build: $(WEB_DIR)/playground.wasm $(WEB_DIR)/wasm_exec.js

$(WEB_DIR)/playground.wasm: $(shell find cmd pkg -name '*.go') go.mod go.sum
	GOOS=js GOARCH=wasm go build -o $@ ./cmd/wasm

$(WEB_DIR)/wasm_exec.js: $(WASM_EXEC)
	cp "$<" $@

## Serve the playground locally
serve: build
	python3 -m http.server $(PORT) -d $(WEB_DIR)

## Open the running playground in a browser
view:
	$(BROWSER) "$(URL)"

test:
	go test ./...

clean:
	rm -f $(WEB_DIR)/playground.wasm $(WEB_DIR)/wasm_exec.js
