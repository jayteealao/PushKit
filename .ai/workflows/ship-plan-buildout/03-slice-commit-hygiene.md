---
schema: sdlc/v1
type: slice
slug: ship-plan-buildout
slice-slug: commit-hygiene
status: defined
stage-number: 3
created-at: "2026-05-22T22:46:55Z"
updated-at: "2026-05-22T22:46:55Z"
complexity: s
depends-on: []
tags:
  - lefthook
  - commitlint
  - github-actions
  - ci
refs:
  index: 00-index.md
  slice-index: 03-slice.md
  siblings:
    - 03-slice-nsis-installer.md
    - 03-slice-backend-version.md
    - 03-slice-android-versioning.md
    - 03-slice-release-orchestration.md
  plan: 04-plan-commit-hygiene.md
  implement: 05-implement-commit-hygiene.md
---

# Slice: commit-hygiene

## Goal

Conventional Commits are enforced both locally (via a commit-msg hook) and in CI (as a backstop against `--no-verify` bypass), and a lightweight pre-merge CI workflow gates every PR with backend tests, CLI tests, Android assembleDebug + lint, and the commitlint backstop.

## Why This Slice Exists

This slice is the foundation. Every subsequent commit in the workflow needs to parse as Conventional Commits so `git-cliff` can generate a changelog in the final slice. Landing this first means commit-hygiene enforcement starts working on the second-and-onward commits of this same workflow. It also unblocks PR-level confidence — without `ci.yml`, every later slice merges blind.

This is `complexity: s` because the moving parts are external (lefthook binary, commitlint package, an off-the-shelf GitHub Action). The novel surface is just `lefthook.yml`, `commitlint.config.cjs`, and `ci.yml`.

## Scope

### In

- New `lefthook.yml` at repo root configuring the `commit-msg` hook to delegate to `commitlint`.
- New `commitlint.config.cjs` extending `@commitlint/config-conventional`.
- New `package.json` at repo root with `devDependencies: { @commitlint/cli, @commitlint/config-conventional }` (lockfile included). This is the smallest Node toolchain footprint accepted in Round 3.
- New `.github/workflows/ci.yml` with four parallel jobs:
  - `backend-test` — `go vet ./...` + `go test ./...` in `backend/`.
  - `cli-test` — `go vet ./...` + `go test ./...` in `cli/`.
  - `android-build` — `actions/setup-java@v4` Temurin 17 + `gradle/actions/setup-gradle@v4` + `./gradlew assembleDebug lint testDebugUnitTest`.
  - `commitlint-backstop` — `wagoid/commitlint-github-action@v6` validates every PR commit.
- README addition documenting the `lefthook install` one-time setup command for new clones.

### Out (handled by other slices)

- Backend `--version` flag — `backend-version` slice.
- NSIS installer authoring — `nsis-installer` slice.
- Android `versionCode` rewrite — `android-versioning` slice.
- `release.yml` expansion — `release-orchestration` slice.

## Acceptance Criteria

- **Given** a clean clone with `lefthook install` run, **when** the developer attempts `git commit -m "hack stuff"`, **then** the commit is rejected with a clear error citing the conventional-commits spec. *(AC3 from shape — interactive verification on dev machine.)*
- **Given** a PR is opened against `main` containing a commit with a non-conventional message, **when** GitHub Actions runs `ci.yml`, **then** the `commitlint-backstop` job fails red. *(AC2 from shape — automated.)*
- **Given** a PR is opened against `main` containing a syntax error in `backend/`, **when** GitHub Actions runs `ci.yml`, **then** the `backend-test` job fails red. *(AC1 from shape — automated.)*
- **Given** a normal PR with conventional commits and clean code, **when** `ci.yml` runs, **then** all four jobs pass within ≤ 8 minutes p95 wall time.

## Dependencies on Other Slices

None. This slice is the root of the dependency graph.

## Risks

- **Node toolchain footprint.** `@commitlint/cli` pulls a small node_modules tree. Acceptable per Round 3 (lefthook chosen knowing this trade-off) but adds `package.json` + `package-lock.json` to a Go/Kotlin repo. Document the rationale in README.
- **Lefthook bypass.** A developer can `git commit --no-verify` and bypass the local hook. CI backstop is the durable defense.
- **Gradle cold-cache pre-merge time.** First-run `assembleDebug + lint + testDebugUnitTest` can exceed 8 minutes on a cold runner. Mitigate with `actions/cache` keyed on `~/.gradle/caches` + `~/.gradle/wrapper` + the `gradle/wrapper/gradle-wrapper.properties` hash.
- **`wagoid/commitlint-github-action@v6` versions.** Pinned to `v6.2.1` (current). Re-pin during plan stage if a v7 lands before implementation.
- **First commit ambiguity.** The first commit on the `feat/ship-plan-buildout` branch should itself be conventional (e.g., `feat(ci): add commit-msg hook and pre-merge workflow`).
