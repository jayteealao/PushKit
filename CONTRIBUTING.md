# Contributing to PushKit

Thanks for contributing! PushKit is a monorepo with three components — a Go backend, a Go CLI, and a Kotlin/Compose Android app. This guide gets you set up and explains the checks your change must pass.

## Prerequisites

- **Node.js ≥ 22.12** — for the commit-message tooling.
- **Go 1.25** — declared in the modules' `go.mod` and used by CI.
- **Java 17 + Android SDK** — only if you work on the Android app.

## One-time setup

From the repository root:

```bash
npm install
```

This installs the dev tooling and runs the `prepare` script, which executes `lefthook install` to wire a `commit-msg` git hook. The hook runs `commitlint` on every commit so non-conforming messages are caught before they land.

## Commit messages

PushKit uses [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): description`.

**Scopes** (enforced as a warning by commitlint):

`backend`, `cli`, `android`, `ci`, `docs`, `deps`, `installer`, `release`

Examples:

```
feat(cli): add --sha256 flag to upload
fix(backend): handle missing Content-Type header
docs(android): document the first-run credential flow
ci(release): pin go-test action to a SHA
```

The local hook lints each commit. `git commit --no-verify` bypasses it locally, but the `commitlint-backstop` CI job re-lints every commit on a PR, so a bad message will still fail there.

## Running the checks locally

Run the same commands CI runs for whichever component you touched.

**Backend** (`backend/`):

```bash
cd backend
go vet ./...
go test ./...
```

**CLI** (`cli/`):

```bash
cd cli
go vet ./...
go test ./...
```

**Android** (`android/`):

```bash
cd android
./gradlew assembleDebug lint testDebugUnitTest
```

**Vulnerability scan** (run in both `backend/` and `cli/`):

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

Add or update tests for any behavior change — see the testing notes in [AGENT.md](AGENT.md).

## CI gates

Pull requests to `main` must pass these jobs (`.github/workflows/ci.yml`):

| Job | What it checks |
|-----|----------------|
| `backend-test` | `go vet` + `go test` for the backend |
| `cli-test` | `go vet` + `go test` for the CLI |
| `android-build` | `assembleDebug`, `lint`, `testDebugUnitTest` |
| `vuln-scan` | `govulncheck` over backend and CLI |
| `commitlint-backstop` | Conventional Commits on every commit in the PR |

CI is the source of truth — don't merge on red.

## Releasing

Releases are handled by maintainers via a tag-driven pipeline; see [RELEASING.md](RELEASING.md). Contributors don't tag releases.

## House rules

Project-wide engineering conventions (single source of truth, explicit errors, safe-git defaults, dependency policy) live in [AGENT.md](AGENT.md) / [CLAUDE.md](CLAUDE.md). Skim them before a larger change.
