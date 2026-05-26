---
schema: sdlc/v1
type: implement
slug: ship-plan-buildout
slice-slug: backend-version
status: complete
stage-number: 5
created-at: "2026-05-26T11:27:12Z"
updated-at: "2026-05-26T11:27:12Z"
metric-files-changed: 4
metric-lines-added: 93
metric-lines-removed: 0
metric-deviations-from-plan: 0
metric-review-fixes-applied: 0
commit-sha: ""
tags:
  - go
  - backend
  - cosmetic
refs:
  index: 00-index.md
  implement-index: 05-implement.md
  slice-def: 03-slice-backend-version.md
  plan: 04-plan-backend-version.md
  siblings:
    - 05-implement-commit-hygiene.md
    - 05-implement-nsis-installer.md
  verify: 06-verify-backend-version.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout backend-version"
---

# Implement: backend-version

## Summary of Changes

Added `--version` flag support to `backend/cmd/server/main.go` and four test cases covering the flag handler and ldflags injection. Fixed the cosmetic URL bug (`pushkit/cli` → `jayteealao/PushKit`) in both `Makefile` and `.github/workflows/release.yml`. All changes are mechanical with zero new module dependencies.

## Files Changed

- `backend/cmd/server/main.go` — added `var Version = "dev"` (with ldflag comment), `printVersion(w io.Writer, v string)` helper, and flag check at the top of `main()` before logger/config init. Added `flag`, `fmt`, `io` to imports (stdlib only).
- `backend/cmd/server/main_test.go` — new file; four tests: `TestPrintVersion`, `TestPrintVersionDefault`, `TestVersionFlag_Binary` (subprocess, uses binary built in `TestMain`), `TestVersionFlag_LdflagsInjected` (subprocess, builds with injected version — AC11 coverage).
- `Makefile` — line 13: `--url "https://github.com/pushkit/cli"` → `"https://github.com/jayteealao/PushKit"`.
- `.github/workflows/release.yml` — line 44: same URL fix.

## Shared Files (also touched by sibling slices)

- None. No overlap with `commit-hygiene` or `nsis-installer` files.

## Notes on Design Choices

- **Flag before logger/config**: `flag.Parse()` fires before `config.Load()` so `./pushkit-server --version` works without any env vars set. This matches the plan rationale.
- **No `-v` alias**: Per plan decision, `-v` is reserved for potential future verbose flag. `--version` only.
- **Stdlib `flag` only**: No cobra dependency added to the backend. One flag does not justify the dependency weight.
- **`package main` test file**: `printVersion` is unexported; the test file uses `package main` so it can call it directly without exporting.
- **`TestMain` builds once**: All subprocess tests share the binary built in `TestMain`; only `TestVersionFlag_LdflagsInjected` rebuilds with a custom ldflags. This keeps test suite overhead to ~1 extra build.

## Deviations from Plan

None. All seven plan steps executed exactly as specified.

## Anything Deferred

- Wiring `-ldflags -X main.Version` into the release pipeline — deferred to `release-orchestration` slice per plan scope.

## Known Risks / Caveats

- **`flag` + future cobra**: If cobra is ever added to the backend, the stdlib `flag` registration must be removed (pflag and stdlib flag are incompatible without bridging). Documented in the plan.
- **Subprocess test build time**: `TestMain` adds ~3–5s compilation overhead per `go test ./cmd/server/...` run. Acceptable for xs slice.
- **Windows `.exe` extension**: Both `TestMain` and `TestVersionFlag_LdflagsInjected` append `.exe` on `GOOS=windows`. Verified on developer machine.

## Freshness Research

No freshness research required — this slice uses only Go stdlib (`flag`, `fmt`, `io`, `os`). No external APIs or third-party libraries involved.

## Recommended Next Stage

- **Option A (default):** `/wf verify ship-plan-buildout backend-version` — run `go vet`, `go test ./cmd/server/...`, and the interactive build verification (AC11). `go vet` already passes clean (confirmed during implement).
- **Option B:** `/wf review ship-plan-buildout` — skip verify and go straight to slug-wide review if the maintainer is satisfied with the `go vet` result.
