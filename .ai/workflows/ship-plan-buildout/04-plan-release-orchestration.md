---
schema: sdlc/v1
type: plan
slug: ship-plan-buildout
slice-slug: release-orchestration
status: complete
stage-number: 4
created-at: "2026-05-26T18:25:15Z"
updated-at: "2026-05-26T18:25:15Z"
metric-files-to-touch: 5
metric-step-count: 16
has-blockers: false
revision-count: 0
stack-source: confirmed
tags:
  - github-actions
  - release
  - git-cliff
  - pypi
  - github-release
  - nsis
  - post-publish-checks
refs:
  index: 00-index.md
  plan-index: 04-plan.md
  slice-def: 03-slice-release-orchestration.md
  siblings:
    - 04-plan-commit-hygiene.md
    - 04-plan-nsis-installer.md
    - 04-plan-backend-version.md
    - 04-plan-android-versioning.md
  implement: 05-implement-release-orchestration.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout release-orchestration"
---

# Plan: release-orchestration

## Current State

`.github/workflows/release.yml` is 52 lines, single `build-and-publish` job. Triggered on `push: tags: ['v*']`. Workflow-level `permissions: contents: read`; job-level `environment: pypi` + `permissions: id-token: write`. Five steps: `actions/checkout@v4` → `actions/setup-go@v5` (`go-version-file: cli/go.mod`) → `actions/setup-python@v5` (`python-version: "3.12"`) → `Extract version from tag` (`echo "VERSION=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"`) → inline `pip install go-to-wheel && go-to-wheel ./cli ... --output-dir dist/` → `pypa/gh-action-pypi-publish@release/v1`. No `concurrency:`, no `env:`, default `fetch-depth: 1`, no caching, no `${{ secrets.* }}` references — fully OIDC.

`.github/workflows/ci.yml` is 84 lines, four parallel jobs on `ubuntu-latest`: `backend-test` (go vet + test in `backend/`, Go 1.24), `cli-test` (same in `cli/`), `android-build` (`actions/setup-java@v4` Temurin 17 + `gradle/actions/setup-gradle@v4` cache-read-only on PRs + `./gradlew assembleDebug lint testDebugUnitTest` in `android/`), `commitlint-backstop` (`fetch-depth: 0` + `wagoid/commitlint-github-action@v6`). These are the four required status check names that `ci.yml` produces today; the `retest` job in `release.yml` will mirror the first three (commitlint-backstop is PR-only).

`backend/installer/pushkit.nsi` is tracked: `OutFile "pushkit-server-setup.exe"` (output lands at `backend/installer/pushkit-server-setup.exe`), `!ifndef VERSION` default `0.0.0-dev`, `!addplugindir "plugins"` (resolves to `backend/installer/plugins/SimpleSC.dll` — also tracked, ~400 KB), `RequestExecutionLevel admin`, `SetCompressor /SOLID lzma`. The `File "..\pushkit-server.exe"` directive (in the install section, not visible in top-60) resolves from `backend/installer/` → `backend/pushkit-server.exe`. The CI cross-compile job MUST place the binary at exactly that path.

`backend/cmd/server/main.go` has `var Version = "dev"` (line 22) and a `--version` flag handler that runs before `config.Load()`. Output format: `pushkit-server <v>\n` via `printVersion(os.Stdout, Version)`. Cosmetic URL fix is landed in `Makefile:13` and `release.yml:44` (both already read `--url "https://github.com/jayteealao/PushKit"`).

`android/app/build.gradle.kts:15-16`:

```kotlin
versionCode = providers.gradleProperty("versionCodeOverride").orNull?.toIntOrNull() ?: 1
versionName = providers.gradleProperty("versionNameOverride").orNull ?: "1.0"
```

Local-dev defaults preserved. CI must pass `-PversionCodeOverride=<int>` (not `-D`) with `fetch-depth: 0`.

`README.md` has `Quickstart`, `Architecture`, `API Endpoints`, `Configuration (Backend)`, `Project Structure`, `Development setup`, `Testing`. **No `## Releasing`, `## Backend installer`, or shields.io badge.** Both new README sections live in this slice.

`Makefile` has three targets: `build-wheels` (default `VERSION ?= 0.1.0`, inline `pip install go-to-wheel` + `go-to-wheel ./cli ... --output-dir dist/`), `publish` (uses Twine — not used in CI; will not be wired), `clean` (`rm -rf dist/`). The `build-wheels` target is the source-of-truth invocation — CI calls `make build-wheels VERSION=<tag-without-v>` rather than re-inlining.

No `actions/upload-artifact`, `actions/download-artifact`, `gh release create`, `softprops/action-gh-release`, or `concurrency:` usage anywhere in `.github/workflows/`. No `${{ secrets.* }}` references in either workflow. No `cliff.toml` at repo root.

`negrutiu/nsis-install@v2` (per freshness research) is the NSIS install action choice — neither `windows-latest` (now `windows-2025`) nor `windows-2022` preinstalls NSIS 3.12.

## Reuse Opportunities

| Candidate | Where | Match | Recommendation |
|---|---|---|---|
| Existing `build-wheels` step (Set up Go + Set up Python + Extract version + Build wheels) | `release.yml:18-46` | Direct match for the new `build-cli-wheel` job | **Reuse** — promote to its own job; keep `make build-wheels VERSION=$VERSION` invocation. |
| `make build-wheels` Makefile target | `Makefile:5-15` | Source-of-truth wheel build | **Reuse** — CI calls `make build-wheels VERSION=<tag-without-v>`, no re-inlining. |
| PyPI publish step | `release.yml:48-51` | Direct match for `publish-pypi` job | **Reuse** — move to dedicated job with `environment: pypi` + `permissions: id-token: write` + `attestations: false`. |
| `actions/setup-go@v5` go-version pin idiom | `release.yml:22-24` (uses `go-version-file: cli/go.mod`) and `ci.yml:19-22` (uses `go-version: "1.24"`) | Two distinct idioms in the repo | **Reuse mixed** — use `go-version: "1.24"` for the cross-compile job (matches ci.yml; backend module declares 1.24); use `go-version-file: cli/go.mod` only for the wheel build (preserves the existing release.yml choice). |
| `gradle/actions/setup-gradle@v4` cache idiom | `ci.yml:63-66` | Direct match for `build-android-apk` | **Reuse** — copy the `cache-read-only: ${{ github.ref != 'refs/heads/main' }}` idiom; on tag pushes this evaluates `true` (tag refs are not `refs/heads/main`), which is correct — release runs read from main's cache but do not write. |
| `actions/setup-java@v4` Temurin 17 pattern | `ci.yml:57-61` | Direct match | **Reuse** — identical setup for the android job. |
| `commitlint-backstop` workflow-style | `ci.yml:72-83` | Step naming + indentation conventions | **Reuse style** — 2-space indent, Title-Case step names (`Set up Go`, `Build wheels`), kebab-case job names. |
| Workflow-level `permissions: contents: read` | `release.yml:8-9`, `ci.yml:9-10` | Existing posture | **Extend** — release workflow now needs `contents: write` at workflow level (or on `create-github-release` job) so `softprops/action-gh-release@v3` can create the release. Place at job level to minimize blast radius — `contents: write` on `create-github-release` and `post-publish-windows`/`post-publish-linux` (the latter for `gh api repos/.../releases/tags/<tag>`); workflow-level stays `contents: read`. |

No reuse candidates for `actions/upload-artifact`, `actions/download-artifact`, `softprops/action-gh-release`, `orhun/git-cliff-action`, `negrutiu/nsis-install`, `concurrency:`, NSIS install steps, `cliff.toml`, or the README `## Releasing`/`## Backend installer` sections — implement fresh in all cases.

## Likely Files / Areas to Touch

| File | Change type | Why |
|---|---|---|
| `.github/workflows/release.yml` | Rewrite | Expand from 1 job to 9 jobs (8 in default graph + post-publish split into two). Add workflow-level `concurrency:`. |
| `cliff.toml` | New, root | git-cliff config. Init via `git cliff --init keepachangelog`, then tune `tag_pattern` + scope vocabulary. |
| `README.md` | Modify (add 2 sections + 1 badge) | shields.io badge above title; new `## Releasing` section; new `## Backend installer` section. Place after `## Testing`. |
| `.ai/workflows/ship-plan-buildout/03-slice-release-orchestration.md` | Modify (1 line, optional) | Inherits the nsis-installer slice's PO answer that flipped service-component default-checked — already reconciled in `04-plan-nsis-installer.md § Blockers`. Re-confirm no further reconciliation needed here. |

No changes to `Makefile` (already correct), `ci.yml` (sibling), `backend/`, `android/`, `cli/`, `commitlint.config.cjs`, `lefthook.yml`, `package.json`, `.gitignore`. No new files outside `cliff.toml`.

## Proposed Change Strategy

Single-track integrator. Build the 8-job graph plus the two post-publish jobs in one PR-shaped commit set on `feat/ship-plan-buildout`, after `android-versioning` is implemented + verified. Use `softprops/action-gh-release@v3` (per discovery Round 1) for the release-creation step to avoid the documented `gh release create` upload-flake risk. Cross-compile on Linux + NSIS-wrap on Windows (cheapest topology; aligns with NSIS slice's relative-path contract). Pin all actions to match existing workflows; introduce `actions/upload-artifact@v4` and `actions/download-artifact@v4` (v3 hard-deprecated). Disable Sigstore attestations on the PyPI publish (v0.x posture). Document the break-glass `PYPI_API_TOKEN` recovery procedure in the new `## Releasing` README section — no fallback job in `release.yml` itself.

**Job graph (8 default + 2 post-publish):**

```
tag-guard ──> retest ──┬──> build-cli-wheel ──────┐
                       ├──> build-backend-binary ─┐
                       ├──> build-android-apk ────┤
                       └──> generate-changelog ───┤
                                                  │
build-backend-binary ──> build-backend-installer ─┤
                                                  │
                              ┌───────────────────┘
                              ▼
                          publish-pypi
                              │
                              ▼
                       create-github-release ──┬──> post-publish-linux
                                               └──> post-publish-windows
```

`build-backend-binary` is the Linux cross-compile job (Go → `pushkit-server.exe`). `build-backend-installer` is the `windows-2022` NSIS-wrap job that consumes it. They are split because the cross-compile is cheap-and-fast on Linux while makensis needs Windows; splitting also lets the build-backend-binary artifact be the source-of-truth `pushkit-server.exe` PE for any future consumer (e.g., a macOS notarisation worker).

Per shape Risks (line 130-131): if `publish-pypi` succeeds but `create-github-release` fails, PyPI has the wheel without a corresponding GH Release. This is accepted residual risk; recovery is manual `gh release create` against the existing tag, or `pip yank` + `-rc.N+1`. The graph order is deliberate — PyPI first (slow, high-reliability) then GH Release (fast, easy to retry).

## Step-by-Step Plan

### Step 1 — Author `cliff.toml`

Run locally:

```bash
git cliff --init keepachangelog
```

This seeds a Keep-a-Changelog template at repo root. Edit:

- Set `[git]` `tag_pattern = "^v[0-9]+\\.[0-9]+\\.[0-9]+(?:-(?:rc|alpha|beta)\\.[0-9]+)?$"` to match our scheme (`v0.1.0`, `v0.1.0-rc.1`).
- Set `[changelog]` `header` to `# Changelog\n\nAll notable changes to this project will be documented in this file.\n`.
- Ensure `[git]` `conventional_commits = true` and the `commit_parsers` list includes (at minimum) `feat`, `fix`, `perf`, `refactor`, `style`, `test`, `docs`, `build`, `ci`, `chore`, `revert`. Scope vocabulary (backend/cli/android/ci/docs/deps/installer/release) does NOT need an explicit list — git-cliff renders scopes as-is when present.
- Ensure `[git]` `filter_unconventional = true` so the 6 pre-existing non-conventional commits are dropped from the v0.1.0-rc.1 changelog rather than crashing the render.

Commit shape: `feat(release): add git-cliff configuration for changelog generation`.

### Step 2 — Rewrite `.github/workflows/release.yml`

Replace the entire 52-line file. Skeleton (annotated; the implement stage produces the YAML):

```yaml
name: Release

on:
  push:
    tags: ["v*"]

permissions:
  contents: read

concurrency:
  group: release-${{ github.ref_name }}
  cancel-in-progress: false

jobs:
  tag-guard:           # ubuntu-latest, fetch-depth: 0 (need history)
  retest:              # needs: [tag-guard], matrix-strategy 3 jobs OR 3 separate jobs
    backend-test:      # mirrors ci.yml backend-test
    cli-test:          # mirrors ci.yml cli-test
    android-build:     # mirrors ci.yml android-build (defaults, no -P flags)
  build-cli-wheel:     # needs: [retest], ubuntu-latest, env: pypi NOT required here (no publish)
  build-backend-binary:  # needs: [retest], ubuntu-latest (Linux cross-compile)
  build-backend-installer:  # needs: [build-backend-binary], windows-2022 (NSIS-wrap)
  build-android-apk:   # needs: [retest], ubuntu-latest, fetch-depth: 0 + -P flags
  generate-changelog:  # needs: [retest], ubuntu-latest, fetch-depth: 0
  publish-pypi:        # needs: [build-cli-wheel, build-backend-installer, build-android-apk, generate-changelog]
                       # all-or-nothing: gated by the four build jobs completing
                       # environment: pypi, permissions: id-token: write, attestations: false
  create-github-release: # needs: [publish-pypi, build-backend-installer, build-android-apk, generate-changelog]
                         # permissions: contents: write, uses softprops/action-gh-release@v3
  post-publish-linux:  # needs: [create-github-release], ubuntu-latest
  post-publish-windows: # needs: [create-github-release], windows-2022
```

Per-job specs follow.

#### 2a. `tag-guard` (ubuntu-latest)

```
Checkout (fetch-depth: 0)
Verify tag is on main:
  git merge-base --is-ancestor "$GITHUB_SHA" origin/main
  # exits 0 if reachable, non-zero otherwise — fail-loud
```

The `git merge-base --is-ancestor` exit code is the gate. No conditional `if:` on downstream jobs — they `needs: [tag-guard]` which already gates them. Annotated rationale in step output: `echo "::error::Tag $GITHUB_REF_NAME is not on main"` before exiting.

#### 2b. `retest` — three sub-jobs OR three steps in one job (decision)

Sub-decision deferred to implement stage. Recommended: **three separate jobs** (`retest-backend`, `retest-cli`, `retest-android`) for failure isolation and parallelism. Matrix is rejected — the three setups (Go × 2 + Java) are different enough that matrix `include:` becomes denser than three explicit jobs. Each job mirrors its `ci.yml` counterpart exactly (Go 1.24, Temurin 17, Gradle setup-gradle@v4, working-directory per module).

`needs: [tag-guard]` on each.

#### 2c. `build-cli-wheel` (ubuntu-latest, needs: [retest-backend, retest-cli, retest-android])

```
Checkout (default fetch-depth — wheel build doesn't need history)
Set up Go (go-version-file: cli/go.mod)
Set up Python (python-version: "3.12")
Extract version: echo "VERSION=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"
Build wheel: make build-wheels VERSION=${{ steps.version.outputs.VERSION }}
Upload artifact:
  uses: actions/upload-artifact@v4
  with:
    name: cli-wheel
    path: dist/*.whl
    retention-days: 7
```

#### 2d. `build-backend-binary` (ubuntu-latest, needs: [retest-*])

```
Checkout (default fetch-depth)
Set up Go (go-version: "1.24" — matches ci.yml backend-test)
Extract version: VERSION="${GITHUB_REF_NAME#v}"
Cross-compile:
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
    go build -ldflags "-X main.Version=$VERSION" \
    -o pushkit-server.exe ./cmd/server
  # working-directory: backend
Upload artifact:
  uses: actions/upload-artifact@v4
  with:
    name: backend-windows-binary
    path: backend/pushkit-server.exe
    retention-days: 7
```

The `-X main.Version=$VERSION` uses the **stripped** form (`0.1.0-rc.1` not `v0.1.0-rc.1`) per `backend-version` plan's note ("This slice is agnostic — `printVersion` prints whatever is in `Version`"). Smoke-test expects `pushkit-server 0.1.0-rc.1` (per shape AC7), so the stripped form is correct.

#### 2e. `build-backend-installer` (windows-2022, needs: [build-backend-binary])

```
Checkout (default fetch-depth)
Download backend binary:
  uses: actions/download-artifact@v4
  with:
    name: backend-windows-binary
    path: backend/
  # places pushkit-server.exe at backend/pushkit-server.exe — matches the
  # NSIS File "..\pushkit-server.exe" directive's resolution from backend/installer/.
Install NSIS 3.12:
  uses: negrutiu/nsis-install@v2
  with:
    nsis-version: "3.12"   # exact version pin, locks CVE-2025-43715 fix
Extract version: VERSION="${{ github.ref_name }}"; echo "VERSION_STRIPPED=${VERSION#v}" >> "$GITHUB_OUTPUT"
Run makensis:
  makensis /V3 /DVERSION=${{ steps.version.outputs.VERSION_STRIPPED }} backend/installer/pushkit.nsi
  # output lands at backend/installer/pushkit-server-setup.exe (per nsis-installer plan)
Upload artifact:
  uses: actions/upload-artifact@v4
  with:
    name: windows-installer
    path: backend/installer/pushkit-server-setup.exe
    retention-days: 7
```

Note: `negrutiu/nsis-install@v2`'s exact input name should be verified against the action's README during implement (some marketplace actions use `version:` instead of `nsis-version:` — the plan stage cannot ground-truth this without running it). Pin the action by floating tag `@v2`; the `tech-research-enforcer` skill should re-verify the action's current state at implement time.

#### 2f. `build-android-apk` (ubuntu-latest, needs: [retest-*])

```
Checkout:
  uses: actions/checkout@v4
  with:
    fetch-depth: 0    # required for git rev-list --count HEAD
Set up Java (Temurin 17)
Set up Gradle (cache-read-only: ${{ github.ref != 'refs/heads/main' }} — evaluates true on tag refs)
Extract version overrides:
  VERSION_CODE=$(git rev-list --first-parent --count HEAD)
  VERSION_NAME="${GITHUB_REF_NAME#v}"
  echo "VERSION_CODE=$VERSION_CODE" >> "$GITHUB_OUTPUT"
  echo "VERSION_NAME=$VERSION_NAME" >> "$GITHUB_OUTPUT"
Build APK:
  ./gradlew assembleDebug \
    -PversionCodeOverride=${{ steps.overrides.outputs.VERSION_CODE }} \
    -PversionNameOverride=${{ steps.overrides.outputs.VERSION_NAME }}
  # working-directory: android
  # NOTE: -P (project property), NOT -D (system property). The android-versioning
  # plan flagged this as the #1 silent-failure mode.
Rename APK:
  cp android/app/build/outputs/apk/debug/app-debug.apk pushkit-android.apk
Upload artifact:
  uses: actions/upload-artifact@v4
  with:
    name: android-apk
    path: pushkit-android.apk
    retention-days: 7
```

Build `assembleDebug` (not `assembleRelease`) per shape: Android release-signing is explicitly out of scope for v0.x. The output filename in `android/app/build/outputs/apk/debug/` is `app-debug.apk` — rename to `pushkit-android.apk` before upload so the GH Release asset name matches shape AC8.

#### 2g. `generate-changelog` (ubuntu-latest, needs: [retest-*])

```
Checkout:
  uses: actions/checkout@v4
  with:
    fetch-depth: 0    # git-cliff needs full history
Run git-cliff:
  uses: orhun/git-cliff-action@v4
  with:
    config: cliff.toml
    args: --unreleased --tag ${{ github.ref_name }} -o CHANGELOG-${{ github.ref_name }}.md
    # --unreleased: include all unreleased commits (works for first release)
    # --tag <ref>: write <ref> as the section heading
    # -o <file>: per-release file, not committed back to main
Upload artifact:
  uses: actions/upload-artifact@v4
  with:
    name: changelog
    path: CHANGELOG-${{ github.ref_name }}.md
    retention-days: 7
```

Per ship-plan Block B + commit-hygiene cross-cutting concerns, **no CHANGELOG.md is committed to main**. The artifact is consumed by `create-github-release`'s `body_path` and then discarded with the workflow run.

#### 2h. `publish-pypi` (ubuntu-latest, needs: all four build jobs)

```
needs: [build-cli-wheel, build-backend-installer, build-android-apk, generate-changelog]
environment: pypi
permissions:
  id-token: write
steps:
  Download wheel:
    uses: actions/download-artifact@v4
    with:
      name: cli-wheel
      path: dist/
  Publish:
    uses: pypa/gh-action-pypi-publish@release/v1
    with:
      packages-dir: dist/
      attestations: false   # v0.x posture; SLSA hardening deferred per intake
```

The `needs:` includes ALL four build jobs (not just the wheel) per shape's all-or-nothing requirement: if any artifact build failed, PyPI does not receive anything. This is the canonical "fail-loud" gate.

#### 2i. `create-github-release` (ubuntu-latest, needs: [publish-pypi, build-backend-installer, build-android-apk, generate-changelog])

```
permissions:
  contents: write   # required for softprops to create the release
steps:
  Download all build artifacts:
    uses: actions/download-artifact@v4
    with:
      pattern: "{windows-installer,android-apk,changelog}"
      merge-multiple: true
      path: release-assets/
    # Lands: release-assets/pushkit-server-setup.exe
    #        release-assets/pushkit-android.apk
    #        release-assets/CHANGELOG-v0.1.0-rc.1.md
  Generate SHA256SUMS:
    cd release-assets
    sha256sum pushkit-server-setup.exe pushkit-android.apk > SHA256SUMS
    # The wheel is NOT included — PyPI is canonical for it (per shape).
  Determine prerelease:
    if [[ "${GITHUB_REF_NAME}" =~ -(rc|alpha|beta) ]]; then
      echo "IS_PRERELEASE=true" >> "$GITHUB_OUTPUT"
    else
      echo "IS_PRERELEASE=false" >> "$GITHUB_OUTPUT"
    fi
  Create release:
    uses: softprops/action-gh-release@v3
    with:
      tag_name: ${{ github.ref_name }}
      body_path: release-assets/CHANGELOG-${{ github.ref_name }}.md
      prerelease: ${{ steps.prerelease.outputs.IS_PRERELEASE }}
      make_latest: ${{ steps.prerelease.outputs.IS_PRERELEASE == 'false' && 'true' || 'false' }}
      files: |
        release-assets/pushkit-server-setup.exe
        release-assets/pushkit-android.apk
        release-assets/SHA256SUMS
      fail_on_unmatched_files: true
```

The `release-assets/` staging directory keeps the SHA256SUMS-generation step's `cd` scope clean (only the assets to hash, not the workflow files). `fail_on_unmatched_files: true` surfaces a missing asset as a job failure rather than silently shipping a partial release.

The wheel artifact is deliberately NOT attached. PyPI is the canonical location per shape AC6.

#### 2j. `post-publish-linux` (ubuntu-latest, needs: [create-github-release])

```
permissions:
  contents: read    # gh api needs read on releases
steps:
  Set up Python (python-version: "3.12")    # for fresh-resolve
  Extract normalized version:
    # PEP 440 normalization: v0.1.0-rc.1 → 0.1.0rc1
    VERSION_PEP440=$(python -c "from packaging.version import Version; print(Version('${GITHUB_REF_NAME#v}'))")
    echo "VERSION_PEP440=$VERSION_PEP440" >> "$GITHUB_OUTPUT"
  fresh-resolve (AC6):
    pip install --no-cache-dir --pre pushkit==${{ steps.normalize.outputs.VERSION_PEP440 }}
    pushkit --version
    # Expected output: "pushkit version 0.1.0-rc.1" (or normalized form)
  github-release (AC5 partial — automated assertion):
    gh api repos/jayteealao/PushKit/releases/tags/${{ github.ref_name }} \
      --jq '.assets[].name' | sort > actual-assets.txt
    echo -e "SHA256SUMS\npushkit-android.apk\npushkit-server-setup.exe" | sort > expected-assets.txt
    diff actual-assets.txt expected-assets.txt
  Download APK + SHA256SUMS:
    gh release download ${{ github.ref_name }} \
      --pattern "pushkit-android.apk" --pattern "SHA256SUMS" \
      --pattern "pushkit-server-setup.exe" \
      --dir post-publish-assets/
  apk-probe (AC8):
    # Install Android command-line tools to get aapt
    sudo apt-get update && sudo apt-get install -y aapt
    aapt dump badging post-publish-assets/pushkit-android.apk | grep -E "versionCode|versionName"
    # Assert versionName=${GITHUB_REF_NAME#v}
    # Assert versionCode is non-empty integer matching git rev-list --first-parent --count HEAD at the tag
  sha256sum-verify (AC10):
    cd post-publish-assets
    sha256sum -c SHA256SUMS
    # Must show pushkit-server-setup.exe: OK + pushkit-android.apk: OK
env:
  GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Note: the Linux `apt-get install aapt` may not pull the exact `build-tools 34.0.0` aapt the android-versioning slice's local-verify uses. Alternative: use `actions/setup-android-sdk` or run via the Android SDK already on the runner image. Implement stage decides; both produce the same `aapt dump badging` output. The runner image's pre-installed Android SDK location is `/usr/local/lib/android/sdk` — using its `build-tools/<ver>/aapt` is the cleanest path.

#### 2k. `post-publish-windows` (windows-2022, needs: [create-github-release])

```
permissions:
  contents: read
steps:
  Download installer from GH Release (smoke-test source per Round 3):
    gh release download ${{ github.ref_name }} \
      --pattern "pushkit-server-setup.exe" --dir .
    # Tests the real upload + GH Release API path end-to-end
  smoke-test (AC7):
    Start-Process -FilePath "pushkit-server-setup.exe" -ArgumentList "/S" -Wait
    & "$env:ProgramFiles\PushKit\pushkit-server.exe" --version
    # Expected output: "pushkit-server 0.1.0-rc.1"
  Assert output matches:
    $expected = "pushkit-server ${{ github.ref_name }}".Replace("pushkit-server v", "pushkit-server ")
    $actual = (& "$env:ProgramFiles\PushKit\pushkit-server.exe" --version).Trim()
    if ($actual -ne $expected) { Write-Error "Smoke-test mismatch: expected '$expected', got '$actual'"; exit 1 }
env:
  GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Silent install is fire-and-forget — the installer's `RequestExecutionLevel admin` triggers UAC, but GitHub Actions runners run as Administrator by default. No extra elevation needed.

Per the nsis-installer plan, silent install now REGISTERS the service (Round 2 PO decision). The smoke-test does not start/stop the service — it just verifies the binary is in place and `--version` works. The service-lifecycle interactive verification is the maintainer's local AC7 path, not CI's.

### Step 3 — Add `## Releasing` section to `README.md`

Place after the existing `## Testing` section. Content shape:

```markdown
## Releasing

PushKit ships via a tag-and-walk-away pipeline. Pushing a `v*` tag from `main` triggers the full release workflow.

### Cutting a release

```bash
git tag -a v0.1.0-rc.1 -m ""
git push --tags
```

That is the only manual action. The release workflow (`release.yml`) runs automatically and produces:

- **PyPI**: `pushkit==0.1.0rc1` published via OIDC Trusted Publishing (PEP 440 normalized).
- **GitHub Release**: marked prerelease for `-rc`/`-alpha`/`-beta` tags. Assets: `pushkit-server-setup.exe` (Windows installer), `pushkit-android.apk` (Android debug APK), `SHA256SUMS` (verifies asset integrity). Release notes auto-generated by git-cliff from Conventional Commits.

### Pipeline overview

1. `tag-guard` — refuses tags not reachable from `origin/main`.
2. `retest-{backend,cli,android}` — re-runs the pre-merge CI gates.
3. `build-cli-wheel` (Linux) — `make build-wheels VERSION=<tag-without-v>`.
4. `build-backend-binary` (Linux) — cross-compiles `pushkit-server.exe`.
5. `build-backend-installer` (Windows) — installs NSIS 3.12, runs `makensis`.
6. `build-android-apk` (Linux) — sets `versionCode=git rev-list --count HEAD`, `versionName=<tag-without-v>`.
7. `generate-changelog` (Linux) — runs `git-cliff --unreleased --tag <tag>`.
8. `publish-pypi` (Linux) — OIDC publish via `pypa/gh-action-pypi-publish@release/v1` with `attestations: false` (SLSA hardening out of scope for v0.x).
9. `create-github-release` (Linux) — creates the GH Release with all assets + SHA256SUMS + notes via `softprops/action-gh-release@v3`.
10. `post-publish-linux` + `post-publish-windows` — `pip install --pre`, `aapt dump badging`, `sha256sum -c SHA256SUMS`, silent install + `--version`.

Total wall time: ≤ 15 minutes for v0.1.0-rc.1 (shape NFR).

### Watching the run

```bash
gh run watch --repo jayteealao/PushKit
```

### Rollback

If a release ships with a bug:

1. **Yank the wheel from PyPI**: log into PyPI, mark `0.1.0rc1` as yanked. Yanking blocks `pip install pushkit==0.1.0rc1` for new installs but keeps the file downloadable for pins.
2. **Cut a new release tag** at the fix commit (e.g., `v0.1.0-rc.2`). The full pipeline runs again.
3. **The original GitHub Release stays** — manually delete its assets if desired (via web UI or `gh release delete-asset`), but the tag itself stays in git history.

Per shape: once a tag is published, it's final. We do not re-run `release.yml` on the same tag; `gh release create` will fail on a duplicate tag. Recovery is always `pip yank` + `-rc.N+1`.

### Required GitHub repo settings (one-time, manual)

These are configured outside CI; document for future maintainers:

1. **PyPI Trusted Publisher** (already configured per intake): owner `jayteealao`, repo `PushKit`, workflow `release.yml`, environment `pypi`.
2. **Tag-protection rule**: restrict `v*` tag creation to maintainers via Settings → Rules → Rulesets. The `tag-guard` job is a CI-level backstop; this is the GitHub-level guard.
3. **`PYPI_API_TOKEN` sealed secret** (break-glass only): create a Repository Secret named `PYPI_API_TOKEN` with a PyPI API token scoped to the `pushkit` project. The default publish path is 100% OIDC; this secret is the recovery path if OIDC ever fails.

### Break-glass: PyPI OIDC outage

If `pypa/gh-action-pypi-publish` cannot get an OIDC token (PyPI side outage, clock skew, etc.), temporarily swap the `publish-pypi` step:

```yaml
- uses: pypa/gh-action-pypi-publish@release/v1
  with:
    packages-dir: dist/
    password: ${{ secrets.PYPI_API_TOKEN }}   # break-glass only
    attestations: false
```

Push the tag again with a bumped `-rc.N+1`. **Revert this change** before merging the next normal release PR — the default path is OIDC.
```

### Step 4 — Add `## Backend installer` section to `README.md`

Place after the new `## Releasing` section. Content shape:

```markdown
## Backend installer

The Windows installer (`pushkit-server-setup.exe`) is distributed via GitHub Releases.

### Download

Latest release: https://github.com/jayteealao/PushKit/releases/latest

For prereleases (release candidates): https://github.com/jayteealao/PushKit/releases

### Install

**Interactive (UAC prompt):**

```powershell
.\pushkit-server-setup.exe
```

Default components: `pushkit-server.exe` in `%ProgramFiles%\PushKit\`, Start Menu shortcut, Apps & Features entry. The optional Windows service component (default-checked) registers `PushKitServer` as a manual-start Windows service.

**Silent:**

```powershell
.\pushkit-server-setup.exe /S
```

The silent path installs all default-checked components, including the Windows service. Requires an elevated PowerShell session — UAC will prompt non-interactively if the session is not already elevated.

### Uninstall

Settings → Apps & Features → PushKit Server → Uninstall. Or silently:

```powershell
& "$env:ProgramFiles\PushKit\uninstall.exe" /S
```

### SmartScreen warning

The installer is not code-signed (out of scope for v0.x). On first launch Windows SmartScreen displays "Unknown publisher". Click **More info** → **Run anyway** to proceed. Code-signing is on the roadmap for a future release.
```

### Step 5 — Add shields.io badge to top of `README.md`

Insert immediately under the `# PushKit — S3 Push System` title:

```markdown
# PushKit — S3 Push System

[![Latest release](https://img.shields.io/github/v/release/jayteealao/PushKit?include_prereleases=true&sort=semver)](https://github.com/jayteealao/PushKit/releases/latest)

Upload files from CLI, download them on Android. ...
```

The badge renders against the GitHub Releases API. shields.io cache TTL is 5–30 min after a release event (per shape research) — humans wait briefly before AC12 verification.

### Step 6 — Pre-flight validation (local, before pushing the workflow)

Before opening the PR / cutting the validation tag:

```bash
# 1. cliff.toml validity
git cliff --tag v0.1.0-rc.1 --unreleased -o /tmp/test-changelog.md
# Expected: non-empty file with ## v0.1.0-rc.1 heading + at least one entry

# 2. PEP 440 normalization
python -c "from packaging.version import Version; print(Version('0.1.0-rc.1'))"
# Expected: 0.1.0rc1

# 3. release.yml syntax
gh workflow view release.yml --repo jayteealao/PushKit
# Or via actionlint locally (if installed)

# 4. README rendering
grep -E '^## ' README.md
# Expected: Architecture, Quickstart, API Endpoints, Configuration (Backend),
#           Project Structure, Development setup, Testing, Releasing, Backend installer
```

### Step 7 — Commit + push the workflow changes

Conventional commits:

- `feat(release): add tag-driven release pipeline with NSIS installer + APK + git-cliff`
- `docs(release): document tag-and-walk-away release flow + break-glass recovery`

Split is for readable git history; can be one commit if size permits.

### Step 8 — Pre-tag GitHub repo setting checks (one-time, manual)

The maintainer confirms these are configured (per the new README `## Releasing` "Required GitHub repo settings"):

1. PyPI Trusted Publisher exists with the quartet `jayteealao` / `PushKit` / `release.yml` / `pypi`.
2. Tag-protection ruleset blocks `v*` tag creation by non-maintainers.
3. `PYPI_API_TOKEN` repository secret exists (sealed, break-glass only).

### Step 9 — Throwaway-tag validation (AC4 + AC13)

Per the slice acceptance criteria, two interactive validations on throwaway tags:

**AC4 — tag-guard rejects off-main tags**

```bash
git checkout -b test/off-main-tag-validation
git commit --allow-empty -m "chore: deliberate off-main commit for tag-guard test"
git push origin test/off-main-tag-validation
git tag v0.0.0-tagguard-test
git push origin v0.0.0-tagguard-test
gh run watch --repo jayteealao/PushKit
# Expected: tag-guard fails red; all downstream jobs show "skipped"
# Then: delete throwaway tag + branch
git push origin :v0.0.0-tagguard-test
git push origin :test/off-main-tag-validation
```

**AC13 — Single-failure abort**

```bash
git checkout -b test/nsis-abort-validation
# Introduce a deliberate NSIS error: add a bad File directive to backend/installer/pushkit.nsi
# e.g., File "..\nonexistent-file.txt"
git commit -am "chore: deliberately break NSIS for validation"
git push origin test/nsis-abort-validation
# Merge to feat/ship-plan-buildout via PR (or push directly if branch protection allows)
# Tag and push:
git tag v0.0.0-nsis-abort-test
git push origin v0.0.0-nsis-abort-test
gh run watch --repo jayteealao/PushKit
# Expected: build-backend-installer fails red; publish-pypi, create-github-release skipped
# Then: revert the NSIS break, delete throwaway tag
```

Both validations are throwaway and must be cleaned up before the real `v0.1.0-rc.1` tag.

### Step 10 — Validation tag push (the real one)

Per slice goal: the workflow's success metric is `v0.1.0-rc.1` going end-to-end:

```bash
# On feat/ship-plan-buildout, after all five slices verified and the PR merges to main:
git checkout main
git pull
git tag -a v0.1.0-rc.1 -m ""
git push origin v0.1.0-rc.1
gh run watch --repo jayteealao/PushKit
```

The watch terminates green when `post-publish-windows` succeeds. Then the AC verification steps (AC5/AC9/AC12 human-in-the-loop, others automated in CI) run per `06-verify-release-orchestration.md`.

### Step 11 — README badge cache check (AC12)

Wait ≥ 5 min after `create-github-release` green; refresh `https://github.com/jayteealao/PushKit` in the browser. Badge should show `v0.1.0-rc.1`. If still cached after 30 min, force-refresh with browser DevTools (Ctrl+Shift+R).

## CI Contract (consumed by upstream slices)

- **`commit-hygiene`** — provides `ci.yml` workflow file shape + job names (`backend-test`, `cli-test`, `android-build`) and Conventional Commits enforcement (required for git-cliff to render entries). This slice's `retest-*` jobs duplicate the `ci.yml` shape verbatim per shape Round 1 ("Heavy on tag retest").
- **`nsis-installer`** — provides `backend/installer/pushkit.nsi` with `File "..\pushkit-server.exe"` directive. This slice's `build-backend-installer` job places the binary at `backend/pushkit-server.exe` before `makensis` runs. NSIS 3.12 installed via `negrutiu/nsis-install@v2`.
- **`backend-version`** — provides `var Version` + `--version` flag + `printVersion` helper. This slice's `build-backend-binary` job builds with `-ldflags "-X main.Version=$VERSION_STRIPPED"`; `post-publish-windows` smoke-test asserts the output.
- **`android-versioning`** — provides `providers.gradleProperty` overrides. This slice's `build-android-apk` job passes `-PversionCodeOverride=$(git rev-list --count HEAD) -PversionNameOverride=${GITHUB_REF_NAME#v}` with `fetch-depth: 0`. **`-P` not `-D`** — silent failure otherwise (android-versioning plan flagged this).

## Test / Verification Plan

### Automated checks

- **YAML lint**: `actionlint` locally (if installed) to validate `release.yml` syntax before pushing. Optional but cheap.
- **No unit tests**: this slice produces YAML + Markdown + a TOML config. No production code added; no test framework applies. Verification is entirely the throwaway-tag + validation-tag runs.

### Interactive verification (human-in-the-loop)

Stack context (from `00-index.md`): `platforms: [service, cli, android]`, `testing: [go-testing, gradle-junit, android-lint]`, `available-skills: tech-research-enforcer, android-cli, lazylogcat, framework-conventions-guide, testing-setup`, `available-mcp: web-reader, web-search-prime`. Maintainer's local machine is Windows 11 Pro (10.0.26100). For each AC the verification uses `gh` CLI + the maintainer's Windows 11 shell (`PowerShell`/`Git Bash` depending on the command). No tooling outside `stack:` is required for any AC.

**AC4 — `tag-guard` aborts off-main tag**

- **What to verify:** Push a tag from a non-main branch; `tag-guard` job exits non-zero and all downstream jobs are skipped.
- **Platform & tool:** Windows 11 Pro + `git` + `gh` CLI. Stack: `[service, cli]`, no skills needed.
- **Companion skills:** None.
- **Steps:** Step 9's "AC4" sub-block above (create branch, commit, push, tag, watch, clean up).
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac4/`
  - `actions-run-graph.png` — screenshot showing `tag-guard` red + all others skipped.
  - `terminal-push-and-watch.txt` — PowerShell transcript of the push + watch sequence.
- **Pass criteria:** `tag-guard` red; `retest-*` + `build-*` + `publish-pypi` + `create-github-release` + `post-publish-*` all `skipped`. No PyPI artifact uploaded; no GH Release for the throwaway tag.

**AC5 — `gh release view` lists assets + notes + prerelease**

- **What to verify:** After `v0.1.0-rc.1` workflow completes, `gh release view --json` shows 3 assets + non-empty body + `prerelease: true`.
- **Platform & tool:** Windows 11 + `gh` CLI + browser.
- **Companion skills:** `available-mcp: web-reader` (optional) to fetch the GH Release page HTML for archival.
- **Steps:**
  1. `gh release view v0.1.0-rc.1 --repo jayteealao/PushKit`
  2. `gh release view v0.1.0-rc.1 --repo jayteealao/PushKit --json assets,body,prerelease`
  3. Open `https://github.com/jayteealao/PushKit/releases/tag/v0.1.0-rc.1` in browser.
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac5/`
  - `gh-release-view.txt` — terminal output.
  - `gh-release-json.txt` — JSON dump.
  - `release-page-browser.png` — screenshot showing "Pre-release" label, three assets, non-empty notes body.
- **Pass criteria:** JSON `assets` array contains `pushkit-server-setup.exe`, `pushkit-android.apk`, `SHA256SUMS` (3 entries). `prerelease: true`. `body` non-empty.

**AC6 — Fresh PyPI install + `--version`**

- **What to verify:** CI job `post-publish-linux` step `fresh-resolve` succeeds.
- **Platform & tool:** GitHub Actions CI (Linux runner) — automated. Maintainer observation via `gh run view --log`.
- **Companion skills:** None.
- **Steps:** Step 2j (CI step). Maintainer reviews the step log:
  1. `gh run view <run-id> --repo jayteealao/PushKit --log` — filter to `post-publish-linux` → `fresh-resolve`.
  2. Confirm `pip install` exit 0 and `pushkit --version` output `pushkit version 0.1.0-rc.1` (or normalized).
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac6/post-publish-checks-fresh-resolve.txt` — CI step log.
- **Pass criteria:** `pip install` exit 0; `--version` output matches.

**AC7 — Silent install + `--version` smoke-test on `windows-2022`**

- **What to verify:** CI job `post-publish-windows` step `smoke-test` succeeds.
- **Platform & tool:** GitHub Actions CI (`windows-2022`) — automated.
- **Companion skills:** None.
- **Steps:** Step 2k (CI step). Maintainer reviews log:
  1. `gh run view <run-id> --log` — filter to `post-publish-windows`.
  2. Confirm silent install exit 0; `--version` output exactly `pushkit-server 0.1.0-rc.1`.
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac7/smoke-test-ci-log.txt`.
- **Pass criteria:** Both commands exit 0; `--version` output matches.

**AC8 — `aapt dump badging` reports correct versionName + versionCode**

- **What to verify:** CI job `post-publish-linux` step `apk-probe` succeeds.
- **Platform & tool:** GitHub Actions CI (Linux runner with pre-installed Android SDK build-tools) — automated. Maintainer also runs locally as belt-and-suspenders.
- **Companion skills:** `available-skills: android-cli` (optional, if needed for local SDK path resolution).
- **Steps (CI):** Step 2j sub-step `apk-probe`. Maintainer review via log.
- **Steps (local belt-and-suspenders, Windows 11 + Git Bash):**
  1. `gh release download v0.1.0-rc.1 --repo jayteealao/PushKit -A pushkit-android.apk`
  2. `& "$env:ANDROID_HOME\build-tools\34.0.0\aapt.exe" dump badging pushkit-android.apk | Select-String "versionCode|versionName"`
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac8/`
  - `apk-probe-ci-log.txt` — CI step log.
  - `aapt-local.txt` — optional local terminal output.
- **Pass criteria:** Output contains `versionName='0.1.0-rc.1'` and `versionCode='<N>'` where N matches `git rev-list --first-parent --count HEAD` at the tagged commit. CI step exits 0.

**AC9 — Release notes contain `## v0.1.0-rc.1` heading + categorized entry**

- **What to verify:** Human reads the GH Release notes; finds the version heading + at least one categorized entry.
- **Platform & tool:** Windows 11 + browser / `gh` CLI.
- **Companion skills:** `available-mcp: web-reader` (optional) for note extraction.
- **Steps:**
  1. `gh release view v0.1.0-rc.1 --repo jayteealao/PushKit --json body --jq .body` — print notes.
  2. Human inspects: find `## v0.1.0-rc.1` (or `## [v0.1.0-rc.1]`) and ≥1 entry under a category heading (e.g., `### Features`, `### Bug Fixes`).
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac9/`
  - `release-notes.txt` — full body.
  - `release-notes-browser.png` — screenshot of rendered notes.
- **Pass criteria:** Heading present, ≥1 categorized entry; non-empty body.

**AC10 — `sha256sum -c SHA256SUMS` passes for all assets**

- **What to verify:** All entries in `SHA256SUMS` verify OK.
- **Platform & tool:** GitHub Actions CI (Linux) — automated in `post-publish-linux`. Also local belt-and-suspenders on Windows 11.
- **Companion skills:** None.
- **Steps (CI):** Step 2j sub-step `sha256sum-verify`.
- **Steps (local belt-and-suspenders, Windows 11 PowerShell):**
  ```powershell
  cd $env:TEMP
  gh release download v0.1.0-rc.1 --repo jayteealao/PushKit
  Get-Content SHA256SUMS | ForEach-Object {
    $h,$f = $_ -split "  ", 2
    $actual = (Get-FileHash $f -Algorithm SHA256).Hash.ToLower()
    if ($actual -ne $h) { Write-Error "MISMATCH: $f"; exit 1 }
    Write-Host "$f: OK"
  }
  ```
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac10/sha256sum-output.txt`.
- **Pass criteria:** Every line `OK`; exit 0; no `MISMATCH` or `FAILED`.

**AC12 — shields.io badge renders `v0.1.0-rc.1` within 30 min**

- **What to verify:** README badge on github.com shows the tag.
- **Platform & tool:** Windows 11 + browser.
- **Companion skills:** `available-mcp: web-reader` (optional, programmatic check of the SVG label).
- **Steps:**
  1. Wait ≥ 5 min after `create-github-release` job green.
  2. Open `https://github.com/jayteealao/PushKit` in browser; observe badge.
  3. If still cached: Ctrl+Shift+R force-refresh, wait another 5–25 min (shields.io TTL 5–30 min).
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac12/`
  - `readme-badge.png` — screenshot.
  - `badge-svg-text.txt` — optional extracted label.
- **Pass criteria:** Badge text reads `v0.1.0-rc.1`. Manual retry once if shields.io cache lags past 30 min.

**AC13 — Single-failure abort (deliberate NSIS error)**

- **What to verify:** Deliberate NSIS error on throwaway tag → `build-backend-installer` red, `publish-pypi` + `create-github-release` skipped.
- **Platform & tool:** Windows 11 + `git` + `gh` CLI + browser (Actions UI).
- **Companion skills:** None.
- **Steps:** Step 9's "AC13" sub-block (create branch, break NSIS, tag, watch, clean up).
- **Evidence capture:** `.ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/ac13/`
  - `actions-run-graph.png` — screenshot showing red NSIS + skipped downstream.
  - `nsis-job-log.txt` — log tail with the compile error.
  - `release-list.txt` — `gh release list` confirming no release for the throwaway tag.
- **Pass criteria:** `build-backend-installer` red; `publish-pypi` and `create-github-release` skipped; no GH Release; no new PyPI version (`pip index versions pushkit` unchanged).

## Risks / Watchouts

- **First-release PyPI OIDC misconfig.** Per shape Block F + slice Risks: top first-release failure mode. Mitigation: verify the Trusted Publisher quartet (owner=`jayteealao`, repo=`PushKit`, workflow=`release.yml`, environment=`pypi`) on the PyPI dashboard before pushing the validation tag.
- **PyPI publish before downstream failure.** All-or-nothing ordering: `publish-pypi` `needs:` all four build jobs. If one build fails, no PyPI push. Triple-checked in step 2h.
- **GH Release race with PyPI.** If `publish-pypi` succeeds but `create-github-release` fails, PyPI has the wheel without a release page. Recovery: manual `gh release create` against the tag, or `pip yank` + `-rc.N+1`. Documented in `## Releasing` README.
- **Windows runner cold start.** ~2–4 min cold-start on `windows-2022` adds to the wall-clock budget (shape NFR ≤ 15 min). The `build-backend-installer` and `post-publish-windows` jobs both pay this. Worth monitoring on the validation tag; acceptable per NFR.
- **git-cliff first-tag handling.** Verified in step 6: `git cliff --tag <tag> --unreleased -o <file>` works for a repo with zero prior tags. The 6 pre-existing non-conventional commits are dropped by `filter_unconventional = true`.
- **`softprops/action-gh-release@v3` is third-party.** Adds one supply-chain surface. Floating major-version pin matches repo style; per discovery, this trade is accepted vs `gh release create`'s no-retry behaviour.
- **`gh release create` upload flake.** Mitigated by switching to `softprops/action-gh-release@v3` (built-in retry); see slice Risk doc.
- **`PYPI_API_TOKEN` rotation forgotten.** Sealed break-glass secret. Document in `## Releasing` that it should be rotated annually or whenever a maintainer leaves. Annual rotation is OUT of this slice's enforcement scope.
- **Concurrency lock.** `concurrency: { group: release-${{ github.ref_name }}, cancel-in-progress: false }` — two simultaneous tag pushes collide and the second queues. Rare; cadence is low.
- **Validation tag pollution.** Pushing `v0.1.0-rc.1` for validation publishes to real PyPI + creates a real GH Release. If validation reveals a bug, recovery is `pip yank` + `-rc.2` (per intake). Accepted per intake.
- **CHANGELOG.md commit lifecycle.** Per ship-plan Block B + commit-hygiene cross-cutting concerns: NOT committed before release. The generate-changelog job produces a per-release artifact embedded in GH Release notes only.
- **`-P` vs `-D` flag confusion (Gradle).** `-DversionCodeOverride=N` is silently invisible to `providers.gradleProperty`. The android-versioning plan flagged this; step 2f uses `-P` exclusively.
- **`fetch-depth: 0` required for three jobs.** `tag-guard` (git merge-base needs history), `build-android-apk` (git rev-list count), `generate-changelog` (cliff needs full history). Easy to miss on a job; if missed, the value silently degrades to `1` and the count comes out as `1` or the changelog is empty. Triple-check on PR review.
- **`actions/upload-artifact@v4` immutability.** Each artifact name must be unique within a workflow run. Names used: `cli-wheel`, `backend-windows-binary`, `windows-installer`, `android-apk`, `changelog`. No collisions.
- **`negrutiu/nsis-install@v2` input names.** Action input may be `version:` rather than `nsis-version:`. Implement stage verifies against the action's current README. The `tech-research-enforcer` skill can be invoked at implement time.
- **OIDC token expiry mid-job.** Per freshness research: tokens are ~15 min lifetime. The `publish-pypi` job is short (download wheel + publish). Risk only if the Linux runner queue depth pushes job start to >15 min after `tag-guard` runs — not actually how OIDC tokens work (they're issued at job start), so this is a misframing; no mitigation needed.
- **shields.io cache TTL.** AC12 allows up to 30 min for badge refresh; this is normal shields.io behaviour and not a workflow bug.
- **README rendering on GitHub.** The shields.io badge image requires HTTPS image fetch; corporate proxies sometimes block. Not a CI concern — only affects local README preview.
- **Tag protection rule is manual.** Not enforced by code. If the maintainer skips this step, a non-maintainer with push access could create `v*` tags. `tag-guard` is the CI backstop but a malicious user could push a tag on a branch they own that happens to be on main — the only durable defense is the GitHub-level tag-protection rule.

## Dependencies on Other Slices

Inbound (this slice consumes):

- **`commit-hygiene`** (verified) — `ci.yml` exists with job names `backend-test`, `cli-test`, `android-build`, `commitlint-backstop`. Conventional Commits enforcement live since first commit of branch. `retest-*` jobs in this slice mirror the first three.
- **`nsis-installer`** (implemented, verify in progress) — `backend/installer/pushkit.nsi` exists with the `File "..\pushkit-server.exe"` directive. The vendored `plugins/SimpleSC.dll` is tracked. NSIS 3.12 minimum.
- **`backend-version`** (verified) — `var Version` + `--version` flag. The `printVersion` helper writes `pushkit-server <v>\n`. `Makefile:13` and existing `release.yml:44` URL fix is landed.
- **`android-versioning`** (planned, ready to implement) — `android/app/build.gradle.kts` accepts `-PversionCodeOverride` / `-PversionNameOverride`. Local defaults preserved. This slice MUST consume after `android-versioning` is implemented.

Outbound: none (this is the final integrator slice).

**Implementation sequencing constraint:** This slice's `build-android-apk` job depends on `android-versioning` being merged to the branch. The other three jobs (`build-backend-binary`/`build-backend-installer`/`build-cli-wheel`) depend on slices already verified. Implementing this slice while `android-versioning` is still in-progress is technically possible but would mean `build-android-apk` fails on dry-run until android-versioning lands. Recommendation: finish android-versioning first.

## Assumptions

- Maintainer's Windows 11 Pro machine has `gh` CLI ≥ 2.0, `git` ≥ 2.40, `python` ≥ 3.12 (for PEP 440 normalization in step 6), Android SDK with `build-tools/34.0.0/aapt.exe` at `%ANDROID_HOME%\build-tools\34.0.0\aapt.exe` (confirmed by android-versioning verify).
- GitHub-hosted runners (`ubuntu-latest`, `windows-2022`) remain available with the current preinstalled toolchain through v0.x. If `ubuntu-latest` switches major (e.g., 24.04 → 26.04) mid-run, the `apt-get install aapt` step may fail — fallback is `actions/setup-android-sdk`.
- `negrutiu/nsis-install@v2` continues to support NSIS 3.12 pinning. If the action is archived/deprecated, fallback is `repolevedavaj/install-nsis@v1` (alternative pin in discovery Round 2) or inline `choco install nsis -y --version 3.12`.
- The 6 pre-existing non-conventional commits stay as-is (forward-only enforcement per shape Round 5). `filter_unconventional = true` in `cliff.toml` drops them from the v0.1.0-rc.1 changelog.
- The PyPI Trusted Publisher quartet was configured at intake and is still valid; the validation tag exercises it.
- `PYPI_API_TOKEN` repository secret exists (sealed); never invoked unless OIDC fails.
- Branch protection / tag protection on GitHub: documented but configured manually by the maintainer; not enforced by code.
- `softprops/action-gh-release@v3` continues to support `body_path`, `prerelease`, `files`, `make_latest`, `fail_on_unmatched_files`. If the action's inputs drift, implement stage re-verifies via tech-research-enforcer.
- `python -c "from packaging.version import Version; print(Version(...))"` is the canonical PEP 440 normalization. Python ≥ 3.10 with `packaging` (transitive dep of `pip`) is sufficient. `actions/setup-python@v5` provides this.
- Sub-agent freshness research from 2026-05-26 is current. Re-verify time-sensitive pins (action versions, NSIS version) at implement time if the implement stage runs more than ~7 days after this plan.

## Blockers

None.

## Freshness Research

Web sub-agent run 2026-05-26. Pinned decisions:

| Tool | Pin | Source | Rationale |
|---|---|---|---|
| `actions/checkout` | `@v4` | [github.com/actions/checkout](https://github.com/actions/checkout/releases) | Matches existing ci.yml + release.yml. v6 exists but bumping creates ci-vs-release drift. |
| `actions/setup-go` | `@v5` | [github.com/actions/setup-go](https://github.com/actions/setup-go/releases) | Matches existing. v6 default cache-key changed; staying on v5 avoids surprise. |
| `actions/setup-java` | `@v4` | [github.com/actions/setup-java](https://github.com/actions/setup-java/releases) | Latest stable major. |
| `actions/setup-python` | `@v5` | Set up Python v5.4.0 (2026-01) — current | Latest stable; matches existing release.yml. |
| `gradle/actions/setup-gradle` | `@v4` | [blog.gradle.org/github-actions-for-gradle-v6](https://blog.gradle.org/github-actions-for-gradle-v6) | Matches existing ci.yml. v6 introduces proprietary cache provider — staying on v4 for OSS hygiene. |
| `actions/upload-artifact` | `@v4` | [github.blog v3 deprecation](https://github.blog/changelog/2024-04-16-deprecation-notice-v3-of-the-artifact-actions/) | v3 hard-deprecated 2025-01-30; v4 mandatory. |
| `actions/download-artifact` | `@v4` | Same | Same. |
| `pypa/gh-action-pypi-publish` | `@release/v1` with `attestations: false` | [github.com/pypa/gh-action-pypi-publish](https://github.com/pypa/gh-action-pypi-publish/releases) | Current latest patch v1.14.0. Attestations OFF for v0.x per discovery Round 2. |
| `orhun/git-cliff-action` | `@v4` | [github.com/orhun/git-cliff-action](https://github.com/orhun/git-cliff-action) | Latest major; bundles cliff v2.5.0. `version: latest` would pull v2.11.0 but bundled is sufficient for our needs. |
| `softprops/action-gh-release` | `@v3` | [github.com/softprops/action-gh-release](https://github.com/softprops/action-gh-release/releases) | v3.0.0 (Node 24 runtime); latest stable. Built-in retry + glob support. |
| `negrutiu/nsis-install` | `@v2` with `nsis-version: "3.12"` | [github.com/marketplace/actions/install-nsis](https://github.com/marketplace/actions/install-nsis) | Most actively maintained NSIS install action 2026-05; supports version pinning. NSIS 3.12 locks CVE-2025-43715 fix. |
| `wagoid/commitlint-github-action` | `@v6` (in retest jobs? — NO) | n/a | `retest-*` jobs do NOT include commitlint backstop — that's `ci.yml`'s job on PRs. Tag-driven retest only re-runs the test jobs. |
| `windows-2022` (runner) | explicit | [Windows2025-Readme](https://github.com/actions/runner-images/blob/main/images/windows/Windows2025-Readme.md) | Pin to avoid `windows-latest` → `windows-2025` drift and the June 2026 VS-2026 migration window. Stable through April 2027 per current image lifecycle. |
| Go cross-compile flags | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0` | [go.dev/wiki/WindowsCrossCompiling](https://go.dev/wiki/WindowsCrossCompiling) | Idiomatic for Go 1.24; produces portable PE32+ binary compatible with NSIS. |
| PyPI PEP 440 normalization | `v0.1.0-rc.1` → `0.1.0rc1` | [peps.python.org/pep-0440](https://peps.python.org/pep-0440/) | The `v` prefix is stripped; hyphen + dot in pre-release segment are normalized. |
| Sigstore attestations | `false` for v0.x | Discovery Round 2 | SLSA hardening out of scope per intake/shape. |
| `git cliff --init keepachangelog` | initial template | [git-cliff.org/docs/usage/examples](https://git-cliff.org/docs/usage/examples/) | Industry-standard Keep-a-Changelog template. Tune `tag_pattern` + scope vocabulary. |

Anti-patterns avoided:

- `tj-actions/changed-files` supply-chain compromise (March 2025, CISA KEV): we use floating major-version tags only on first-party (`actions/*`) and well-known (`pypa/*`, `gradle/*`) actions; the two third-party actions (`negrutiu/nsis-install@v2`, `softprops/action-gh-release@v3`, `orhun/git-cliff-action@v4`) are accepted residual supply-chain risk for v0.x. SHA-pinning is deferred to a future SLSA-hardening workflow per intake.
- `actions/checkout` `persist-credentials` exfiltration: not relevant since this slice does not push commits or run untrusted PR code (only tag-triggered).
- OIDC token expiry: `publish-pypi` is short by design; the four build jobs run before it, and the OIDC token is issued at job-start not workflow-start.

CVEs in past 12 months: NSIS 3.10 has CVE-2025-43715 (already addressed by 3.12 pin); none active for any other pinned dependency.

## Revision History

*(none — first revision)*

## Recommended Next Stage

Current status (2026-05-26):
- `commit-hygiene`: ✅ verified
- `nsis-installer`: ✅ implemented (verify in progress)
- `backend-version`: ✅ verified
- `android-versioning`: ✅ planned → ready to implement
- `release-orchestration`: ✅ planned (this artifact)

- **Option A (default):** `/wf implement ship-plan-buildout android-versioning` — finish the precursor slice first. Two files, ~5 lines of Gradle DSL, no blockers. The integrator (this slice) depends on `android-versioning` being on the branch.
- **Option B:** `/wf implement ship-plan-buildout release-orchestration` — implement this slice directly. Viable only if `android-versioning` is implemented in the same session beforehand. Plan is execution-ready; no blockers.
- **Option C:** `/wf plan ship-plan-buildout all` — review-all mode would re-validate all five plans for cross-cohesion now that the integrator's specifics are pinned (NSIS install action, action pins, post-publish topology). Optional sanity pass.
- **Option D:** `/wf review ship-plan-buildout` — early review pass against the cumulative branch diff (slug-wide review scope) before implementing the remaining slices. Lets reviewer findings shape the final integrator's implementation rather than chasing them post-hoc.
