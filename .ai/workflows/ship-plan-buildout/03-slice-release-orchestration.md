---
schema: sdlc/v1
type: slice
slug: ship-plan-buildout
slice-slug: release-orchestration
status: defined
stage-number: 3
created-at: "2026-05-22T22:46:55Z"
updated-at: "2026-05-22T22:46:55Z"
complexity: xl
depends-on:
  - commit-hygiene
  - nsis-installer
  - backend-version
  - android-versioning
tags:
  - github-actions
  - release
  - git-cliff
  - pypi
  - github-release
  - post-publish-checks
refs:
  index: 00-index.md
  slice-index: 03-slice.md
  siblings:
    - 03-slice-commit-hygiene.md
    - 03-slice-nsis-installer.md
    - 03-slice-backend-version.md
    - 03-slice-android-versioning.md
  plan: 04-plan-release-orchestration.md
  implement: 05-implement-release-orchestration.md
---

# Slice: release-orchestration

## Goal

`.github/workflows/release.yml` expands from a single PyPI-publish job into an 8-job graph that, on a `v*` tag push, produces a complete GitHub Release with three artifacts + SHA256SUMS + git-cliff-generated notes, then runs post-publish checks against the live release. This slice is the integrator that ties all four prior slices together and validates the entire ship-plan contract by cutting `v0.1.0-rc.1`.

## Why This Slice Exists

The four prior slices produce the inputs (CI baseline, NSIS script, backend `--version`, Android version-override mechanism). This slice is the orchestrator: tag-guard, retest, cross-compile + NSIS-wrap, APK build with version injection, wheel build + publish, changelog generation, GitHub Release creation, post-publish verification. It also includes the `concurrency:` and `permissions:` blocks, the `cliff.toml` git-cliff config, and the README updates.

`complexity: xl` — this is the largest slice by far. ~200 lines of YAML across 8 jobs, a `cliff.toml`, README sections for `## Releasing` and `## Backend installer`, the shields.io badge, and a non-trivial validation gate (real PyPI publish + real GitHub Release).

## Scope

### In

- `.github/workflows/release.yml` rewritten with 8 jobs in this dependency graph:

  ```
  tag-guard ──> retest ──┬──> build-cli-wheel ──┐
                          ├──> build-backend-windows-installer ──┤
                          ├──> build-android-apk ────────────────┤
                          └──> generate-changelog ───────────────┤
                                                                  ├──> publish-pypi ──> post-publish-checks
                                                                  └──> create-github-release ──┘
  ```

  - `tag-guard`: `git merge-base --is-ancestor $GITHUB_SHA origin/main` — exits non-zero if tag's commit isn't on main.
  - `retest`: re-runs `backend-test`, `cli-test`, `android-build` (the three test jobs from `ci.yml`) against the tagged commit. If anything fails, the release aborts.
  - `build-cli-wheel` (Linux): `make build-wheels VERSION=<tag-without-v>` → uploads wheel artifact.
  - `build-backend-windows-installer` (Linux cross-compile + Windows NSIS-wrap, OR all-Windows):
    - Cross-compile `pushkit-server.exe` on Linux with `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=<tag>"`.
    - Upload exe artifact, download on `windows-2022` runner.
    - `makensis -DVERSION=<tag> backend/installer/pushkit.nsi` → upload `pushkit-server-setup.exe` artifact.
    - (Plan stage: confirm cross-compile is preferred over all-Windows for speed; matrix decision documented.)
  - `build-android-apk` (Linux, full-depth checkout): set `versionCode = git rev-list --count HEAD` and `versionName = <tag-without-v>` via Gradle properties → `./gradlew assembleDebug` → upload APK artifact.
  - `generate-changelog` (Linux, full-depth checkout): `orhun/git-cliff-action@v3 --tag <tag> --unreleased -o CHANGELOG-<tag>.md` → upload changelog artifact.
  - `publish-pypi` (Linux, `environment: pypi`, `permissions: id-token: write`): downloads wheel artifact, runs `pypa/gh-action-pypi-publish@release/v1`. Sealed `PYPI_API_TOKEN` is referenced ONLY in a documented break-glass branch (e.g., a `publish-pypi-fallback` job that's manually triggered, not in the default graph).
  - `create-github-release` (Linux): downloads installer + APK + changelog artifacts, generates `SHA256SUMS` over the binary assets, runs `gh release create <tag> --notes-file CHANGELOG-<tag>.md $PRERELEASE_FLAG pushkit-server-setup.exe pushkit-android.apk SHA256SUMS`. Auto-detects prerelease from `-rc.` substring in tag.
  - `post-publish-checks` (matrix: Linux + Windows): `fresh-resolve` (`pip install --no-cache-dir --pre pushkit==<normalized> && pushkit --version`), `github-release` (`gh api ... releases/tags/<tag>` returns 200 with three assets), `smoke-test` (Windows: `pushkit-server-setup.exe /S && "%ProgramFiles%\PushKit\pushkit-server.exe" --version` matches tag), `apk-probe` (`aapt dump badging` reports correct metadata).
- New `cliff.toml` at repo root: Keep-a-Changelog output template, conventional-commits parsing rules, version regex matching `v[0-9]+\.[0-9]+\.[0-9]+(?:-rc\.[0-9]+)?`.
- `README.md` updates:
  - shields.io release badge at top: `![Latest release](https://img.shields.io/github/v/release/jayteealao/PushKit?include_prereleases=true&sort=semver)`.
  - New `## Releasing` section covering tag conventions, the single push command, expected CI runtime, rollback procedure (`pip yank` + `-rc.N+1`), and the break-glass `PYPI_API_TOKEN` path.
  - New `## Backend installer` section covering the silent install (`/S`) flag, optional service component, SmartScreen warning explanation.
- `concurrency: { group: release-${{ github.ref }}, cancel-in-progress: false }` on the workflow.
- `permissions:` top-level set to `contents: write` (for `gh release create`) and `id-token: write` only on the `publish-pypi` job.
- `tag-protection` rule on the GitHub repo restricting `v*` tag creation to maintainers — configured manually by the maintainer (documented in `## Releasing` section, not enforced by code).
- Validation: push `v0.1.0-rc.1` from `main` after all slices are merged. Watch all 8 jobs complete green. Verify all 8 success criteria from intake (now AC1–AC13 in shape).

### Out (handled by other slices)

- The inputs themselves (NSIS script, backend version flag, Android version-override Gradle properties, commit-hygiene CI baseline) — all four prior slices.

### Explicitly out of scope (per intake + shape)

- Android release signing.
- Backend DB migrations.
- Windows code-signing.
- Go module path renames.
- Container image publishing to GHCR.
- SLSA provenance / sigstore attestations.

## Acceptance Criteria

Inherits the cumulative success criteria of v0.1.0-rc.1 from intake (success-criteria 1–9) and shape (AC1–AC13). Specifically, this slice owns:

- **Given** the workflow is fully merged into `main` and a tag `v0.1.0-rc.1` is pushed from `main`'s HEAD, **when** `release.yml` runs to completion, **then** all 8 jobs succeed and the GitHub Release `v0.1.0-rc.1` exists with assets `pushkit-server-setup.exe`, `pushkit-android.apk`, `SHA256SUMS`, marked as prerelease, with non-empty release notes. *(AC5 — interactive.)*
- **Given** the release workflow completes, **when** `pip install --no-cache-dir --pre pushkit==0.1.0rc1 && pushkit --version` runs in a fresh venv on a clean runner, **then** it returns `pushkit version 0.1.0-rc.1` (or PEP-440-normalized equivalent). *(AC6 — automated, runs as `post-publish-checks.fresh-resolve`.)*
- **Given** the release exists with `pushkit-server-setup.exe`, **when** silent-installed on `windows-latest` and `pushkit-server.exe --version` is run, **then** output is `pushkit-server 0.1.0-rc.1`. *(AC7 — automated via `post-publish-checks.smoke-test`.)*
- **Given** the release exists with `pushkit-android.apk`, **when** `aapt dump badging pushkit-android.apk` runs, **then** output contains `versionName='0.1.0-rc.1'` and a monotonic `versionCode`. *(AC8 — automated via `post-publish-checks.apk-probe`.)*
- **Given** the release exists with `SHA256SUMS`, **when** `sha256sum -c SHA256SUMS` runs after downloading all three assets, **then** every line reports OK. *(AC10 — automated.)*
- **Given** the release exists, **when** GitHub Release notes are read, **then** they contain a section for the tag with at least one categorized commit entry. *(AC9 — human-in-the-loop.)*
- **Given** v0.1.0-rc.1 is published, **when** the README is viewed on github.com 30+ minutes post-publish, **then** the shields.io badge renders `v0.1.0-rc.1`. *(AC12 — human-in-the-loop.)*
- **Given** a tag is pushed with a deliberately broken NSIS file (validation step in this slice's verify), **when** `release.yml` runs, **then** `build-backend-windows-installer` fails red, `publish-pypi` is NOT executed, and `create-github-release` is NOT executed. *(AC13 — interactive verification, throwaway tag.)*
- **Given** a tag is pushed pointing at a commit not on `main`, **when** `release.yml` runs, **then** `tag-guard` exits non-zero and no downstream job runs. *(AC4 — automated.)*

## Dependencies on Other Slices

- **`commit-hygiene`** — `ci.yml` exists, conventional commits enforced. `retest` job composes the same matrix.
- **`nsis-installer`** — `backend/installer/pushkit.nsi` exists and accepts `/DVERSION=`. The `build-backend-windows-installer` job invokes it.
- **`backend-version`** — backend supports `var Version` + `--version` flag. The `post-publish-checks.smoke-test` job depends on it.
- **`android-versioning`** — `android/app/build.gradle.kts` accepts `-PversionNameOverride` and `-PversionCodeOverride`. The `build-android-apk` job uses them.

## Risks

- **First-release PyPI OIDC misconfig.** The #1 first-release failure mode per ship-plan Block F. Mitigation: re-verify the Trusted Publisher quartet (`owner=jayteealao`, `repo=PushKit`, `workflow_filename=release.yml`, `environment=pypi`) against the PyPI dashboard immediately before pushing the validation tag.
- **PyPI publish before downstream failure.** All-or-nothing ordering is enforced by the job graph: `publish-pypi` depends on all four build jobs. Triple-check the `needs:` block in the plan stage.
- **GitHub Release race with PyPI.** If `publish-pypi` succeeds but `create-github-release` fails, PyPI has the wheel but no release page exists. Recovery: manually run `gh release create` against the tag, or push `-rc.2`.
- **Windows runner cold start.** ~2–4 min cold-start adds to the wall-clock budget (shape NFR: ≤ 15 min tag-to-release-visible). Watch for this in the validation tag run.
- **git-cliff first-tag handling.** No prior tag → `git cliff --unreleased --tag <tag>` is required. Without `--unreleased`, git-cliff returns empty for a tag with no predecessor.
- **`gh release create` upload flake.** `gh` occasionally fails on large asset uploads. Plan stage will spec a retry policy (e.g., 3 attempts with exponential backoff).
- **`PYPI_API_TOKEN` rotation forgotten.** Sealed break-glass secret has no rotation policy by default. Document in `## Releasing` README that it should be rotated annually or whenever a maintainer leaves.
- **Concurrency lock.** Two simultaneous tag pushes (rare) collide on the `concurrency: { group: release-${{ github.ref }} }` lock. The second is queued; `cancel-in-progress: false` keeps the first running to completion.
- **Validation tag pollution.** Pushing a real `v0.1.0-rc.1` for validation publishes to real PyPI. If validation reveals a bug, the recovery path is `pip yank` + `-rc.2`, not deletion. Accepted per intake.
- **CHANGELOG.md commit lifecycle.** Per ship-plan Block B, the changelog is NOT committed before release. The release workflow generates it fresh from `git-cliff` and embeds it in the GitHub Release notes; nothing is pushed back to `main`. If the maintainer later wants a committed CHANGELOG.md, that's a separate workflow.
- **Wheel publish vs GH Release race condition.** The current shape says `publish-pypi` depends on all four build jobs, then `create-github-release` depends on `publish-pypi`. So PyPI happens before GH Release. If `create-github-release` fails, PyPI has the wheel but no release page exists yet. Mitigation: `create-github-release` is short and reliable; manual retry trivial.
