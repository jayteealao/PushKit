---
schema: sdlc/v1
type: shape
slug: ship-plan-buildout
status: complete
stage-number: 2
created-at: "2026-05-22T22:26:43Z"
updated-at: "2026-05-22T22:26:43Z"
docs-needed: true
docs-types: [reference, how-to, readme]
tags:
  - release-engineering
  - ci-cd
  - github-actions
  - nsis
  - git-cliff
  - lefthook
  - commitlint
  - android-versioning
  - oidc
refs:
  index: 00-index.md
  intake: 01-intake.md
  po-answers: po-answers.md
  research:
    codebase: "task af26ffb3da0e285f6 (codebase architecture survey)"
    external: "task a39868cddea3448b7 (freshness research)"
  next: 03-slice.md
next-command: wf-slice
next-invocation: "/wf slice ship-plan-buildout"
---

# Shape: ship-plan-buildout

## Problem Statement

The project ships nothing today end-to-end. `.github/workflows/release.yml` is a single-job stub that only publishes a Python wheel to PyPI when a `v*` tag is pushed; the backend Windows installer, Android APK, changelog, GitHub Release, and post-publish verifications described in `.ai/ship-plan.md` (Blocks C–G) do not exist. There is also no pre-merge CI: PRs into `main` are not gated by tests, lint, or commit hygiene. Conventional Commits — required by `git-cliff` to generate release notes — are neither enforced nor in use (0 of the last 6 commits parse).

The maintainer wants **tag-and-walk-away** behavior: push `v0.1.0-rc.1` from `main`, get a complete GitHub Release with three artifacts, an auto-generated changelog, and SHA256SUMS, with zero manual steps after the tag push. The current state cannot deliver that.

## Primary Actor / User

- **Repo maintainer (jayteealao)** — produces releases. Wants the tag push to be the only action; CI does everything else.
- **CLI users** — `pip install pushkit` (or `pip install --pre pushkit` for prereleases). Expect a versioned wheel on PyPI within minutes of a tag.
- **Backend operators on Windows** — download `pushkit-server-setup.exe` from a GitHub Release, run silent (`/S`) or interactive install, get a working `pushkit-server.exe --version` returning the tag.
- **Android testers** — sideload `pushkit-android.apk` from a GitHub Release; `aapt dump badging` reports the expected `versionName` and monotonic `versionCode`.

## Desired Behavior

### Pre-merge gate (`.github/workflows/ci.yml`)

On every `pull_request` targeting `main` (and on `push: branches: [main]` as a backstop):

1. `backend-test` — `go vet ./...` + `go test ./...` in `backend/`.
2. `cli-test` — `go vet ./...` + `go test ./...` in `cli/`.
3. `android-build` — Gradle `assembleDebug` + `lint` + `testDebugUnitTest` on Linux.
4. `commitlint-backstop` — `wagoid/commitlint-github-action@v6` validates every commit in the PR against Conventional Commits, defending against `--no-verify` bypass.

All four jobs run in parallel. No release-build artifacts are produced pre-merge (lightweight pre-merge, heavy on tag).

### Tag-driven release (`.github/workflows/release.yml`)

On `push: tags: ['v*']`:

1. `tag-guard` — ensures `$GITHUB_SHA` is reachable from `origin/main` via `git merge-base --is-ancestor`. Exits non-zero (release aborts) if the tag is not on `main`. Belt-and-suspenders: a GitHub branch-protection rule restricts who can push `v*` tags (configured manually by the maintainer; documented in handoff).
2. `retest` — re-runs `backend-test` + `cli-test` + `android-build` from `ci.yml` against the tagged commit. If anything fails, the release is aborted.
3. `build-cli-wheel` — runs `make build-wheels VERSION=<tag-without-v>` on Linux. Produces wheel(s) in `dist/`. Uploads as a build artifact.
4. `build-backend-windows-installer` — on `windows-latest`:
   - Cross-compiles `pushkit-server.exe` from the Linux retest job's artifact OR builds in-place (decision deferred to plan stage — likely Linux cross-compile + Windows-runner NSIS-wrap).
   - Runs `makensis backend/installer/pushkit.nsi` (NSIS 3.10 is pre-installed on `windows-2022`) with the version injected via `/DVERSION=<tag>`. Produces `pushkit-server-setup.exe`.
   - Uploads as a build artifact.
5. `build-android-apk` — on Linux:
   - Computes `versionCode = $(git rev-list --count HEAD)` with full-depth checkout.
   - Rewrites `versionCode` and `versionName` in `android/app/build.gradle.kts` in-place (not committed back).
   - Runs `./gradlew assembleDebug`. Produces `pushkit-android.apk` from `android/app/build/outputs/apk/debug/`.
   - Uploads as a build artifact.
6. `publish-pypi` — depends on `retest`, `build-cli-wheel`, `build-backend-windows-installer`, `build-android-apk` (all-or-nothing — installer/APK failures must abort before PyPI). Downloads the wheel artifact and publishes via `pypa/gh-action-pypi-publish@release/v1` using OIDC Trusted Publishing (no token).
7. `generate-changelog` — depends on `retest`. Installs `git-cliff` (cargo install or pinned GitHub Release binary), runs `git cliff --tag <tag> --unreleased -o CHANGELOG.md` against full history. Outputs the section for this tag.
8. `create-github-release` — depends on every preceding job. Downloads installer + APK + wheel artifacts. Generates `SHA256SUMS` (single file, industry-standard `<hash>  <filename>` format) over the three binary assets. Uses `gh release create <tag> --notes-file <changelog-section> --prerelease=<tag-has-rc> pushkit-server-setup.exe pushkit-android.apk SHA256SUMS`. The wheel is NOT attached to the GitHub Release (PyPI is the canonical location).
9. `post-publish-checks` — single job at the tail:
   - `fresh-resolve`: `pip install --no-cache-dir --pre pushkit==<normalized-version> && pushkit --version` in a fresh venv.
   - `github-release`: `gh api repos/jayteealao/PushKit/releases/tags/<tag>` returns 200 with expected assets.
   - `smoke-test`: silent-install `pushkit-server-setup.exe /S` on a fresh `windows-latest`, then `pushkit-server.exe --version` returns the tag.
   - `apk-probe`: `aapt dump badging pushkit-android.apk` reports the expected `versionName` and `versionCode`.

### Commit-message enforcement (developer machines + CI)

- `lefthook.yml` checked in at repo root. `lefthook install` is documented as a one-time per-clone setup command in the README and in `08-handoff.md`.
- `commit-msg` hook delegates to `@commitlint/cli` config in `commitlint.config.cjs` (extends `@commitlint/config-conventional`).
- CI backstop (`wagoid/commitlint-github-action@v6`) catches `--no-verify` bypasses on PRs — fails the PR if any commit doesn't parse.

### Backend version flag

- Add `var Version = "dev"` to `backend/cmd/server/main.go` (mirroring `cli/main.go:11`).
- Add `--version` flag (and `-v` alias) that prints `pushkit-server <version>` and exits 0. Flag is the first thing handled in `main()` so it runs before any DB/S3 init.
- CI build uses `-ldflags "-X main.Version=<tag>"` to inject the tag.

### Android version injection

- CI computes `versionCode = git rev-list --count HEAD` and `versionName = <tag-without-v>` (e.g., `0.1.0-rc.1`).
- A small inline `sed` (or Gradle property) rewrites the two lines in `android/app/build.gradle.kts` before `./gradlew assembleDebug`. The rewrite is NOT committed back.

### NSIS installer (`backend/installer/pushkit.nsi`)

Full-lifecycle installer with optional Windows service component:

- Default components: install `pushkit-server.exe` to `%ProgramFiles%\PushKit\`, add Start Menu shortcut, register uninstaller (visible in Apps & Features / Programs and Features), embed version metadata.
- Optional component (unticked by default): register Windows service `PushKitServer` via `sc.exe create PushKitServer binPath= "%ProgramFiles%\PushKit\pushkit-server.exe" start= demand`. Requires UAC elevation.
- Upgrade path: installer detects existing service via `sc query PushKitServer`. If present and running, stops it before file replace and restarts after. If user opted out of the service component in a prior install, this branch is skipped.
- Uninstaller: stops the service if running, deregisters it (`sc delete PushKitServer`), removes files, removes Start Menu shortcut, deregisters uninstaller.
- Silent install: `pushkit-server-setup.exe /S` installs default components (no service). Documented in the `## Installer` section of README.
- No code-signing (out of scope for v0.x — accepts SmartScreen warning).

### Cosmetic URL fix

- `Makefile`: change `--url "https://github.com/pushkit/cli"` → `https://github.com/jayteealao/PushKit`.
- `.github/workflows/release.yml`: grep for any `pushkit/cli` and fix.
- Go module paths (`github.com/pushkit/{cli,backend}`) are NOT renamed (confirmed out of scope).

## Acceptance Criteria

Listed Given/When/Then with verification classification.

### AC1 — Pre-merge CI gates a PR

- **Given** a PR is opened against `main` containing a commit that fails `go vet` in `backend/`,
  **When** GitHub Actions runs `ci.yml`,
  **Then** the `backend-test` job fails red and PR status checks show `ci / backend-test (failed)`.
- **Verification:** `automated` — verified by intentionally pushing a syntax error in a throwaway PR (or via dry-run in this workflow's slicing).

### AC2 — Conventional Commits enforced in CI

- **Given** a PR contains a commit with message `WIP: hack stuff`,
  **When** GitHub Actions runs `ci.yml`,
  **Then** the `commitlint-backstop` job fails red.
- **Verification:** `automated`.

### AC3 — Local commit-msg hook rejects non-conventional commits

- **Given** a clean clone with `lefthook install` run,
  **When** the developer attempts `git commit -m "hack stuff"`,
  **Then** the commit is rejected with a clear error citing the conventional-commits spec.
- **Verification:** `interactive` — Tool: Git CLI on developer machine. Method: clone fresh, run `lefthook install`, attempt a non-conventional commit. Evidence capture: terminal output / screenshot of rejection.

### AC4 — Tag from a non-main commit aborts the release

- **Given** a tag `v0.0.0-test.1` is pushed pointing at a commit that is not reachable from `origin/main`,
  **When** `release.yml` fires,
  **Then** the `tag-guard` job exits non-zero before any artifact is built or published.
- **Verification:** `automated` — verifiable in a throwaway branch during this workflow's verify stage.

### AC5 — Tag on main produces all three artifacts

- **Given** the workflow is fully merged into `main` and a tag `v0.1.0-rc.1` is pushed from `main`'s HEAD,
  **When** `release.yml` runs to completion,
  **Then** the run succeeds end-to-end and the GitHub Release `v0.1.0-rc.1` exists with assets `pushkit-server-setup.exe`, `pushkit-android.apk`, `SHA256SUMS`, plus auto-generated release notes from `git-cliff`. The release is marked as prerelease (`-rc.*` heuristic).
- **Verification:** `interactive` — Tool: `gh` CLI + browser. Method: `gh release view v0.1.0-rc.1 --repo jayteealao/PushKit` lists the three assets; web UI shows the release marked Pre-release. Evidence capture: terminal output + screenshot of the GitHub Release page.

### AC6 — Wheel is on PyPI and installable

- **Given** the release workflow completes successfully,
  **When** a fresh `windows-latest` runner executes `pip install --no-cache-dir --pre pushkit==0.1.0rc1 && pushkit --version`,
  **Then** the command returns `pushkit version 0.1.0-rc.1` (or the PEP-440-normalized equivalent the CLI version-string library produces).
- **Verification:** `automated` — runs as the `fresh-resolve` step in the post-publish-checks job.

### AC7 — Installer silent install works on Windows

- **Given** the GitHub Release exists with `pushkit-server-setup.exe`,
  **When** a fresh `windows-latest` runner downloads the asset and runs `pushkit-server-setup.exe /S` followed by `"%ProgramFiles%\PushKit\pushkit-server.exe" --version`,
  **Then** the binary reports `pushkit-server 0.1.0-rc.1`.
- **Verification:** `automated` (via post-publish-checks `smoke-test` job) but ALSO `interactive` once on the maintainer's local Windows machine for the optional-service component path. Tool: PowerShell on Windows 11. Method: interactive install with "Register Windows Service" component ticked, verify `sc query PushKitServer` returns `STATE: 1 STOPPED`, verify uninstall via Apps & Features removes the service. Evidence capture: screen recording or sequence of screenshots.

### AC8 — APK reports expected version metadata

- **Given** the GitHub Release exists with `pushkit-android.apk`,
  **When** `aapt dump badging pushkit-android.apk` is run (locally or in CI),
  **Then** output contains `versionName='0.1.0-rc.1'` and `versionCode='<git-rev-list-count-at-tag>'` where the latter is monotonically greater than any prior release's versionCode.
- **Verification:** `automated` — runs as the `apk-probe` step in post-publish-checks.

### AC9 — Changelog is non-empty and references the tag

- **Given** the release workflow completes,
  **When** the release notes are read from the GitHub Release page,
  **Then** the notes contain a `## v0.1.0-rc.1` (or similar) heading and at least one categorized commit entry (e.g., `### Features` with one bullet).
- **Verification:** `manual` — human reads the GitHub Release notes. (Automated would require parsing the notes' structure, which is brittle vs git-cliff template changes.)

### AC10 — SHA256SUMS verifies

- **Given** the GitHub Release has assets + `SHA256SUMS`,
  **When** a user downloads all assets and runs `sha256sum -c SHA256SUMS` (Linux/git-bash),
  **Then** every line reports OK.
- **Verification:** `automated` — runs as a tail step in post-publish-checks.

### AC11 — Backend `pushkit-server --version` works after `go build`

- **Given** the backend has `var Version` and a `--version` flag implemented,
  **When** built locally via `go build -ldflags "-X main.Version=v0.1.0-rc.1"` and run with `--version`,
  **Then** the binary prints `pushkit-server 0.1.0-rc.1` to stdout and exits 0.
- **Verification:** `automated` — backend unit test exercises the flag handler.

### AC12 — README badge resolves

- **Given** v0.1.0-rc.1 is released,
  **When** the README is viewed on github.com (or rendered locally),
  **Then** the shields.io latest-release badge (with `include_prereleases=true`) renders the string `v0.1.0-rc.1` within 30 minutes of release publish.
- **Verification:** `manual` — human checks the README on GitHub after release. (Cache-bound; not deterministic enough for automation.)

### AC13 — Single artifact failure aborts the release

- **Given** a tag is pushed with a deliberate NSIS build error (e.g., reference to a missing file in `pushkit.nsi`),
  **When** `release.yml` runs,
  **Then** `build-backend-windows-installer` fails red, `publish-pypi` is NOT executed (gated by job graph), and `create-github-release` is NOT executed. PyPI receives nothing. No GitHub Release is created.
- **Verification:** `interactive` — Tool: GitHub Actions UI + a throwaway tag during this workflow's verify stage. Method: introduce a deliberate NSIS error on a feature branch, tag from it (knowing it'll fail), watch the Actions UI confirm the abort, then revert. Evidence capture: screenshot of Actions run graph showing red NSIS job + skipped downstream jobs.

## Non-Functional Requirements

- **Pipeline latency (release):** Tag push → GitHub Release visible ≤ 15 minutes for v0.1.0-rc.1. (Heavy steps: Windows runner spin-up, Android Gradle build, NSIS, PyPI publish, post-publish checks.)
- **Pre-merge CI latency:** PR push → all four CI jobs complete ≤ 8 minutes p95. Heavy job is `android-build` (Gradle cold cache); cache via `actions/cache` for `~/.gradle` and `android/.gradle`.
- **Idempotency:** Re-running `release.yml` on the same tag is undefined / not supported for v0.x — once a tag is published, it is. Cadence is on-demand and low-frequency, so no special-case logic for re-runs.
- **Concurrency:** GitHub Actions `concurrency: { group: release-<tag>, cancel-in-progress: false }` on the release workflow prevents two simultaneous runs of the same tag (rare but possible if a maintainer races).
- **Footprint:** Installer ≤ 25 MB. APK ≤ 30 MB (debug-signed, no R8 minification expected in debug build). Wheel ≤ 15 MB.
- **Audit:** Zero long-lived secrets in CI; one sealed `PYPI_API_TOKEN` break-glass secret as backup. All other auth is OIDC + auto-provided `GITHUB_TOKEN`.

## Edge Cases / Failure Modes

- **First release with thin commit history.** 0 of last 6 commits are conventional. `git-cliff --unreleased` produces a changelog from whatever it can parse. Expectation: changelog for v0.1.0-rc.1 will be sparse — primarily the conventional commits added in this workflow's PR. Either accept that or hand-author a `## v0.1.0-rc.1` overlay section. Forward-only enforcement decided in Round 2.
- **PyPI Trusted Publisher misconfigured.** First-release failure mode #1. Diagnosis playbook is in `.ai/ship-plan.md` Block F (`pypi-trusted-publish-failure`). Backup: the sealed `PYPI_API_TOKEN` secret + a documented break-glass path that swaps the OIDC publish for `twine upload --repository pypi` for one release.
- **NSIS build flake on `windows-2022`.** Pre-installed NSIS version drift (3.10 today; the `windows-2025` image may not yet have it). Mitigation: pin `runs-on: windows-2022` explicitly until shape-stage research confirms `windows-latest` and `windows-2025` are equivalent. Fallback: install via `choco install nsis -y --version 3.10` if `makensis` is not in PATH.
- **Shallow clone breaks Android versionCode.** `actions/checkout@v4` default is `fetch-depth: 1`, which breaks `git rev-list --count HEAD`. Job must set `fetch-depth: 0`.
- **versionCode collision after rebase.** If history is ever rewritten (e.g., a maintainer force-pushes main), `git rev-list --count HEAD` can produce a value lower than a previously-released APK's versionCode, blocking install on devices with the prior version. Mitigation: the maintainer is the only pusher and is aware (documented in handoff README).
- **Tag pushed twice (e.g., `git push --tags --force` after a fix).** GitHub Actions runs the workflow again. Second run may collide with an existing GitHub Release. `gh release create` will fail on duplicate tag; the release.yml does not handle this — accept fail-loud and rely on `pip yank + new -rc.N` recovery path.
- **`--no-verify` bypass on local commits.** Lefthook is bypassable. CI backstop in `ci.yml` is the durable defense; commits that don't parse fail the PR.
- **Windows runner cold start adds minutes.** Windows runners spin up slower than Linux. Mitigation: run all Linux jobs (retest, wheel build, APK build, changelog) in parallel with the Windows installer job so total wall time is gated by the slowest, not the sum.
- **OIDC clock skew.** PyPI Trusted Publishing tokens are short-lived. If the runner's clock is off (rare on hosted runners), publish fails with a confusing error. Mitigation: documented in the `pypi-trusted-publish-failure` playbook.
- **Service install on Windows requires UAC.** Optional Windows service component in NSIS requires elevation. If `/S` silent install is run without admin rights, the service component must skip gracefully (not abort the install). Implementation: `UserInfo::GetAccountType` check before `sc.exe create`.
- **Existing service on upgrade.** If a prior install left `PushKitServer` registered and running, the new installer must stop, replace, restart. NSIS `Function .onInit` checks `sc query` and branches accordingly.
- **APK install on a device that has a higher versionCode.** A user who manually built an APK with a high versionCode and installed it cannot downgrade. Out of scope — not a CI concern.
- **PEP-440 prerelease normalization confusion.** `v0.1.0-rc.1` git tag → `0.1.0rc1` PyPI version. The acceptance test uses `pip install --pre pushkit==0.1.0rc1` (not `0.1.0-rc.1`). Documented in `## Installer` and `## Releasing` sections of README.

## Affected Areas

### Existing files that will be modified

- `.github/workflows/release.yml` — expanded from 1 job to ~8 jobs.
- `Makefile` — URL fix; potentially add `build-android-apk` and `build-windows-installer` targets for local dev parity.
- `backend/cmd/server/main.go` — add `var Version = "dev"` + `--version`/`-v` flag.
- `android/app/build.gradle.kts` — no structural change; CI rewrites `versionCode`/`versionName` lines in-place during build (lines 15–16).
- `README.md` — add shields.io release badge; add `## Releasing` section documenting the tag-and-walk-away flow; add `## Backend installer` section with the silent-install command and the optional-service component note.

### New files

- `.github/workflows/ci.yml` — pre-merge gate (4 jobs).
- `backend/installer/pushkit.nsi` — full-lifecycle NSIS script with optional Windows service component.
- `backend/installer/README.md` — short reference for installer authors covering the `/DVERSION=` symbol, the silent-install flag, and the optional-service component switch.
- `cliff.toml` — git-cliff configuration (conventional commits parsing, Keep-a-Changelog output format, version pattern matching).
- `lefthook.yml` — commit-msg hook delegating to commitlint.
- `commitlint.config.cjs` — extends `@commitlint/config-conventional` (Note: introduces a Node toolchain footprint just for commitlint config. Acceptable per Round 3 decision — lefthook chosen, but commitlint itself is the parser).
- `CHANGELOG.md` — first version generated by git-cliff at v0.1.0-rc.1 build time; not committed before that point (per ship-plan Block B).

### Files NOT touched

- `backend/Dockerfile`, `backend/docker-compose.yml` (local-dev only; out of scope per intake).
- `backend/internal/**` (no business-logic change).
- `cli/cmd/**` (already has `--version` plumbing via cobra).
- Go `go.mod` module paths (rename out of scope, only `--url` cosmetic fix in Makefile).
- Android signing config (debug-only stays).

## Dependencies / Sequencing Notes

External binaries / actions to introduce:

- `pypa/gh-action-pypi-publish@release/v1` — already in use; confirmed current via freshness research.
- `actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-python@v5` — already in use.
- `actions/setup-java@v4` (Temurin 17), `gradle/actions/setup-gradle@v4` — to add for the Android job.
- `wagoid/commitlint-github-action@v6` — to add for the commitlint backstop.
- `evilmartians/lefthook@v2.1.8` binary installed locally; install command documented in README.
- `git-cliff` binary — pinned to a specific version (likely `orhun/git-cliff-action@v3` to avoid `cargo install` time in CI).
- `softprops/action-gh-release@v3` OR `gh release create` (decision deferred to plan stage — `gh release create` is built-in and slightly simpler; `softprops` provides nicer asset-glob semantics).

Internal ordering:

- `tag-guard` is the first job in `release.yml`. Everything depends on it.
- `retest` depends on `tag-guard`. `build-cli-wheel`, `build-backend-windows-installer`, `build-android-apk`, `generate-changelog` all depend on `retest`.
- `publish-pypi` depends on all build jobs (all-or-nothing per Round 1).
- `create-github-release` depends on every preceding job. Last to run.
- `post-publish-checks` depends on `create-github-release` AND `publish-pypi`.

Cross-cutting: every Go job needs full-depth checkout (`fetch-depth: 0`) for versionCode derivation and git-cliff history. Cache `~/.gradle` and `~/go/pkg/mod` to keep pre-merge CI ≤ 8 min.

## Questions Asked This Stage

5 rounds of 4 questions each (20 total) via AskUserQuestion, plus 2 clarifying follow-ups on out-of-scope items.

Round 1 (what does the feature do): partial-failure behavior, tag-on-main enforcement, pre-merge depth, user outcome.
Round 2 (how does it behave): backend version variable wiring, conventional-commit history backfill, Android versionCode source, PyPI rollback path.
Round 3 (what does it look like): CI file layout, hook runner, NSIS scope, SHA256SUMS format.
Round 4 (what can go wrong): service-install lifecycle depth, bypass defense, secrets surface, URL fix scope.
Round 5 (where are the boundaries): out-of-scope confirmation, cadence, README badge form, commit backfill final answer.
Clarifications: Android release signing scope, Go module path renames scope.

## Answers Captured This Stage

Locked decisions (all 22 answers appended to `po-answers.md`):

- All-or-nothing artifact failure (one failed build → no release).
- Tag-on-main: job-level guard + GitHub branch-protection rule.
- Pre-merge lightweight (test+lint+commitlint only); heavy builds on tag.
- Tag-and-walk-away user outcome (zero post-tag manual steps).
- Backend: add `var Version` + `--version` flag (ldflags injection).
- Conventional Commits: forward-only enforcement; thin v0.1.0-rc.1 changelog accepted.
- Android `versionCode = git rev-list --count HEAD` (full-depth checkout required).
- PyPI rollback: yank + new -rc patch tag.
- CI file layout: separate `ci.yml` + `release.yml`.
- Hook runner: lefthook + commitlint.config.cjs.
- NSIS scope: full installer with optional Windows service component (full lifecycle automation — start/stop/replace/restart on upgrade, deregister on uninstall).
- SHA256SUMS: single file for all release assets.
- Service lifecycle: full automation (Round 4 escalated from Round 3's "minimal").
- Commitlint CI backstop: enforced, fails PR on non-conventional commits.
- Secrets: OIDC + sealed backup `PYPI_API_TOKEN` (break-glass only).
- URL fix scope: Makefile + release.yml; Go module renames stay out.
- Cadence: on-demand, low frequency; no idempotency safeguards.
- README badge: shields.io with `include_prereleases=true`.
- Backfill: no — forward-only, accept thin first changelog.
- Android release signing: stays out (debug-only for v0.x).
- Go module path renames: stays out.

## Out of Scope

- Android release signing (`signingConfigs.release`). v0.1.0-rc.1 APK is debug-signed. Deferred to a future workflow that introduces a keystore + signing-secret rotation.
- Backend DB migration tooling. Raw SQLite continues.
- Windows code-signing of the installer. No certificate. SmartScreen warning is accepted for v0.x.
- Go module path renames (`github.com/pushkit/cli` → `github.com/jayteealao/PushKit/cli`). Cosmetic URL fix in `Makefile` + `release.yml` only.
- macOS / Linux backend installers. Windows-only per ship-plan Block A.
- Container image publishing to GHCR for the backend.
- SLSA provenance, sigstore signing. Future supply-chain hardening.
- Sentry / OpenTelemetry instrumentation.
- Concurrent / parallel release of multiple tags (e.g., a hotfix and a feature tag racing). Cadence is low and on-demand; not designed for.
- Re-running a successful release. Once a tag is published, it's final. Recovery is via `-rc.N+1`.

## Definition of Done

This shape becomes "done" when:

1. **Pre-merge CI** (`ci.yml`) is green on the PR that lands this workflow's changes, exercising all four jobs (backend-test, cli-test, android-build, commitlint-backstop).
2. **Release workflow** (`release.yml`) runs end-to-end on a `v0.1.0-rc.1` tag push and produces a GitHub Release with the three required assets + `SHA256SUMS` + git-cliff-generated notes, with the release marked Pre-release.
3. **PyPI** has `pushkit==0.1.0rc1` (PEP-440-normalized) installable via `pip install --pre pushkit==0.1.0rc1`.
4. **Installer smoke** passes on a fresh `windows-latest` runner: `pushkit-server-setup.exe /S` then `--version` returns `0.1.0-rc.1`.
5. **APK probe** confirms `versionName='0.1.0-rc.1'` and a monotonic `versionCode` matching `git rev-list --count HEAD` at the tagged commit.
6. **README badge** resolves to `v0.1.0-rc.1` within 30 minutes of release publish.
7. **Commit-msg hook** rejects a non-conventional commit on a freshly-cloned dev machine after `lefthook install`.
8. **Cosmetic URL fix** is live: `Makefile` and `release.yml` no longer reference `github.com/pushkit/cli`.

## Verification Strategy

### Automated checks (CI/test suite can run these)

- `go vet ./...` + `go test ./...` for backend and CLI — runs in `ci.yml` and in `retest` job of `release.yml`.
- `./gradlew assembleDebug` + `lint` + `testDebugUnitTest` — runs in `ci.yml`.
- `wagoid/commitlint-github-action@v6` — runs in `ci.yml`.
- Backend unit test for `--version` flag — to be added in implement stage, alongside the flag handler.
- Post-publish checks job in `release.yml`: `fresh-resolve` (PyPI), `github-release` (assets present), `smoke-test` (silent install + --version), `apk-probe` (aapt output), SHA256SUMS verify.

### Interactive verification (requires running app/tools and observing behavior)

- **AC3** (local commit-msg hook rejection):
  - Platform: developer machine (Windows / macOS / Linux).
  - Tool: Git CLI + `lefthook` (installed via `lefthook install` after fresh clone).
  - What to verify: `git commit -m "WIP hack"` is rejected with an error citing Conventional Commits.
  - Evidence capture: terminal output / screenshot.

- **AC5** (GitHub Release visible end-to-end):
  - Platform: any.
  - Tool: `gh` CLI + browser.
  - What to verify: `gh release view v0.1.0-rc.1 --repo jayteealao/PushKit` lists three binary assets + SHA256SUMS + non-empty notes; web UI shows Pre-release flag.
  - Evidence capture: terminal output + screenshot of Releases page.

- **AC7** (optional Windows service component path):
  - Platform: maintainer's Windows machine (real interactive install, NOT CI).
  - Tool: NSIS-built installer + PowerShell + `sc.exe`.
  - What to verify: interactive install with service component ticked → `sc query PushKitServer` shows the service. Upgrade install while service is running → service is stopped, file replaced, service restarted. Uninstall → service is stopped and deregistered, no orphan registry keys.
  - Evidence capture: PowerShell terminal log + `Apps & Features` screenshot showing no residue post-uninstall.

- **AC13** (single-failure abort):
  - Platform: any with `gh` CLI.
  - Tool: GitHub Actions UI.
  - What to verify: a throwaway tag with a broken NSIS script aborts the release; PyPI is unchanged; no GitHub Release created.
  - Evidence capture: screenshot of Actions run graph (red NSIS, skipped downstream).

### Human-in-the-loop checks (require human judgement)

- **AC9** (changelog non-empty and references the tag) — human reads the generated CHANGELOG section. Brittle to automate; readable in seconds by a human.
- **AC12** (README badge resolves to v0.1.0-rc.1) — human refreshes README on github.com ~30 min post-release.

No purely interactive flows are blocked behind privileged credentials; the maintainer's Windows machine is sufficient for AC7's full lifecycle test.

## Documentation Plan

Using Diátaxis classification:

### `reference` — Releasing reference (target: `README.md` new section `## Releasing`)

- **Audience:** maintainer (jayteealao); secondary: any future co-maintainer.
- **Must cover:**
  - Tagging conventions: `vX.Y.Z` for stable, `vX.Y.Z-rc.N` for prereleases.
  - The single command: `git tag -a v0.1.0-rc.1 -m "" && git push --tags`.
  - What CI does in order (one-line summary per job).
  - Where to look on failure (Actions UI, post-publish-checks job names).
  - Rollback procedure: `pip yank` + new -rc tag.
  - Required GitHub repo settings: PyPI Trusted Publisher quartet, branch-protection rules for `v*` tags.
- **Must NOT cover (boundary):** internal git-cliff config tuning, NSIS authoring, lefthook install details (those have their own homes).

### `how-to` — Backend installer guide (target: `README.md` new section `## Backend installer`)

- **Audience:** Windows operators (downstream).
- **Must cover:**
  - Where to download the installer (link to GitHub Releases).
  - Silent-install flag (`/S`) for unattended scenarios.
  - Optional Windows service component: when to enable, how to start/stop/uninstall.
  - SmartScreen warning explanation (no code-signing for v0.x).
- **Must NOT cover (boundary):** PyPI CLI, Android app, backend API documentation.

### `reference` — Installer authoring reference (target: `backend/installer/README.md`)

- **Audience:** future installer maintainers.
- **Must cover:**
  - Required `/D` defines (`VERSION`).
  - Component model.
  - How upgrade path detects existing service.
  - Local-dev: how to run `makensis` on a Windows machine to validate changes pre-CI.
- **Must NOT cover (boundary):** anything user-facing.

### `readme-update` — Top-of-README cosmetic refresh

- **Audience:** anyone landing on the repo.
- **Must cover:**
  - shields.io release badge with `include_prereleases=true`.
  - One-line pointer to `## Releasing` and `## Backend installer`.
- **Must NOT cover (boundary):** detailed release procedure (that's in `## Releasing`).

No `tutorial` or `explanation` docs needed for v0.x — the release contract is straightforward enough that reference + how-to suffice.

## Freshness Research

### Source: PyPI Trusted Publisher documentation (docs.pypi.org/trusted-publishers/)

**Why it matters:** First-release failure mode #1 per ship-plan Block F.

**Takeaway:** OIDC publishing via `pypa/gh-action-pypi-publish@release/v1` requires `permissions: id-token: write` (no token). The action is Linux-only and Docker-based — must build wheels separately and pass via `actions/upload-artifact`. The `environment:` field on the job (e.g., `environment: pypi`) is REQUIRED by PyPI's trusted-publisher record matching, even though the action itself doesn't enforce it. v1.11.0+ also produces Sigstore attestations (PEP 740) by default — non-breaking; can be left on or explicitly disabled with `attestations: false` if undesired for v0.x.

### Source: actions/runner-images Windows2022-Readme.md

**Why it matters:** Decides whether to `choco install nsis` (adds ~30s) or rely on pre-installed `makensis`.

**Takeaway:** NSIS 3.10 is pre-installed on `windows-2022` runners. `windows-2025` migration as of May 2026 has PR #11755 pending to add NSIS 3.11; may not be the default yet. Decision: pin `runs-on: windows-2022` explicitly until `windows-latest` (which can be either at any time) is confirmed stable for NSIS. Documented in plan-stage notes.

### Source: git-cliff.org/docs/usage/bump-version + GitHub Releases

**Why it matters:** Required for the auto-generated changelog. Edge case: first tag with no prior tag.

**Takeaway:** git-cliff 2.x supports `--bump` (derives next semver from conventional commits) and `--bumped-version` (outputs the calculated string). For the first release with no prior tag, use `git cliff --unreleased --tag <tag>` to include all commits since repo init. The Keep-a-Changelog template (`changelog-keepachangelog.md`) ships with the project and matches the README badge expectation. Use `orhun/git-cliff-action@v3` to skip `cargo install` time (~2 min saved per CI run).

### Source: lefthook.dev + evilmartians/lefthook

**Why it matters:** Hook runner chosen per Round 3.

**Takeaway:** Lefthook v2.1.8 ships a single Go binary, language-agnostic. The `commit-msg` hook accepts `commit_msg_file: true` and `commands.commitlint.run: npx --no -- commitlint --edit {1}` to delegate to `@commitlint/cli`. This pulls in a small Node dependency tree just for commitlint; acceptable. Alternative for zero-Node: vendored shell `commit-msg` script reading `git rev-parse --verify HEAD` and grepping for `^(feat|fix|chore|docs|...)(\(.+\))?:` — feasible but produces clunky error output. Decision per Round 3: lefthook + commitlint.

### Source: pypa/gh-action-pypi-publish (GitHub)

**Why it matters:** Pinning policy.

**Takeaway:** `release/v1` is the floating major-version branch; latest tagged release in that branch as of May 2026 is v1.11.0+. The project recommends pinning to `release/v1` (rolling) for OIDC users; if SHA-pinning is preferred for supply-chain hygiene, use `pypa/gh-action-pypi-publish@<sha>` and re-pin on each minor. Decision: stay on `release/v1` for v0.x; consider SHA-pinning when SLSA hardening is in scope.

### Source: PEP 440 + sethmlarson.dev/pep-440

**Why it matters:** Prerelease normalization for `pip install` syntax.

**Takeaway:** Git tag `v0.1.0-rc.1` → wheel filename version segment `0.1.0rc1` (hyphens and dots in pre-release segment stripped). Both `pip install pushkit==0.1.0rc1` and `pip install pushkit==0.1.0-rc.1` resolve to the same release (pip normalizes the user input). For `pip install pushkit` (no version pin) to pick up prereleases, the `--pre` flag is required. Documented in README.

### Source: docs.github.com/en/actions/security-guides/automatic-token-authentication

**Why it matters:** `GITHUB_TOKEN` scopes for `gh release create`.

**Takeaway:** `GITHUB_TOKEN` with `contents: write` permission is sufficient to create releases and upload assets. No PAT required. Default `GITHUB_TOKEN` scopes have been tightening; explicit `permissions:` block at workflow level avoids surprises.

### Source: shields.io/badges/git-hub-release

**Why it matters:** README badge configuration.

**Takeaway:** URL shape is `https://img.shields.io/github/v/release/<owner>/<repo>?include_prereleases=true&sort=semver`. Cache TTL is 5–30 minutes after a GitHub release publish event. Markdown: `[![Latest release](https://img.shields.io/github/v/release/jayteealao/PushKit?include_prereleases=true&sort=semver)](https://github.com/jayteealao/PushKit/releases/latest)`. Sort=semver matters with prereleases — without it the badge may pick lexicographically larger tags.

## Recommended Next Stage

- **Option A (default): `/wf slice ship-plan-buildout`** — multi-area scope (backend Go code, Android Gradle config, NSIS authoring, multiple CI workflows, hook tooling, README updates). Strongly benefits from incremental delivery. Natural slices: (1) Conventional Commits + lefthook + pre-merge ci.yml, (2) backend `--version` flag + cosmetic URL fixes, (3) NSIS installer authoring, (4) Android versionCode/versionName injection in release.yml, (5) release.yml expansion + git-cliff + GitHub Release + post-publish checks. Five slices is a reasonable upper bound; some pairs may collapse during slicing.

- **Option B: `/wf plan ship-plan-buildout`** — only viable if the entire scope is treated as one delivery unit. Not recommended given the heterogeneity (Go code + Gradle + NSIS + YAML across two workflows) and the appetite ("large" per intake).

- **Option C: `/wf intake ship-plan-buildout`** — not needed. Intake answers + shape clarifications cover the question space.

- **Option D: Blocked — re-run shape** — not applicable. All 20+2 questions answered; no awaiting-input fields.
