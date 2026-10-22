# Conforma Playground

Browser-local Conforma policy playground PoC using Go, WebAssembly, and OPA.

This spike proves a narrow policy authoring loop: inline Rego plus inline
JSON/YAML input evaluated by OPA from Go compiled to WebAssembly.

The UI uses the Conforma upstream color palette and logo treatment.

It intentionally does not run the full Conforma CLI command tree. The current
goal is to learn whether the browser can support the core `ec validate input`
authoring experience before pulling in image validation, OCI fetching, Sigstore,
Rekor, or remote policy source loading.

## Build and Run

From the repository root:

```bash
make serve
```

That builds `web/playground.wasm`, copies Go's `wasm_exec.js` runtime helper,
and serves `web/` on <http://localhost:8090>. Override the port with
`make serve PORT=9000`.

In another shell, `make view` opens that URL in your browser. Other targets are
`make build`, `make test`, and `make clean`.

`playground.wasm` and `wasm_exec.js` are generated local artifacts and are
ignored by Git.

## GitHub Pages

The `Deploy GitHub Pages` workflow builds the WASM artifact, copies Go's
`wasm_exec.js` runtime helper, and publishes the static bundle from `dist/`.

## Repository Layout

- `pkg/playground/` contains the browser-safe playground evaluator package.
- `cmd/wasm/` exposes the evaluator package to JavaScript when compiled to WASM.
- `web/` contains the static playground UI and visual assets.
- `fixtures/` contains example policies, data, and inputs for parity checks.

## What It Supports

- A single pasted Rego module.
- Multiple pasted Rego modules separated by `# file: <name>.rego` markers.
- Pasted JSON or YAML input.
- Optional pasted JSON or YAML data available under `data`.
- `deny`, `violation`, and `warn` rules in the module package.
- OPA `# METADATA` comments for rule title and `custom.short_name`.
- A Conforma-like JSON result with `success`, `violations`, `warnings`, and
  parse/evaluation `errors`.
- A small rule catalog plus `successCount` for the rules that were checked
  without producing a violation or warning result.

## Fixture

The browser opens with the fixture from
`fixtures/conforma-shaped/policy.rego`, `input-fail.yaml`, and `data.yaml`.
That fixture is still intentionally small, but it exercises helper modules,
separate rule data, and structured violation/warning objects with `code`, `msg`,
and `metadata`. It also includes OPA `# METADATA` comments so the playground can
surface rule titles and short names in the output.

See `PARITY.md` for the current parity notes and the recommended follow-up
comparison with the real `ec validate input` path.

## What It Does Not Prove

- Full `ec validate input` parity.
- EnterpriseContractPolicy parsing.
- Include/exclude filtering.
- Remote policy fetching.
- OCI image, attestation, Rekor, or Sigstore flows.
- Website integration or production UX.

Those are follow-up decisions after the spike establishes whether a small
browser-local evaluator is practical.
