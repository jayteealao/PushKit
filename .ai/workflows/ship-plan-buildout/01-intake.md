---
schema: sdlc/v1
type: intake
slug: ship-plan-buildout
status: complete
stage-number: 1
created-at: "2026-05-22T21:28:25Z"
updated-at: "2026-05-22T21:28:25Z"
tags:
  - release-engineering
  - ci-cd
  - github-actions
  - nsis
  - git-cliff
  - android
  - go
refs:
  index: 00-index.md
  ship-plan: ../../ship-plan.md
  next: 02-shape.md
next-command: wf-shape
next-invocation: "/wf shape ship-plan-buildout"
---

# Intake

## Restated Request

Build out the CI infrastructure and application-side code required to satisfy the project's ship-plan contract (`.ai/ship-plan.md`). The plan describes a unified-tag, multi-artifact `publish` flow (CLI → PyPI, backend Windows installer → GitHub Releases, Android APK → GitHub Releases), but most of the wiring does not yet exist. This workflow implements it and validates by cutting `v0.1.0-rc.1` end-to-end.

## Intended Outcome

A pushable `v*` tag on `main` reliably produces three artifacts (PyPI wheel, NSIS-wrapped backend installer, debug-signed Android APK) with auto-generated release notes, attaches them to a single GitHub Release, and passes four post-publish checks. Pre-merge CI gates `main` with backend tests, CLI tests, Android build, and Android lint. Conventional Commits are enforced via git hooks.

## Primary User / Actor

- **Repo maintainer (jayteealao)** — needs a predictable, low-friction release flow. Tags a version, walks away, gets a release.
- **Downstream consumers** —
  - CLI users (`pip install pushkit`) — get a versioned wheel.
  - Backend operators on Windows — download an installer that just works.
  - Android testers — sideload an APK from GitHub Releases.

## Known Constraints

- Unified `vX.Y.Z` git tag is the single source of truth for version (per ship-plan Block B).
- Tags must originate from `main`'s history (per ship-plan Block C / playbook `tag-on-wrong-branch`).
- PyPI publishing uses OIDC Trusted Publishing only — no long-lived tokens in CI.
- Android is **debug-signed** for v0.x. Release signing is explicitly out of scope.
- Backend is built statically (`CGO_ENABLED=0`) so the Windows binary is a single `.exe` before NSIS wraps it.
- The NSIS build runs on `windows-latest` (Block C decision, not cross-compiled).
- No backend DB migration tooling exists or is being added.

## Assumptions

- The PyPI Trusted Publisher entry (project `pushkit`, owner `jayteealao`, repo `PushKit`, workflow `release.yml`, environment `pypi`) is already configured. **Confirmed by PO.**
- `windows-latest` GitHub-hosted runner has `makensis` pre-installed (or trivially installable via Chocolatey). To be verified during shape's freshness research.
- `git-cliff` can produce acceptable release notes from the project's existing (mostly conventional) commit history.
- `go-to-wheel` and `pypa/gh-action-pypi-publish@release/v1` remain current. To be verified during shape.
- The Android `versionCode` can be derived deterministically from git (e.g., `git rev-list --count HEAD`) and rewritten into `android/app/build.gradle.kts` at build time without committing the change back.

## Product Owner Questions Asked

See `po-answers.md` for the full transcript. Highlights:
- Branch strategy + appetite + review scope (Batch A).
- Whether this workflow stops at "infra ready" or cuts a real prerelease.
- NSIS script location.
- Commit-lint enforcement mechanism.
- PyPI prep status.
- Stack confirmation.

## Product Owner Answers

- **Branch:** dedicated, `feat/ship-plan-buildout` from `main`.
- **Appetite:** large.
- **Review scope:** slug-wide.
- **First release in-workflow:** yes — `v0.1.0-rc.1` end-to-end is the acceptance gate.
- **NSIS location:** `backend/installer/pushkit.nsi`.
- **Commit lint:** enforced via git hooks (mechanism TBD: lefthook / husky / vendored).
- **PyPI prep:** already done; out of scope here.
- **Stack:** confirmed, plus `android-cli` + `lazylogcat` for the Android surface.

## Unknowns / Open Questions

Carried into shape (also live in `00-index.md > open-questions`):

1. Should commitlint also re-run in CI as a backstop against `--no-verify` bypass, or are git hooks alone sufficient?
2. Which git-hooks runner: lefthook (Go-native, single binary), husky (Node toolchain — adds a node_modules dependency), or a vendored shell `commit-msg` script (zero dependencies, ugly)?
3. How is the Android `versionCode` derived? Candidates: `git rev-list --count HEAD`, tag count, or a counter file checked into the repo.
4. Does pre-merge CI live in a new `.github/workflows/ci.yml`, or extend `release.yml` to handle both `pull_request` and `push: tags`?
5. NSIS script complexity: minimal silent installer with Start Menu shortcut, or full installer with optional Windows-service registration?
6. Where do `SHA256SUMS` come from — generated in CI, included in the release, optionally signed (no signing in scope today)?
7. Will the backend installer bundle anything besides `pushkit-server.exe` (e.g., a sample `.env`, the README excerpt)?
8. README "latest release" badge format — `shields.io/github/v/release/...` vs a self-hosted badge.

## Dependencies / External Factors

- **GitHub Actions** — runners, marketplace actions (`actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-python@v5`, `pypa/gh-action-pypi-publish@release/v1`, plus to-add: `actions/setup-java`, `gradle/actions/setup-gradle`, `softprops/action-gh-release` or `gh` CLI).
- **PyPI** — Trusted Publisher entry, project name `pushkit`. Confirmed prepped.
- **NSIS** — version on `windows-latest`. Needs shape-stage check.
- **git-cliff** — config file format + binary install on `ubuntu-latest`. Needs shape-stage check.
- **commitlint stack** — runner choice has dependency implications (lefthook vs husky vs shell).
- **Android Gradle Plugin 8.2.2 + Kotlin 1.9.22** — pinned versions; no upgrade in scope.

## Risks if Misunderstood

- **Multi-artifact ≠ multi-tag.** All three artifacts must come from one tag's CI run, or version-skew is guaranteed. The release workflow's job graph must enforce this.
- **OIDC misconfig** is the #1 first-release failure mode — PyPI Trusted Publisher entry must match `workflow_filename + environment + owner + repo` exactly.
- **`--no-verify` bypass.** Git hooks alone don't enforce commit-lint reliably; humans turn off hooks. A CI backstop is the only durable gate.
- **Tag scope creep.** A tag pushed from a feature branch (or for the wrong artifact set) triggers a real release; the workflow needs a guard or branch-protection rule.
- **Android `versionCode` collisions.** Once an APK with a given `versionCode` is installed on a device, a lower or equal `versionCode` won't install. Pick a monotonic source and stick with it.
- **Cross-compiling Go for Windows from Linux runner** is normal, but the NSIS step has to run on Windows — the job graph needs an artifact handoff between Linux and Windows runners (or build everything on Windows).
- **Empty changelog.** First release has no prior tag to diff against; `git-cliff` needs a special case (or a `[unreleased]` section).
- **README badge cycle.** A badge that auto-updates from the Releases API is fine; one that requires a commit on each release becomes a chicken-and-egg with branch protection.

## Success Criteria

A tag push of `v0.1.0-rc.1` on `main` produces, with zero manual steps after the push:

1. **Pre-merge CI** is green on the PR that lands this workflow (backend `go vet+test`, CLI `go vet+test`, Android `gradle build`, Android `lint + testDebugUnitTest`, commitlint).
2. **Release workflow** runs on the tag push and completes all seven jobs (`retest`, `build-cli-wheel`, `publish-pypi`, `build-backend-windows-installer`, `build-android-apk`, `generate-changelog`, `create-github-release`).
3. **PyPI:** `pip install --no-cache-dir pushkit==0.1.0rc1 && pushkit --version` returns `0.1.0rc1` (or whatever PyPI normalizes the prerelease to).
4. **GitHub Release `v0.1.0-rc.1`** exists, marked as prerelease, with assets `pushkit-server-setup.exe`, `pushkit-android.apk`, `SHA256SUMS`, plus auto-generated release notes from `git-cliff`.
5. **Installer smoke:** silent install (`pushkit-server-setup.exe /S`) on a Windows runner produces a working `pushkit-server.exe --version` returning `0.1.0-rc.1`.
6. **APK probe:** `aapt dump badging pushkit-android.apk` reports `versionName='0.1.0-rc.1'` and a sane monotonic `versionCode`.
7. **README** shows a "latest release" badge resolving to `v0.1.0-rc.1`.
8. **Commit hygiene:** every commit on `feat/ship-plan-buildout` parses under Conventional Commits (git-cliff produces a non-empty changelog).
9. **Cosmetic fix landed:** `Makefile` and `.github/workflows/release.yml` no longer reference `https://github.com/pushkit/cli`; both point to `https://github.com/jayteealao/PushKit`.

## Out of Scope for Now

- Migrating Android off debug signing (deferred — would unlock a future `mobile-app-store` additional-contract on the ship-plan).
- Backend DB migration tooling (still raw SQLite).
- Changes to `backend/Dockerfile` / `backend/docker-compose.yml` — local-dev assets, not the release surface.
- Go module path renames (`github.com/pushkit/cli`, `github.com/pushkit/backend`).
- Windows code-signing the installer (no certificate; users will see SmartScreen warnings — accepted for v0.x).
- macOS / Linux backend installers (Windows-only per ship-plan Block A).
- Backend container image publishing to GHCR (not in ship-plan).
- Asset signing (SLSA provenance, sigstore) — future hardening.
- Adding observability tooling (Sentry, OpenTelemetry) — outside this workflow.

## Freshness Research

Light intake-stage notes. Full freshness pass happens in shape.

- **Source:** PyPI Trusted Publishing docs (https://docs.pypi.org/trusted-publishers/)
  **Why it matters:** First-release failure mode #1 (per ship-plan playbook). Field names and `environment` semantics need to match exactly.
  **Takeaway:** Already prepped per PO — re-verify the exact `workflow_filename`, `environment`, `repo`, `owner` quartet against the Actions workflow during shape.

- **Source:** `git-cliff` official docs / GitHub releases
  **Why it matters:** Bump-rule and changelog-generation tool referenced in ship-plan Block B. Need a `cliff.toml` and the right CLI flags.
  **Takeaway:** Verify current major version + `--bumped-version` flag availability during shape.

- **Source:** NSIS docs + `windows-latest` runner image manifest
  **Why it matters:** Decides whether to `choco install nsis` or rely on pre-installed `makensis`.
  **Takeaway:** Verify availability + version on the GitHub-hosted runner during shape.

- **Source:** Conventional Commits + lefthook/husky docs
  **Why it matters:** PO chose git-hooks enforcement; mechanism still open. Each runner has different operational cost.
  **Takeaway:** Compare lefthook (single Go binary), husky (Node toolchain), and vendored shell during shape.

## Recommended Next Stage

- **Option A (default):** `/wf shape ship-plan-buildout` — multiple ambiguities to resolve (commitlint runner, Android versionCode source, NSIS scope, pre-merge CI file layout, installer bundle contents). Twenty product-owner-style questions are warranted.
- **Option B:** Skip to `/wf slice ship-plan-buildout` — only viable if shape's open questions are all answered up front; not recommended given the eight unknowns listed above.
- **Option C:** Blocked — not applicable. Intake answers are complete.
