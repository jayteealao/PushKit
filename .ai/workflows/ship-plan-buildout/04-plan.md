---
schema: sdlc/v1
type: plan-index
slug: ship-plan-buildout
status: complete
stage-number: 4
created-at: "2026-05-22T23:44:29Z"
updated-at: "2026-05-26T18:25:15Z"
planning-mode: single
slices-planned: 5
slices-total: 5
implementation-order:
  - commit-hygiene
  - nsis-installer
  - backend-version
  - android-versioning
  - release-orchestration
conflicts-found: 0
tags:
  - ci
  - lefthook
  - commitlint
  - release
  - github-actions
  - nsis
  - git-cliff
  - pypi
refs:
  index: 00-index.md
  slice-index: 03-slice.md
next-command: wf-implement
next-invocation: "/wf implement ship-plan-buildout android-versioning"
---

# Plan Index: ship-plan-buildout

## Slice Plan Summaries

### `commit-hygiene` (planned)

- **Files to touch:** 6 new (`.gitignore`, `package.json`, `package-lock.json`, `commitlint.config.cjs`, `lefthook.yml`, `.github/workflows/ci.yml`) + 1 modified (`README.md`).
- **Strategy:** Two-track. Local enforcement via lefthook + commitlint installed by `npm i`'s `prepare` script. CI backstop via `wagoid/commitlint-github-action@v6` in a 4-job parallel workflow alongside backend-test, cli-test, android-build.
- **Key decisions locked (plan-stage discovery):**
  - Commitlint v21 (ESM, Node ≥ 20).
  - Scope-enum `[backend, cli, android, ci, docs, deps, installer, release]` at warning severity.
  - Single Go 1.24 across both Go jobs (cli/go.mod stays declaring 1.22.0; compatible).
  - Action pins by floating tag (matches existing `release.yml`).
  - `prepare: lefthook install` script auto-installs hooks on `npm i`.
  - Test invocations inlined per job (no new Makefile target).
- **Key risk:** Gradle cold-cache duration could push p95 close to the 8-minute NFR ceiling on the first run; `setup-gradle@v4` built-in cache should bring subsequent runs well below.

### `nsis-installer` (planned 2026-05-25)

- **Files to touch:** 3 new tracked (`backend/installer/pushkit.nsi`, `backend/installer/plugins/SimpleSC.dll`, `backend/installer/README.md`) + 1 modified (root `.gitignore`).
- **Strategy:** Single-track MUI2 installer (~250 lines NSIS) with required + default-checked optional service components. Vendor `SimpleSC.dll` (Unicode 1.30) for reliable service stop-with-file-release on upgrade. `RequestExecutionLevel admin` + belt-and-suspenders `UserInfo::GetAccountType` in `.onInit`. `SetCompressor /SOLID lzma`. Install dir pinned to `$PROGRAMFILES64\PushKit` (no MUI directory page). Apps-&-Features registry written with `SetRegView 64`; uninstall deletes files by name (no `RMDir /r`).
- **Key decisions locked (plan-stage discovery):**
  - NSIS **3.12** minimum (CI + local). NSIS 3.10 carries CVE-2025-43715 (SYSTEM privilege escalation, CVSS 8.1). Adds a CI install step to `release-orchestration`'s scope.
  - `SimpleSC.dll` Unicode 1.30 vendored under `backend/installer/plugins/`. Raw `sc.exe` rejected (async; race condition on upgrade).
  - Output path: `backend/installer/pushkit-server-setup.exe` (co-located with `.nsi`).
  - `SetCompressor lzma`; accept resulting size (Go binary alone is 27.2 MB > shape NFR of 25 MB; LZMA expected to bring total to ~17–20 MB).
  - Non-admin `/S`: fail loudly (MessageBox /SD IDOK + Quit), non-zero exit.
  - Service component default-CHECKED (diverges from slice AC3 — flagged blocker; slice doc to be updated alongside this plan).
  - Install dir pinned (no DIRECTORY page); user cannot relocate.
  - Local iteration: README documents `go build -o backend/pushkit-server.exe` before `makensis`.
- **Key risk:** Slice AC3 contradiction — silent install now registers the service (was "silent skips service component" per spec). Resolution is a single-line edit to `03-slice-nsis-installer.md`. The plan otherwise has zero hard blockers.

### `backend-version` (planned 2026-05-26)

- **Files to touch:** 4 (`backend/cmd/server/main.go`, `backend/cmd/server/main_test.go` (new), `Makefile`, `.github/workflows/release.yml`).
- **Strategy:** Add `var Version = "dev"` + `flag.Bool("version", ...)` check at the top of `main()` before `config.Load()`. Extract `printVersion(w io.Writer, v string)` for testability. Create `main_test.go` with a unit test for `printVersion` and two subprocess tests via `exec.Command` (default build + ldflags-injected build). Fix two cosmetic URL bugs (`pushkit/cli` → `jayteealao/PushKit`).
- **Key decisions locked (plan-stage discovery):**
  - No `-v` short alias (PO confirmed `--version` only; `-v` conflicts with Go verbose convention).
  - Unit + subprocess test (PO wants both `printVersion` unit test and `TestVersionFlag_LdflagsInjected` subprocess test).
  - Stdlib `flag` only — no new module dependencies.
  - Print to stdout, `os.Exit(0)`.
- **Key risk:** Subprocess test builds the binary in `TestMain` (~3–5 s overhead). Acceptable for xs; can be gated on `testing.Short()` if noisy.

### `android-versioning` (planned 2026-05-26)

- **Files to touch:** 2 (`android/app/build.gradle.kts`, `android/README.md` (new)).
- **Strategy:** Replace hard-coded `versionCode = 1` / `versionName = "1.0"` with `providers.gradleProperty` lookups and safe fallback defaults. No file mutation at CI time — CI passes `-PversionCodeOverride=N -PversionNameOverride=X` to Gradle. Local dev builds continue to use defaults with no flags required.
- **Key decisions locked (plan-stage discovery):**
  - `providers.gradleProperty` (lazy, configuration-cache-safe) over legacy `project.findProperty`.
  - Property names `versionCodeOverride` / `versionNameOverride` to avoid DSL name shadowing.
  - `toIntOrNull()` not `toInt()` — safe against empty-string edge case.
  - Verification via `aapt dump badging` only; no new test source set (zero existing Android tests; two-line build-config change doesn't justify infra from scratch).
  - README at `android/README.md` (module root, more discoverable).
- **Cross-slice CI contract:** `release-orchestration` MUST pass `-PversionCodeOverride=$(git rev-list --count HEAD)` and `-PversionNameOverride=${GITHUB_REF_NAME#v}` with `fetch-depth: 0` checkout. See `04-plan-android-versioning.md § CI Contract`.
- **Key risk:** `-P` vs `-D` flag confusion — `-D` sets a JVM system property, silently invisible to `providers.gradleProperty`. Documented in plan and must be carried forward to the `release-orchestration` plan.

### `release-orchestration` (planned 2026-05-26)

- **Files to touch:** 3 (rewrite `.github/workflows/release.yml`, new `cliff.toml`, modify `README.md` — add shields.io badge + `## Releasing` + `## Backend installer` sections).
- **Strategy:** Expand release.yml from 1 job to 10 jobs (8 default + 2 post-publish split by platform). Cross-compile Go on Linux → upload artifact → download on `windows-2022` → NSIS-wrap. `softprops/action-gh-release@v3` creates the GH Release (native retry + glob support; resolves the documented `gh release create` upload-flake risk). `negrutiu/nsis-install@v2` pins NSIS 3.12 (CVE-2025-43715 fix). `pypa/gh-action-pypi-publish@release/v1` with `attestations: false` (v0.x posture). Prerelease auto-detection covers `-rc`, `-alpha`, `-beta`. Break-glass `PYPI_API_TOKEN` documented in README only, no fallback job in workflow.
- **Key decisions locked (plan-stage discovery):**
  - Cross-compile + Windows NSIS-wrap (cheapest topology; aligns with NSIS slice's `..\pushkit-server.exe` contract).
  - `softprops/action-gh-release@v3` (built-in retry + glob; smaller blast radius than hand-rolled retry loops).
  - `windows-2022` explicit (avoids `windows-latest` → `windows-2025` drift and the June-2026 VS-2026 migration).
  - Match existing `ci.yml`/`release.yml` action pins exactly (no major-version bumps; only introduce `actions/upload-artifact@v4` and `actions/download-artifact@v4` because v3 is hard-deprecated).
  - Prerelease auto-detect: `-rc`, `-alpha`, `-beta` (broader than shape's literal `-rc.*`).
  - Sigstore attestations: false (SLSA hardening out of scope per intake).
  - Break-glass PYPI_API_TOKEN: documented in README only (no fallback job).
  - `negrutiu/nsis-install@v2` with explicit `nsis-version: "3.12"` pin.
  - `post-publish-checks`: two parallel jobs (Linux + Windows) — clear platform-driven split.
  - `cliff.toml`: init from `keepachangelog` template + minor tuning (tag_pattern, scope vocabulary).
  - Smoke-test source: download from the GH Release via `gh CLI` (end-to-end verification of the upload path).
- **Key risk:** First-release PyPI OIDC misconfig (shape Block F top failure mode) — mitigation is re-verifying the Trusted Publisher quartet on the PyPI dashboard before pushing the validation tag.

## Cross-Cutting Concerns

- **Conventional Commits enforcement starts at commit 1 of the branch.** Every other slice's commits must parse. The first commit on `feat/ship-plan-buildout` is `commit-hygiene`'s own setup and must be authored conventionally by hand (the hook can't yet validate its own installation commit).
- **Single shared branch (`feat/ship-plan-buildout`), single PR.** All five slices accumulate into one PR per intake's branch strategy. Each slice's implement/verify cycle runs locally on the branch before the next slice begins.
- **No CHANGELOG.md committed.** Per ship-plan Block B and shape decision, the changelog is generated at release time by `release-orchestration`. The generate-changelog job writes a per-release artifact (`CHANGELOG-<tag>.md`) consumed by `softprops/action-gh-release@v3`'s `body_path` and then discarded with the workflow run.
- **Node toolchain footprint is local-only.** CI uses `wagoid/commitlint-github-action@v6` which ships its own Node — no `npm ci` in any ci.yml job. The `package.json` exists solely so developers can run commitlint locally via lefthook.
- **NSIS 3.12 is the minimum across local + CI.** Driven by CVE-2025-43715 (NSIS ≤ 3.10 SYSTEM privilege escalation). Shape's freshness research said "pre-installed 3.10 on windows-2022 is fine"; this is now updated. `release-orchestration` uses `negrutiu/nsis-install@v2` with `nsis-version: "3.12"` explicitly.
- **Vendored installer plugin (`SimpleSC.dll`).** First vendored binary in the repo. Recorded SHA256 in `backend/installer/README.md`. SLSA hardening (signature verification) deferred to a future workflow per intake.
- **`backend/installer/pushkit.nsi` `File` directive resolves relative to the `.nsi` file.** From `backend/installer/`, `..\pushkit-server.exe` points to `backend/pushkit-server.exe`. Both local (`go build -o`) and CI (`build-backend-binary` cross-compile job places the artifact at `backend/pushkit-server.exe` via `actions/download-artifact@v4`) must put the binary at exactly that path. Cross-slice contract between `nsis-installer` and `release-orchestration`.
- **Installer NFR (≤25 MB).** Go binary alone is 27.2 MB. `SetCompressor lzma` typically nets ~30–40% reduction → expected ~17–20 MB. If actual exceeds 25 MB, the NFR will be raised in handoff (not blocked at plan time).
- **`fetch-depth: 0` is required for three jobs in `release.yml`.** `tag-guard` (needs history for `git merge-base --is-ancestor`), `build-android-apk` (needs full count for `git rev-list --first-parent --count HEAD`), `generate-changelog` (git-cliff needs full history). Silent failure mode: `fetch-depth: 1` produces a count of `1` and an empty changelog. Triple-check at implement-stage review.
- **PyPI Sigstore attestations are OFF.** `pypa/gh-action-pypi-publish@release/v1` with `attestations: false` per discovery Round 2. SLSA hardening (re-enabling attestations) is a future workflow.
- **Break-glass `PYPI_API_TOKEN` is documented, not wired.** The recovery procedure is in the new `## Releasing` README section. The default publish path is 100% OIDC; the secret stays sealed in repository secrets and is invoked manually by editing the publish step for one release when needed.
- **`-P` not `-D` for Gradle version-overrides.** `release-orchestration`'s `build-android-apk` job uses `-PversionCodeOverride` / `-PversionNameOverride`. Using `-D` would set a JVM system property that's silently invisible to `providers.gradleProperty` — the #1 documented Gradle CI failure mode (per `android-versioning` plan).
- **Prerelease tag heuristic broadens shape's literal.** Shape said `-rc.*`; discovery Round 2 broadened to `-rc`, `-alpha`, `-beta` (consistent with semver-prerelease intent). `softprops/action-gh-release@v3`'s `prerelease:` input is fed from a shell-expression step output.

## Integration Points Between Slices

- `commit-hygiene` → all other slices: their commits will be CI-validated by the `commitlint-backstop` job in `ci.yml`. Failure mode is loud (PR red).
- `commit-hygiene` ↔ `release-orchestration`: `ci.yml`'s `android-build` job uses default Gradle properties; `release.yml`'s `build-android-apk` job injects `-PversionCodeOverride` / `-PversionNameOverride`. No file conflict — different invocations of the same Gradle script.
- `commit-hygiene` ↔ `backend-version`: the `--version` flag added in `backend-version` is exercised by `release.yml`'s `post-publish-windows` smoke-test. `ci.yml` doesn't exercise `--version`; it just runs `go test ./...`. No conflict.
- `nsis-installer` → `release-orchestration` (HARD CONTRACT, satisfied by plan):
  - `build-backend-installer` (`windows-2022`) downloads the `backend-windows-binary` artifact into `backend/pushkit-server.exe` via `actions/download-artifact@v4` BEFORE invoking makensis. This is the path the `.nsi`'s `File "..\pushkit-server.exe"` directive resolves to.
  - CI invocation: `makensis /V3 /DVERSION=<tag-without-v> backend/installer/pushkit.nsi` from repo root.
  - Output artifact path: `backend/installer/pushkit-server-setup.exe`, uploaded as `windows-installer`.
  - NSIS version: 3.12 via `negrutiu/nsis-install@v2` with `nsis-version: "3.12"`.
- `backend-version` → `release-orchestration` (HARD CONTRACT, satisfied by plan):
  - `build-backend-binary` (Linux) builds with `-ldflags "-X main.Version=$VERSION_STRIPPED"` (the `v` prefix is stripped to match shape AC7's `pushkit-server 0.1.0-rc.1` expectation).
  - `post-publish-windows.smoke-test` asserts the output via `& "$env:ProgramFiles\PushKit\pushkit-server.exe" --version`.
- `android-versioning` → `release-orchestration` (HARD CONTRACT, satisfied by plan):
  - `build-android-apk` (Linux) uses `fetch-depth: 0` and passes `-PversionCodeOverride=$(git rev-list --first-parent --count HEAD) -PversionNameOverride=${GITHUB_REF_NAME#v}` to `./gradlew assembleDebug`.
  - **`-P` not `-D`** — silent failure otherwise (android-versioning plan flagged this as the #1 Gradle CI gotcha).
  - APK renamed from `app-debug.apk` → `pushkit-android.apk` before upload as artifact `android-apk` to match shape AC8.
- `nsis-installer` ↔ `commit-hygiene`: both touch root `.gitignore`. `commit-hygiene` creates it (adds `node_modules/`, `dist/`); `nsis-installer` appends (`backend/installer/pushkit-server-setup.exe`, `backend/pushkit-server.exe`). Clean append; no conflict.
- `nsis-installer` ↔ `backend-version`: no direct file overlap. The installer is indifferent to whether the wrapped binary has `--version`; `release-orchestration`'s smoke-test is what exercises `--version` post-install.

## Recommended Implementation Order

1. **`commit-hygiene`** — hard prerequisite; smallest planning surface; foundation. **Status: ✅ verified.**
2. **`nsis-installer`** — highest uncertainty; most iteration time. **Status: ✅ implemented (verify in progress).**
3. **`backend-version`** — mechanical. **Status: ✅ verified.**
4. **`android-versioning`** — single-file Gradle change. **Status: ✅ planned → ready to implement.**
5. **`release-orchestration`** — integrator. **Status: ✅ planned (2026-05-26). Ready to implement after slice 4 lands.**

## Conflicts Found

- **Root `.gitignore`**: created by `commit-hygiene`, appended by `nsis-installer`. No structural conflict — clean append if landed in slice order. If `nsis-installer` is rebased before `commit-hygiene` merges, the append must adapt to creating-from-scratch.
- **Slice AC3 vs PO discovery answer (within `nsis-installer`)**: PO chose "service component default-checked" which contradicts `03-slice-nsis-installer.md` line 67 ("silent skips service component"). Resolution: edit the slice doc when `nsis-installer` is implemented. Flagged as the single blocker in `04-plan-nsis-installer.md`.
- **NSIS version assumption in shape**: shape's freshness research said "rely on pre-installed 3.10 on windows-2022 runner" — superseded by CVE-2025-43715 finding. `release-orchestration`'s plan MUST add an explicit NSIS-install step (3.12 minimum). This is a contract update, not a code conflict.

No other slice has file-level overlap. `README.md` sections are owned by separate slices under distinct headings.

## Freshness Research

Carried forward from `02-shape.md` and supplemented by plan-stage web sub-agent (2026-05-23). All planned pins are current as of today:

| Tool | Pin | Current latest | Verdict |
|---|---|---|---|
| lefthook | `^2.1.8` | 2.1.8 (2026-05-19) | current |
| @commitlint/cli | `^21.0.0` | 21.0.1 (2026-05-17) | current |
| @commitlint/config-conventional | `^21.0.0` | 21.x | current |
| wagoid/commitlint-github-action | `@v6` | 6.2.1 (2025-01-14) | current; no v7 |
| actions/setup-java | `@v4` | v4 | current |
| gradle/actions/setup-gradle | `@v4` | v4 | current |
| actions/checkout | `@v4` | v4 | current |
| actions/setup-go | `@v5` | v5 | current |

No CVEs in the past 12 months for any pinned dependency. Supply-chain hardening (SHA-pinning) deferred to a future workflow per discovery decision.

## Recommended Next Stage

Current status (2026-05-26 T18:25 UTC, all five plans complete):
- `commit-hygiene`: ✅ verified
- `nsis-installer`: ✅ implemented (verify in progress)
- `backend-version`: ✅ verified
- `android-versioning`: ✅ planned → ready to implement
- `release-orchestration`: ✅ planned → ready to implement after slice 4

- **Option A (default):** `/wf implement ship-plan-buildout android-versioning` — finish the precursor slice. 2 files, 2 property lines, 1 README. Zero blockers. Run `/compact` first to clear planning context.
- **Option B:** `/wf implement ship-plan-buildout release-orchestration` — implement the integrator. Requires `android-versioning` to be implemented first (or in the same session) so `build-android-apk` has the `-P` overrides to consume. Plan is execution-ready; no blockers.
- **Option C:** `/wf plan ship-plan-buildout all` — review-all mode to re-validate all five plans for cross-cohesion now that the integrator's specifics are pinned. Optional sanity pass.
- **Option D:** `/wf review ship-plan-buildout` — early review pass against the cumulative branch diff (slug-wide review scope) before implementing the remaining slices. Lets reviewer findings shape the final integrator's implementation rather than chasing them post-hoc.
