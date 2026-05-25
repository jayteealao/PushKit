---
schema: sdlc/v1
type: plan-index
slug: ship-plan-buildout
status: complete
stage-number: 4
created-at: "2026-05-22T23:44:29Z"
updated-at: "2026-05-22T23:44:29Z"
planning-mode: single
slices-planned: 1
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
next-invocation: "/wf implement ship-plan-buildout commit-hygiene"
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

### `nsis-installer` (not yet planned)

- Pending. Next planning candidate if maintainer chooses to plan more before implementing. Risk-first order says it's the highest-uncertainty remaining slice.

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

## Integration Points Between Slices

- `commit-hygiene` → all other slices: their commits will be CI-validated by the `commitlint-backstop` job in `ci.yml`. Failure mode is loud (PR red).
- `commit-hygiene` ↔ `release-orchestration`: `ci.yml`'s `android-build` job uses default Gradle properties; `release.yml`'s build-android-apk job will inject `-PversionCodeOverride` / `-PversionNameOverride` (planned by `android-versioning` slice). No file conflict — different invocations of the same Gradle script.
- `commit-hygiene` ↔ `backend-version`: the `--version` flag added in `backend-version` will eventually be exercised by `release.yml`'s post-publish smoke test. `ci.yml` doesn't exercise `--version`; it just runs `go test ./...`. No conflict.

## Recommended Implementation Order

1. **`commit-hygiene`** — hard prerequisite; smallest planning surface; foundation. Plan is ready (this file's sibling `04-plan-commit-hygiene.md`).
2. **`nsis-installer`** — highest uncertainty; most iteration time. Plan after step 1 lands (or in parallel via `/wf plan ship-plan-buildout all`).
3. **`backend-version`** — mechanical. Quick plan.
4. **`android-versioning`** — single-file Gradle change. Quick plan.
5. **`release-orchestration`** — integrator. Plan after 2–4 to incorporate their final shapes.

## Conflicts Found

None at plan stage. Each slice touches a disjoint set of files; the only overlap is `README.md` (each slice contributes its own section under a different heading) and `.github/workflows/release.yml` (only `release-orchestration` will modify it; `backend-version` only touches the cosmetic URL line which is non-conflicting).

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

- **Option A (default):** `/wf implement ship-plan-buildout commit-hygiene` — execute the planned slice. Run `/compact` first; planning context (research, alternatives) is noise for implementation.
- **Option B:** `/wf plan ship-plan-buildout all` — plan the remaining four slices in parallel. Viable since `nsis-installer`, `backend-version`, `android-versioning` are independent. Trade-off: large planning context across multiple files.
- **Option C:** `/wf plan ship-plan-buildout nsis-installer` — plan the next slice individually before implementing. Recommended if the maintainer wants to interleave planning across the workflow rather than front-load all plans.
