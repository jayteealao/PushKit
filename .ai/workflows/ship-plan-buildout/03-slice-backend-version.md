---
schema: sdlc/v1
type: slice
slug: ship-plan-buildout
slice-slug: backend-version
status: defined
stage-number: 3
created-at: "2026-05-22T22:46:55Z"
updated-at: "2026-05-22T22:46:55Z"
complexity: xs
depends-on:
  - commit-hygiene
tags:
  - go
  - backend
  - cosmetic
refs:
  index: 00-index.md
  slice-index: 03-slice.md
  siblings:
    - 03-slice-commit-hygiene.md
    - 03-slice-nsis-installer.md
    - 03-slice-android-versioning.md
    - 03-slice-release-orchestration.md
  plan: 04-plan-backend-version.md
  implement: 05-implement-backend-version.md
---

# Slice: backend-version

## Goal

`backend/cmd/server/main.go` exposes a `Version` package-level variable and a `--version`/`-v` flag that prints `pushkit-server <version>` and exits 0 before any DB/S3 initialization. CI can inject the version via `-ldflags "-X main.Version=<tag>"`. The cosmetic URL bug in `Makefile` and `.github/workflows/release.yml` (`pushkit/cli` → `jayteealao/PushKit`) is also fixed.

## Why This Slice Exists

The `release-orchestration` slice depends on `pushkit-server.exe --version` working — the post-publish smoke test asserts it returns the tag (AC7). The CLI already has this pattern (`cli/main.go:11`); the backend doesn't, so it needs one line of plumbing plus a flag handler. The URL fix is purely cosmetic but ships together because it's the same "small mechanical hygiene work" theme and lives in the same files touched by other release-engineering slices, avoiding merge churn later.

`complexity: xs` — adding a Version var, a flag handler, a unit test, and two find-replace URL edits. Mechanical, low-risk.

## Scope

### In

- `backend/cmd/server/main.go`:
  - Add `var Version = "dev"` at package scope.
  - Add `--version` and `-v` flag handling at the top of `main()` (before DB/S3 init). Prints `pushkit-server <Version>` to stdout, exits 0.
- A unit test that exercises the version output (likely in `backend/cmd/server/main_test.go` or pulled into an internal test helper — decided in plan stage).
- `Makefile`: change `--url "https://github.com/pushkit/cli"` → `--url "https://github.com/jayteealao/PushKit"`.
- `.github/workflows/release.yml`: grep for `pushkit/cli` occurrences and fix to `jayteealao/PushKit`. (Per shape research, the current release.yml does not actually reference that URL but the grep is cheap insurance.)
- Verify wheel metadata after rebuild: `pip show pushkit | grep Home-page` (or equivalent) shows the corrected URL.

### Out (handled by other slices)

- Wiring `-ldflags -X main.Version` injection into the release pipeline — `release-orchestration` slice.
- Backend Go module path rename — out of scope per intake/shape clarification.

## Acceptance Criteria

- **Given** the backend is built locally via `go build -ldflags "-X main.Version=v0.1.0-rc.1" -o pushkit-server.exe ./cmd/server` from `backend/`, **when** the binary is run with `--version`, **then** it prints `pushkit-server v0.1.0-rc.1` to stdout and exits 0. *(AC11 from shape — automated.)*
- **Given** the backend is built without `-ldflags` (default `go build`), **when** the binary is run with `--version`, **then** it prints `pushkit-server dev` and exits 0.
- **Given** the backend is built and run WITHOUT `--version`, **when** the program starts, **then** the version flag handler does NOT interfere with normal server startup (DB/S3 init proceeds as before).
- **Given** `make build-wheels VERSION=0.1.0-test-1` is run locally, **when** the resulting wheel is inspected (`unzip -p dist/pushkit-*.whl "*/METADATA" | grep Home-page`), **then** the URL is `https://github.com/jayteealao/PushKit`. *(AC of success-criterion 9 from intake.)*
- **Given** a backend unit test runs `go test ./...`, **when** the test exercising the version handler runs, **then** it passes and the flag is verified end-to-end (using `flag` package's `os.Args` swap or `exec.Command` against the test binary).

## Dependencies on Other Slices

- **`commit-hygiene`** — this slice's commits must already be conventional. Lands second; first commit on this slice will be e.g. `feat(backend): add --version flag`.

## Risks

- **Flag-handling library choice.** The CLI uses cobra; the backend currently uses stdlib `flag` (or none — to be confirmed in plan stage's read of `main.go`). The plan stage will decide whether to introduce cobra to the backend or stay with stdlib `flag`. Adding cobra is gratuitous for one flag; stdlib `flag` likely suffices.
- **Test for an os.Exit-on-flag handler.** Testing a flag handler that calls `os.Exit(0)` requires either subprocess testing or refactoring the handler to return an int and letting `main()` exit. Plan stage will pick the cleaner approach.
- **Default version string.** `"dev"` vs `"0.0.0-dev"` vs `"unknown"`. Mirror the CLI choice (`"dev"` at `cli/main.go:11`).
- **URL fix changes `dist/`.** The existing `dist/pushkit-0.1.0-py3-none-win_amd64.whl` was built with the wrong URL. After fix, regenerate locally to verify. Not blocking — `dist/` is gitignored / not on `main`.
