---
schema: sdlc/v1
type: plan
slug: ship-plan-buildout
slice-slug: backend-version
status: complete
stage-number: 4
created-at: "2026-05-26T11:13:29Z"
updated-at: "2026-05-26T11:13:29Z"
metric-files-to-touch: 4
metric-step-count: 7
has-blockers: false
revision-count: 0
stack-source: confirmed
tags:
  - go
  - backend
  - cosmetic
refs:
  index: 00-index.md
  plan-index: 04-plan.md
  slice-def: 03-slice-backend-version.md
  siblings:
    - 04-plan-commit-hygiene.md
    - 04-plan-nsis-installer.md
    - 04-plan-android-versioning.md   # not yet written
    - 04-plan-release-orchestration.md # not yet written
  implement: 05-implement-backend-version.md
next-command: wf-implement
next-invocation: "/wf implement ship-plan-buildout backend-version"
---

# Plan: backend-version

## Current State

`backend/cmd/server/main.go` (88 lines) has zero flag/arg parsing — no stdlib `flag`, no cobra, no `os.Args` access. `main()` proceeds directly:

1. Line 19: JSON slog logger initialized.
2. Line 22: `config.Load()` — reads 8 required/optional env vars; exits 1 on error.
3. Lines 28–43: DB open, schema creation, S3 client init — each exits 1 on error.
4. Lines 46–73: chi router built; `http.ListenAndServe` started.

Because `config.Load()` panics on missing env vars, the `--version` check must fire **before line 22** — running `./pushkit-server --version` in a plain CI environment without the full env-var set must not error out first.

The CLI already has the identical pattern locked:
- `cli/main.go:11-12`: `var Version = "dev"` + ldflag comment.
- `cli/cmd/root.go:52-53`: cobra's `rootCmd.Version = Version` — not applicable to the backend (backend has no cobra dependency and CLAUDE.md says avoid new deps).

URL bugs to fix:
- `Makefile:13`: `--url "https://github.com/pushkit/cli"` → `"https://github.com/jayteealao/PushKit"`
- `.github/workflows/release.yml:44`: same string, same fix.

No existing tests for `backend/cmd/server/`. The five test files that exist are all under `backend/internal/` and use stdlib testing + `t.Fatal`/`t.Errorf` assertions — zero testify, zero test helpers beyond `setupTestDB` in `db_test.go`.

## Reuse Opportunities

- `cli/main.go:11-12` → `var Version = "dev"` pattern: **reuse as-is** (copy the declaration + ldflag comment verbatim).
- `backend/internal/*` test patterns: **reuse style** — stdlib `testing`, `t.Fatal`, `t.Errorf`, no third-party libraries. New tests for `cmd/server` follow the same idiom.
- No reuse candidates for flag handling (none exist in backend). Stdlib `flag` package sufficient.

## Likely Files / Areas to Touch

| File | Why |
|---|---|
| `backend/cmd/server/main.go` | Add `var Version`, import `flag`/`fmt`, add flag check at top of `main()` |
| `backend/cmd/server/main_test.go` | New — unit test for `printVersion`, subprocess test for the real binary |
| `Makefile` | Line 13: URL cosmetic fix |
| `.github/workflows/release.yml` | Line 44: URL cosmetic fix |

## Proposed Change Strategy

Single-track, mechanical:

1. Add `var Version = "dev"` + ldflag comment at package scope.
2. Extract `printVersion(w io.Writer, v string)` as a package-level helper (same file). This separates the I/O from the exit, making the output testable without `os.Exit`.
3. At the top of `main()`, before any other initialization: `flag.Bool("version", ...)` + `flag.Parse()` + if check calling `printVersion` then `os.Exit(0)`.
4. Create `backend/cmd/server/main_test.go` with two tests: unit test for `printVersion`, subprocess test via `exec.Command` for the full binary flow (builds the binary in `TestMain`).
5. Fix the two URL cosmetic bugs (one-line edits each).

**Decision: no `-v` short alias.** PO confirmed `--version` only. Reason: `-v` conventionally means verbose in Go tooling; accepting the alias would create a future conflict if verbose logging is ever added to the server binary.

**Decision: unit + subprocess test.** PO wants both a unit test for `printVersion` and an `exec.Command` subprocess test that verifies the real binary builds and reports the expected output.

## Step-by-Step Plan

### Step 1 — Add `var Version` and `printVersion` helper to `backend/cmd/server/main.go`

At package scope, before `func main()`:

```go
// Version is set at build time via -ldflags "-X main.Version=<tag>".
var Version = "dev"

// printVersion writes "pushkit-server <v>" to w.
func printVersion(w io.Writer, v string) {
    fmt.Fprintf(w, "pushkit-server %s\n", v)
}
```

Add `"fmt"` and `"io"` to the import block (stdlib only, no new module dependencies).

### Step 2 — Add `--version` flag check at the top of `main()`

Replace the current opening of `main()` (line 17, before logger init):

```go
func main() {
    showVersion := flag.Bool("version", false, "print version and exit")
    flag.Parse()
    if *showVersion {
        printVersion(os.Stdout, Version)
        os.Exit(0)
    }

    // existing: logger, config.Load(), db, s3, router ...
```

Add `"flag"` to the import block.

**Why before logger init:** `config.Load()` at line 22 exits 1 if required env vars are missing. A bare `--version` invocation (in CI smoke test, local dev, README instructions) must not require the full runtime environment.

### Step 3 — Create `backend/cmd/server/main_test.go`

```go
package main

import (
    "bytes"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
)

// versionBinary holds the path to the built test binary (set by TestMain).
var versionBinary string

func TestMain(m *testing.M) {
    // Build the binary once into a temp dir; clean up after all tests.
    dir, err := os.MkdirTemp("", "pushkit-server-test-*")
    if err != nil {
        panic(err)
    }
    defer os.RemoveAll(dir)

    bin := filepath.Join(dir, "pushkit-server")
    if runtime.GOOS == "windows" {
        bin += ".exe"
    }
    cmd := exec.Command("go", "build", "-o", bin, ".")
    cmd.Dir = "." // run from backend/cmd/server/
    if out, err := cmd.CombinedOutput(); err != nil {
        panic("build failed: " + string(out))
    }
    versionBinary = bin
    os.Exit(m.Run())
}

func TestPrintVersion(t *testing.T) {
    var buf bytes.Buffer
    printVersion(&buf, "1.2.3")
    got := buf.String()
    want := "pushkit-server 1.2.3\n"
    if got != want {
        t.Errorf("printVersion: got %q, want %q", got, want)
    }
}

func TestPrintVersionDefault(t *testing.T) {
    var buf bytes.Buffer
    printVersion(&buf, "dev")
    got := buf.String()
    if !strings.HasPrefix(got, "pushkit-server ") {
        t.Errorf("printVersion default: unexpected output %q", got)
    }
}

func TestVersionFlag_Binary(t *testing.T) {
    out, err := exec.Command(versionBinary, "--version").Output()
    if err != nil {
        t.Fatalf("--version flag failed: %v", err)
    }
    got := strings.TrimSpace(string(out))
    want := "pushkit-server dev"
    if got != want {
        t.Errorf("--version output: got %q, want %q", got, want)
    }
}

func TestVersionFlag_LdflagsInjected(t *testing.T) {
    // Build with an injected version to verify ldflags injection works end-to-end.
    dir, _ := os.MkdirTemp("", "pushkit-server-ldflags-*")
    defer os.RemoveAll(dir)

    bin := filepath.Join(dir, "pushkit-server")
    if runtime.GOOS == "windows" {
        bin += ".exe"
    }
    injected := "v9.8.7-test"
    cmd := exec.Command("go", "build", "-ldflags", "-X main.Version="+injected, "-o", bin, ".")
    cmd.Dir = "."
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("build with ldflags failed: %s", out)
    }

    out, err := exec.Command(bin, "--version").Output()
    if err != nil {
        t.Fatalf("--version on ldflags binary failed: %v", err)
    }
    got := strings.TrimSpace(string(out))
    want := "pushkit-server " + injected
    if got != want {
        t.Errorf("ldflags injection: got %q, want %q", got, want)
    }
}
```

**Notes:**
- `package main` (not `package main_test`) so `printVersion` is accessible for the unit test without exporting it.
- `TestMain` builds the binary once to avoid repeated compilations. Each subprocess test reuses `versionBinary`.
- `TestVersionFlag_LdflagsInjected` is the key AC11 coverage: it proves the linker flag injection works, not just the flag handler.
- Tests do not call `main()` directly — `os.Exit` in `main()` would abort the test process. `printVersion` is the testable surface.
- The `build` command in `TestMain` runs with `cmd.Dir = "."`, which resolves to `backend/cmd/server/` when `go test ./cmd/server/...` is run from `backend/`. This is consistent with the project's test invocation pattern.

### Step 4 — Fix `Makefile:13`

Change:
```
--url "https://github.com/pushkit/cli" \
```
To:
```
--url "https://github.com/jayteealao/PushKit" \
```

One-line edit. No other Makefile changes.

### Step 5 — Fix `.github/workflows/release.yml:44`

Change:
```
--url "https://github.com/pushkit/cli" \
```
To:
```
--url "https://github.com/jayteealao/PushKit" \
```

One-line edit. Same string, same fix.

### Step 6 — Local validation before pushing

From `backend/`:
```bash
go build -o pushkit-server ./cmd/server
./pushkit-server --version         # → "pushkit-server dev"
go build -ldflags "-X main.Version=v0.1.0-rc.1" -o pushkit-server ./cmd/server
./pushkit-server --version         # → "pushkit-server v0.1.0-rc.1"
go test ./cmd/server/...           # → all 4 tests pass
go vet ./...                       # → no issues
```

Check that running without `--version` still starts normally (or exits cleanly on missing env vars — the important thing is it does NOT print version and exit 0 without `--version`).

Also verify from repo root:
```bash
make build-wheels VERSION=0.1.0-test-1
# confirm no error about invalid URL; inspect the .whl metadata
unzip -p dist/pushkit-*.whl "*/METADATA" | grep Home-page
# → should show https://github.com/jayteealao/PushKit
```

### Step 7 — Commit shape

Conventional commit: `feat(backend): add --version flag and fix cosmetic URLs`

This lands on `feat/ship-plan-buildout` after `nsis-installer`.

## Test / Verification Plan

### Automated checks

- **lint/typecheck:** `go vet ./...` from `backend/`. Must produce no diagnostics. The new `flag`/`fmt`/`io` imports are all stdlib — no new module additions.
- **unit tests:**
  - `TestPrintVersion` — verifies `printVersion` writes `"pushkit-server 1.2.3\n"` to an `io.Writer`.
  - `TestPrintVersionDefault` — verifies the `"dev"` default prefix.
  - `TestVersionFlag_Binary` — subprocess: builds the binary, runs `--version`, checks `"pushkit-server dev"`.
  - `TestVersionFlag_LdflagsInjected` — subprocess: builds with `-X main.Version=v9.8.7-test`, runs `--version`, checks injected value. **This is AC11.**
- **integration tests:** Not applicable.

### Interactive verification (human-in-the-loop)

**AC11 — `--version` works after ldflags build**

- **What to verify:** Build with injected version, run `--version`, observe output matches the tag.
- **Platform & tool:** Developer machine + Go toolchain. Confirmed stack: `platforms: [service, cli, android]`, `testing: [go-testing]`. Go stdlib build toolchain covers this.
- **Steps:**
  1. From `backend/`: `go build -ldflags "-X main.Version=v0.1.0-rc.1" -o pushkit-server.exe ./cmd/server`
  2. `./pushkit-server.exe --version` → should print `pushkit-server v0.1.0-rc.1`
  3. `exit $?` → should be 0.
- **Evidence capture:** Terminal output. Paste into `06-verify-backend-version.md`.
- **Pass criteria:** Output is exactly `pushkit-server v0.1.0-rc.1`, exit code 0.

No tooling outside the confirmed stack is required. All ACs are covered by automated tests (`TestVersionFlag_LdflagsInjected`) + the manual build check above.

## Risks / Watchouts

- **`flag.Parse()` before logger init.** Flag parsing happens before the structured logger is set up. If `flag.Parse()` encounters an unknown flag it calls `flag.Usage()` (which prints to stderr) and `os.Exit(2)`. This is expected behavior and is not a risk — it's cleaner than a full server startup for a bad flag. No mitigation needed.
- **`flag` package conflicts with future cobra adoption.** If cobra is ever added to the backend, the stdlib `flag` registration must be removed. `pflag` (cobra's flag library) is incompatible with stdlib `flag` without explicit bridging. Low risk for v0.x; documented here as a known constraint.
- **Subprocess test build time.** `TestMain` runs `go build` once per `go test ./cmd/server/...` invocation, adding ~3–5 seconds compilation overhead. Acceptable for this xs slice; if it becomes noisy in the CI matrix, the build can be gate-flagged with `testing.Short()`.
- **Windows binary extension.** `TestMain` and `TestVersionFlag_LdflagsInjected` both handle `runtime.GOOS == "windows"` by appending `.exe`. Must be verified on the maintainer's Windows machine (where CI will also run the smoke test for AC7).
- **ldflag injection format.** The release-orchestration slice decides exactly how the tag is passed to `-X main.Version`. If `release-orchestration` strips the `v` prefix (injecting `0.1.0-rc.1` not `v0.1.0-rc.1`), the output will differ from `v0.1.0-rc.1`. This slice is agnostic — `printVersion` prints whatever is in `Version`. The AC7 vs. AC11 minor inconsistency in the shape (AC7 says `0.1.0-rc.1`, AC11 says `v0.1.0-rc.1`) is resolved by `release-orchestration`'s ldflags injection choice. Not a blocker for this slice.

## Dependencies on Other Slices

- **Inbound:** `commit-hygiene` must land first so this slice's commits are Conventional Commits. The branch already has `commit-hygiene` implemented.
- **Outbound:** `release-orchestration` depends on `pushkit-server --version` working. The smoke test in the post-publish-checks job asserts `pushkit-server.exe --version` returns the tag. This slice supplies that capability.
- **No file overlap with `nsis-installer`:** The NSIS script (`backend/installer/pushkit.nsi`) is indifferent to whether the wrapped binary has `--version`. No conflict.

## Assumptions

- Go toolchain (1.24) is installed locally for the interactive build verification.
- The `make build-wheels` command completes without error after the Makefile URL fix (the fix is cosmetic; the wheel-build logic is unchanged).
- `go test ./cmd/server/...` is run from `backend/` (consistent with existing README instructions). The subprocess test's `cmd.Dir = "."` resolves correctly to the `cmd/server/` directory.
- The `backend/` module's go.sum does not need updating — no new dependencies added.

## Blockers

None.

## Freshness Research

### Source: pkg.go.dev/flag (Go stdlib)

**Relevance:** Primary library for the `--version` flag implementation.
**Takeaway:** `flag.Bool("version", false, "...")` registers `-version` / `--version` (both one- and two-dash accepted). `flag.Parse()` must be called before reading any `flag.Bool` value. No deprecations; stable since Go 1.0.

### Source: Go community / DigitalOcean — ldflags injection patterns

**Relevance:** Confirms `-ldflags "-X main.Version=<value>"` works for `var Version = "dev"` (not for `const`). Silent behavior if the variable name is misspelled (no error, value stays at default). The plan mitigates this via `TestVersionFlag_LdflagsInjected` — a test that would catch a misspelled ldflag path.
**Takeaway:** Use `main.Version` (not the module path). `var` only (never `const`). Function-initialized vars are NOT injectable.

### Source: rednafi.com/go + abhinavg.net/2022 — subprocess testing patterns

**Relevance:** Chosen test approach (exec.Command in TestMain).
**Takeaway:** `TestMain` to build the binary once, then exec in individual tests, is idiomatic when the test target calls `os.Exit`. The "helper process" pattern (single test binary re-execed) is equally valid but more complex; rejected because it adds `os.Getenv` production-path guards to the binary itself.

### Source: Go community / CLI guidelines — `-v` convention

**Relevance:** PO decision to drop `-v` alias.
**Takeaway:** `-v` in Go tooling universally means verbose (`go test -v`, `go vet -v`). Using it as a `--version` alias would be a footgun if verbose output is ever added to the server. PO confirmed `--version` only.

## Revision History

*(none — first revision)*

## Recommended Next Stage

- **Option A (default):** `/wf implement ship-plan-buildout backend-version` — plan is execution-ready; all decisions locked; no blockers. Consider running `/compact` first to clear planning context before implementation.
- **Option B:** `/wf plan ship-plan-buildout android-versioning` — plan the next slice in implementation order before implementing this one.
