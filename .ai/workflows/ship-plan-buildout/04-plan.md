---
schema: sdlc/v1
type: plan-index
slug: ship-plan-buildout
status: complete
stage-number: 4
created-at: "2026-05-22T23:44:29Z"
updated-at: "2026-05-25T23:33:45Z"
planning-mode: single
slices-planned: 2
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
refs:
  index: 00-index.md
  slice-index: 03-slice.md
next-command: wf-implement
next-invocation: "/wf implement ship-plan-buildout nsis-installer"
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

### `backend-version` (not yet planned)

- Pending. Small mechanical slice; can be planned quickly after `commit-hygiene` lands.

### `android-versioning` (not yet planned)

- Pending. Single-file Gradle change; can be planned quickly.

### `release-orchestration` (not yet planned)

- Pending. The integrator. Should be planned last (or last-but-one) because its plan depends on the other four slices' final shapes.

## Cross-Cutting Concerns

- **Conventional Commits enforcement starts at commit 1 of the branch.** Every other slice's commits must parse. The first commit on `feat/ship-plan-buildout` is `commit-hygiene`'s own setup and must be authored conventionally by hand (the hook can't yet validate its own installation commit).
- **Single shared branch (`feat/ship-plan-buildout`), single PR.** All five slices accumulate into one PR per intake's branch strategy. Each slice's implement/verify cycle runs locally on the branch before the next slice begins.
- **No CHANGELOG.md committed.** Per ship-plan Block B and shape decision, the changelog is generated at release time by `release-orchestration`. Not part of this slice.
- **Node toolchain footprint is local-only.** CI uses `wagoid/commitlint-github-action@v6` which ships its own Node — no `npm ci` in any ci.yml job. The `package.json` exists solely so developers can run commitlint locally via lefthook.
- **NSIS 3.12 is the minimum across local + CI.** Driven by CVE-2025-43715 (NSIS ≤ 3.10 SYSTEM privilege escalation). Shape's freshness research said "pre-installed 3.10 on windows-2022 is fine"; this is now updated. `release-orchestration`'s CI must install NSIS 3.12 explicitly (marketplace action) rather than rely on the runner image.
- **Vendored installer plugin (`SimpleSC.dll`).** First vendored binary in the repo. Recorded SHA256 in `backend/installer/README.md`. SLSA hardening (signature verification) deferred to a future workflow per intake.
- **`backend/installer/pushkit.nsi` `File` directive resolves relative to the `.nsi` file.** From `backend/installer/`, `..\pushkit-server.exe` points to `backend/pushkit-server.exe`. Both local (`go build -o`) and CI (cross-compile artifact placement) must put the binary at exactly that path. Cross-slice contract between `nsis-installer` and `release-orchestration`.
- **Installer NFR (≤25 MB).** Go binary alone is 27.2 MB. `SetCompressor lzma` typically nets ~30–40% reduction → expected ~17–20 MB. If actual exceeds 25 MB, the NFR will be raised in handoff (not blocked at plan time).

## Integration Points Between Slices

- `commit-hygiene` → all other slices: their commits will be CI-validated by the `commitlint-backstop` job in `ci.yml`. Failure mode is loud (PR red).
- `commit-hygiene` ↔ `release-orchestration`: `ci.yml`'s `android-build` job uses default Gradle properties; `release.yml`'s build-android-apk job will inject `-PversionCodeOverride` / `-PversionNameOverride` (planned by `android-versioning` slice). No file conflict — different invocations of the same Gradle script.
- `commit-hygiene` ↔ `backend-version`: the `--version` flag added in `backend-version` will eventually be exercised by `release.yml`'s post-publish smoke test. `ci.yml` doesn't exercise `--version`; it just runs `go test ./...`. No conflict.
- `nsis-installer` → `release-orchestration` (HARD CONTRACT):
  - CI's cross-compile job MUST place the Windows binary at `backend/pushkit-server.exe` (relative to repo root) before invoking makensis. This is the path the `.nsi`'s `File "..\pushkit-server.exe"` directive resolves to.
  - CI invocation: `makensis /V3 /DVERSION=<tag-without-v> backend/installer/pushkit.nsi` from repo root.
  - Output artifact path: `backend/installer/pushkit-server-setup.exe`.
  - NSIS version: 3.12 minimum (install step required; pre-installed 3.10 on `windows-2022` is no longer acceptable).
- `nsis-installer` ↔ `commit-hygiene`: both touch root `.gitignore`. `commit-hygiene` creates it (adds `node_modules/`, `dist/`); `nsis-installer` appends (`backend/installer/pushkit-server-setup.exe`, `backend/pushkit-server.exe`). Clean append; no conflict if landed in slice order.
- `nsis-installer` ↔ `backend-version`: no direct file overlap. The installer is indifferent to whether the wrapped binary has `--version`; the shape's smoke-test in `release-orchestration` is what exercises `--version` post-install.

## Recommended Implementation Order

1. **`commit-hygiene`** — hard prerequisite; smallest planning surface; foundation. Plan ready in `04-plan-commit-hygiene.md`. **Status: implemented; verify in progress.**
2. **`nsis-installer`** — highest uncertainty; most iteration time. Plan ready in `04-plan-nsis-installer.md`. **Status: planned 2026-05-25.**
3. **`backend-version`** — mechanical. Quick plan.
4. **`android-versioning`** — single-file Gradle change. Quick plan.
5. **`release-orchestration`** — integrator. Plan after 2–4 to incorporate their final shapes. Must consume `nsis-installer`'s file-path contracts and the NSIS 3.12 minimum-version requirement.

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

- **Option A (default):** `/wf implement ship-plan-buildout nsis-installer` — execute the freshly-planned slice. `commit-hygiene` is already verified (post-implement); branch is on `feat/ship-plan-buildout`. Plan has one flagged blocker (slice AC3 doc update), resolvable inline. Run `/compact` first to clear planning context.
- **Option B:** `/wf plan ship-plan-buildout backend-version` — plan the next slice (xs/mechanical) before implementing `nsis-installer`. Useful if the maintainer wants all remaining plans before any further implementation.
- **Option C:** `/wf plan ship-plan-buildout all` — plan the remaining three slices in parallel (`backend-version`, `android-versioning`, `release-orchestration`). Trade-off: large context across multiple files; `release-orchestration` benefits from having the other plans finalized first.
