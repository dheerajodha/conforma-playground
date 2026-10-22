# Conforma Playground

Browser-local Conforma policy playground PoC using Go, WebAssembly, and OPA.

This spike proves a narrow policy authoring loop: inline Rego plus inline
JSON/YAML input evaluated by OPA from Go compiled to WebAssembly.

The UI uses the Conforma upstream color palette and logo treatment.

It intentionally does not run the full Conforma CLI command tree. The current
goal is to learn whether the browser can support the core `ec validate input`
authoring experience before pulling in image validation, OCI fetching, Sigstore,
Rekor, or remote policy source loading.

## Repository Split

The browser UI, examples, GitHub Pages workflow, and WASM bridge live in this
repository. The validation engine lives in `github.com/conforma/cli/pkg/playground`
so Conforma owns the policy evaluation behavior while this repository owns the
playground experience.

Until `pkg/playground` is available from the Conforma CLI default branch,
`go.mod` temporarily replaces `github.com/conforma/cli` with the spike branch
from `github.com/dheerajodha/cli`. Remove that replacement once the CLI PR
merges.

## Build and Run

From the repository root:

```bash
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
GOOS=js GOARCH=wasm go build -o web/playground.wasm ./cmd/wasm
python3 -m http.server 8090 -d web
```

Then open:

```text
http://localhost:8090
```

`playground.wasm` and `wasm_exec.js` are generated local artifacts and are
ignored by Git.

## GitHub Pages

The `Deploy GitHub Pages` workflow builds the WASM artifact, copies Go's
`wasm_exec.js` runtime helper, and publishes the static bundle from `dist/`.

## Repository Layout

- `cmd/wasm/` exposes `github.com/conforma/cli/pkg/playground` to JavaScript when compiled to WASM.
- `web/` contains the static playground UI and visual assets.
- `fixtures/` contains example policies, data, and inputs for parity checks.

## What It Supports

- A single pasted Rego module.
- Multiple pasted Rego modules separated by `# file: <name>.rego` markers.
- Pasted JSON or YAML input.
- Optional pasted JSON or YAML data available under `data`.
- `deny`, `violation`, and `warn` rules in the module package.
- OPA `# METADATA` comments for rule title and `custom.short_name`.
- A Conforma-like JSON result with `success`, `success-count`, `violations`,
  `warnings`, `successes`, and parse/evaluation `errors`.
- Rule metadata attached to each result, including code, title, description,
  and solution when provided by `# METADATA`.

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
