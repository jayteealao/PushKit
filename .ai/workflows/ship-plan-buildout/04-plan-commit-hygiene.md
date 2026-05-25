---
schema: sdlc/v1
type: plan
slug: ship-plan-buildout
slice-slug: commit-hygiene
status: complete
stage-number: 4
created-at: "2026-05-22T23:44:29Z"
updated-at: "2026-05-22T23:44:29Z"
metric-files-to-touch: 7
metric-step-count: 10
has-blockers: false
revision-count: 0
stack-source: confirmed
tags:
  - lefthook
  - commitlint
  - github-actions
  - ci
refs:
  index: 00-index.md
  plan-index: 04-plan.md
  slice-def: 03-slice-commit-hygiene.md
  siblings:
    - 04-plan-nsis-installer.md          # not yet written
    - 04-plan-backend-version.md         # not yet written
    - 04-plan-android-versioning.md      # not yet written
    - 04-plan-release-orchestration.md   # not yet written
  implement: 05-implement-commit-hygiene.md
next-command: wf-implement
next-invocation: "/wf implement ship-plan-buildout commit-hygiene"
---

# Plan: commit-hygiene

## Current State

- `.github/workflows/` exists with one file: `release.yml` (52 lines, tag-driven PyPI publish). No pre-merge CI. No status checks on PRs.
- Repo root has **no** `.gitignore`, **no** `lefthook.yml`, **no** `commitlint.config.*`, **no** `package.json` — the slice introduces all of them cleanly.
- Backend (Go 1.24, module `github.com/pushkit/backend`) has 5 `*_test.go` files under `backend/internal/{api,auth,db,s3}`. Standard library testing, no testify, no testdata. `go test ./...` runs cleanly from `backend/`.
- CLI (Go 1.22.0, module `github.com/pushkit/cli`) has 1 `*_test.go` (`cli/internal/config/config_test.go`). Same conventions.
- Android (Gradle 8.5, JDK 17, Kotlin Compose ext 1.5.8, single-module). `android/app/src/test/` and `android/app/src/androidTest/` exist but contain no tests. `./gradlew testDebugUnitTest` will pass vacuously. No KAPT/KSP, no aggressive flags — 8-min p95 budget is feasible with caching.
- Commit history: 0 of the last 6 commits parse as Conventional Commits. Slice 1's own commits will be the first conformant entries (forward-only enforcement per shape Round 2).
- Existing `release.yml` style: 2-space indent, capitalized step names, kebab-case job names, `actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-python@v5`. New `ci.yml` mirrors this.

## Reuse Opportunities

Affected-code sub-agent's reuse scan found minimal reuse surface — this slice is essentially greenfield CI scaffolding. Specifically:

- `Makefile` (repo root) — exists but release-only (`build-wheels`, `publish`, `clean`). Decision in discovery: **do NOT add a `make test` target** in this slice. Keep ci.yml's test invocations inline per-job; revisit if a future slice needs the aggregator.
- `release.yml` (existing) — its action pinning style and step naming are the conventions to copy, not its content. Recommendation: **reuse the style, not the workflow**. Match `actions/checkout@v4`, capitalized step names, kebab-case job names.
- Lefthook / commitlint scaffolding — **no reuse candidates**. None exist in the repo.
- Lint config — **no reuse candidates**. `golangci-lint`, `ktlint`, etc., are not currently configured; out of scope for this slice (intake/shape did not request them).

Net: 6 new files + 1 modified file. No extract-and-share opportunities at this stage.

## Likely Files / Areas to Touch

New files:
- `lefthook.yml` (root) — `commit-msg` hook delegating to commitlint.
- `commitlint.config.cjs` (root) — extends `@commitlint/config-conventional` + scope-enum.
- `package.json` (root) — devDependencies (commitlint v21, lefthook v2.1.8) + `prepare` script.
- `package-lock.json` (root) — npm lockfile, committed.
- `.gitignore` (root) — new, covering `node_modules/` and `dist/` (since neither root-level path is currently ignored).
- `.github/workflows/ci.yml` — 4 parallel jobs.

Modified files:
- `README.md` — expand the existing `## Testing` section to cover `lefthook install` (or document via `npm i` postinstall hook), and document the conventional-commits scope vocabulary.

## Proposed Change Strategy

Two-track:

1. **Local enforcement track** (`lefthook.yml`, `commitlint.config.cjs`, `package.json`, `package-lock.json`, `.gitignore`, README). Goal: a developer who clones, runs `npm i`, and tries a bad commit message gets blocked locally. `prepare: lefthook install` makes the hook auto-install during `npm i`.

2. **CI backstop track** (`.github/workflows/ci.yml`). Goal: a PR that bypasses the local hook (`--no-verify`) still fails red. Four jobs run in parallel for fast feedback: `backend-test`, `cli-test`, `android-build`, `commitlint-backstop`. The commitlint job uses `wagoid/commitlint-github-action@v6` — it ships its own Node, so no `npm ci` in CI.

Sequence the implementation so the **CI workflow lands together with the local hooks** in the same commit-set on `feat/ship-plan-buildout`. The first commit on the branch is itself the lefthook+commitlint setup and MUST parse as Conventional Commits (forward-only enforcement starts at commit 1). Suggested first-commit shape: `feat(ci): add commit-msg hook and pre-merge workflow`.

## Step-by-Step Plan

1. **Create root `.gitignore`** — add at minimum:
   ```
   node_modules/
   dist/
   ```
   (Existing `cli/.gitignore` already ignores `dist/` for the cli build; the root entry catches the `dist/` produced by `make build-wheels` at repo root and the new `node_modules/` from commitlint install.)

2. **Create `package.json`** at repo root:
   - `"name": "pushkit-devtools"` (or similar non-publishable placeholder), `"private": true`, no `"type": "module"` (lets `.cjs` config load naturally).
   - `"devDependencies"`: `"@commitlint/cli": "^21.0.0"`, `"@commitlint/config-conventional": "^21.0.0"`, `"lefthook": "^2.1.8"`.
   - `"scripts": { "prepare": "lefthook install" }` — runs once per `npm i`. Idempotent; safe to re-run.
   - `"engines": { "node": ">=20" }` for documentation (commitlint v21 ESM floor).

3. **Run `npm install`** once locally to generate `package-lock.json`. Commit the lockfile. Verify lockfile integrity is deterministic by re-running on a clean clone.

4. **Create `commitlint.config.cjs`** at repo root:
   ```js
   module.exports = {
     extends: ['@commitlint/config-conventional'],
     rules: {
       'scope-enum': [
         1, // warning
         'always',
         ['backend', 'cli', 'android', 'ci', 'docs', 'deps', 'installer', 'release'],
       ],
     },
   };
   ```
   Warning-severity scope-enum per discovery decision. Promote to severity 2 (error) in a future slice once vocabulary is stable.

5. **Create `lefthook.yml`** at repo root:
   ```yaml
   commit-msg:
     commands:
       commitlint:
         run: npx --no -- commitlint --edit {1}
   ```
   `{1}` is lefthook's placeholder for the commit-msg file path. `npx --no` refuses to fetch packages on the fly — fails fast if commitlint isn't installed.

6. **Create `.github/workflows/ci.yml`** — four parallel jobs under one workflow. Skeleton:
   ```yaml
   name: CI

   on:
     pull_request:
       branches: [main]
     push:
       branches: [main]

   permissions:
     contents: read

   jobs:
     backend-test:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
         - uses: actions/setup-go@v5
           with:
             go-version: "1.24"
         - run: go vet ./...
           working-directory: backend
         - run: go test ./...
           working-directory: backend

     cli-test:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
         - uses: actions/setup-go@v5
           with:
             go-version: "1.24"
         - run: go vet ./...
           working-directory: cli
         - run: go test ./...
           working-directory: cli

     android-build:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
         - uses: actions/setup-java@v4
           with:
             distribution: temurin
             java-version: "17"
         - uses: gradle/actions/setup-gradle@v4
           with:
             cache-read-only: ${{ github.ref != 'refs/heads/main' }}
         - run: ./gradlew assembleDebug lint testDebugUnitTest
           working-directory: android

     commitlint-backstop:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
           with:
             fetch-depth: 0
         - uses: wagoid/commitlint-github-action@v6
           with:
             configFile: commitlint.config.cjs
   ```

   Notes for implementation stage:
   - Single Go version (`1.24`) across both Go jobs per discovery decision. CLI module declares 1.22.0 in `cli/go.mod`; Go 1.24 compiles it without issue (newer toolchain compiling an older-declared module is supported). Note as a follow-up tidy-up to align `cli/go.mod` with `backend/go.mod` in a later slice.
   - `setup-go@v5` automatically caches `~/go/pkg/mod` keyed on `go.sum` — no explicit `actions/cache` needed.
   - `setup-gradle@v4` provides built-in caching keyed on the build files; `cache-read-only` on non-main avoids cache pollution from PRs (per freshness sub-agent recommendation).
   - `commitlint-backstop` requires `fetch-depth: 0` so it can lint every commit in the PR (`wagoid/commitlint-github-action@v6` walks the PR commit range).
   - Floating tag pinning per discovery decision (`@v4`, `@v5`, `@v6`) — matches existing `release.yml`. SHA-pinning deferred to SLSA-hardening pass.

7. **Update `README.md`**:
   - Expand the existing `## Testing` section (lines 122–131) OR add a new `## Development setup` subsection above it. Cover:
     - Prerequisite: Node ≥ 20.
     - One-time setup: `npm install` (auto-runs `lefthook install` via the `prepare` script).
     - What it enforces: Conventional Commits (`feat(scope): ...`) with scope from the fixed vocabulary.
     - Bypass note: `--no-verify` bypasses local hooks but CI catches it.
   - Add a one-line "scope vocabulary: backend, cli, android, ci, docs, deps, installer, release" so contributors don't guess.

8. **Local validation before pushing**:
   - `npm install` (clean) → lockfile generated, `lefthook install` ran (visible in `.git/hooks/commit-msg`).
   - `echo "broken message" | npx commitlint` → exits non-zero with a clear error.
   - `git commit -m "test bad commit" --allow-empty` (after `lefthook install`) → rejected.
   - `git commit -m "feat(ci): test conventional" --allow-empty` → accepted.
   - `cd backend && go vet ./... && go test ./...` → green.
   - `cd cli && go vet ./... && go test ./...` → green.
   - `cd android && ./gradlew assembleDebug lint testDebugUnitTest` → green (will likely take 4–7 min cold).

9. **First branch commit shape**: The first commit on `feat/ship-plan-buildout` will be this slice's setup. Author it conventionally:
   ```
   feat(ci): add commit-msg hook and pre-merge workflow
   ```
   The commit-msg hook can't yet enforce itself on commit 1 (the hook is installed by `npm i` which runs after commit 1 lands in any subsequent clone). Author it conventionally by hand; CI backstop will validate retroactively on the PR.

10. **Open the PR draft**. Push the branch. Confirm all four `ci.yml` jobs run on the PR. Confirm each turns green within the 8-minute p95 budget (first cold-cache run may push closer to 8 min; subsequent runs much faster).

## Test / Verification Plan

### Automated checks

- **lint/typecheck:** `go vet ./...` is part of both backend-test and cli-test jobs. No separate lint stage; Android's `./gradlew lint` produces `app/build/reports/lint-results-debug.html` and fails the job on issues (default behavior).
- **unit tests:**
  - Backend: 5 existing test files under `backend/internal/`. `go test ./...` invokes them; expected pass count > 0.
  - CLI: 1 existing test file (`config_test.go`). `go test ./...` invokes it; expected pass count = 1.
  - Android: `testDebugUnitTest` will report 0 tests (no test sources yet). Vacuous pass is acceptable for this slice — Android test scaffolding is out of scope.
- **commit-message validation (PR backstop):** `wagoid/commitlint-github-action@v6` walks every commit in the PR diff. Fails red on the first non-conventional commit.
- **integration tests:** Not applicable to this slice (no production code added).

### Interactive verification (human-in-the-loop)

**AC3 — Local commit-msg hook rejects non-conventional commits**

- **What to verify:** A fresh clone with `npm install` produces a working `commit-msg` hook that rejects `git commit -m "WIP hack"` and accepts `git commit -m "feat(ci): demo"`.
- **Platform & tool:** Developer machine (the maintainer's Windows 11). Tooling: `git` + the lefthook-installed hook + commitlint v21 from `node_modules/`.
- **Companion skills:** None — this is pure git CLI + Node. Stack lists `framework-conventions-guide`, `android-cli`, `lazylogcat`, `tech-research-enforcer`, `testing-setup`; none apply here.
- **Steps:**
  1. `git clone <repo>` (or `git checkout feat/ship-plan-buildout` on a fresh worktree).
  2. `npm install` — should print "lefthook installed" or similar, write `.git/hooks/commit-msg`.
  3. `git commit -m "WIP hack" --allow-empty` — must be rejected; output should reference Conventional Commits.
  4. `git commit -m "feat(ci): demo" --allow-empty` — must be accepted.
  5. `git reset HEAD~` to undo the demo commit before pushing.
- **Evidence capture:** Copy/paste of terminal output for both the rejection and acceptance cases into `06-verify-commit-hygiene.md`.
- **Pass criteria:** Rejection step exits non-zero with a message naming the offending rule (e.g., "type may not be empty" or "subject may not be empty"); acceptance step exits zero.

**AC2 — Commitlint backstop fires on a non-conventional commit in a PR**

- **What to verify:** A PR containing a commit whose message doesn't parse as Conventional Commits causes the `commitlint-backstop` job in `ci.yml` to fail red.
- **Platform & tool:** GitHub Actions UI (browser). `gh` CLI for shortcuts.
- **Steps:**
  1. On a throwaway branch off `feat/ship-plan-buildout`, `git commit -m "obviously bad" --allow-empty --no-verify` (the `--no-verify` is the bypass we're testing).
  2. Push and open a PR against `feat/ship-plan-buildout` (or main, depending on what's mergeable).
  3. Watch `commitlint-backstop` job in the Actions UI; must turn red.
  4. Close the PR without merging.
- **Evidence capture:** Screenshot of the red `commitlint-backstop` job in the Actions UI.
- **Pass criteria:** Job fails with a non-zero exit code citing the bad commit.

**8-minute p95 wall-time NFR**

- **What to verify:** All four ci.yml jobs complete in ≤ 8 minutes on a normal PR.
- **Platform & tool:** GitHub Actions UI; the workflow's run duration.
- **Steps:**
  1. Open the PR for this slice.
  2. Wait for the workflow to finish.
  3. Note total wall time of the slowest job (likely `android-build` on cold cache).
- **Evidence capture:** Screenshot or `gh run view <id>` output showing per-job durations.
- **Pass criteria:** Total run time ≤ 8 minutes from PR push to all-green. A first cold run may be marginal; allow one retry to populate the Gradle cache before judging p95.

## Risks / Watchouts

- **Node toolchain footprint.** Introducing `package.json` + `package-lock.json` to a Go/Kotlin repo. Footprint is small (3 devDependencies); justified in the README. Watch for accidental scope creep — future contributors may add unrelated Node tools.
- **Lefthook bypass via `--no-verify`.** Local hook is bypassable; CI backstop is the durable defense. Documented in README.
- **Gradle cold-cache time.** First-run `assembleDebug + lint + testDebugUnitTest` on cold runner approaches 8 min. `setup-gradle@v4` built-in caching should bring p95 well below 8 min once a few runs populate cache. If p95 still exceeds 8 min after 3 runs, consider trimming `lint` or moving Android to a separate workflow.
- **Re-tag of `wagoid/commitlint-github-action`.** Floating `@v6` tag could be re-pointed. Accepted residual risk for v0.x (consistent with existing `release.yml` style). Revisit during SLSA hardening.
- **First commit ambiguity.** Commit 1 of the branch installs the hook but cannot pre-validate itself. Author it conventionally by hand; CI backstop validates the PR's full commit range so the bad-first-commit scenario fails CI.
- **commitlint v21 ESM + cosmiconfig + `.cjs`.** Cosmiconfig loads `.cjs` synchronously even when the consumer (commitlint) is ESM. Confirmed by freshness sub-agent. No action needed but flag as a re-verify point if v22+ ever changes config-loading behavior.
- **`prepare` script runs on every `npm i`.** Idempotent — `lefthook install` is safe to re-run. If a developer's existing hook directory has manual files, lefthook will overwrite the `commit-msg` hook. Acceptable for a repo with no prior hook scaffolding.
- **Windows-specific gotcha for `npm i` `prepare` script.** `lefthook` ships a per-platform binary inside its npm package; on Windows it should resolve to `lefthook.exe`. Verify on the maintainer's Windows 11 during AC3 — if `prepare` fails on Windows, fall back to a documented manual `lefthook install` invocation in the README.
- **Android `testDebugUnitTest` with zero tests.** Gradle will print "no tests to run" and exit 0. Confirmed acceptable; not a real signal but cheap to keep wired so the slot exists when tests are added later.

## Dependencies on Other Slices

None inbound. This slice is the root of the workflow's dependency graph.

Outbound:
- `backend-version`, `android-versioning`, `nsis-installer`, `release-orchestration` all assume Conventional Commits enforcement is live, so their commit messages parse for `git-cliff`. They depend on this slice landing first.
- The `.github/workflows/ci.yml` `android-build` job is a sibling to release-orchestration's `build-android-apk` (which will rewrite `versionCode`/`versionName` before assembling). Cohesion: ci.yml uses defaults (no rewrite), release.yml uses the rewrite. No conflict — different workflows, different invocations.

## Assumptions

- The maintainer has Node ≥ 20 installed locally. (Freshness research confirms commitlint v21 requires this; the npm `engines` field documents it but doesn't enforce.)
- The maintainer is the sole developer for the foreseeable future, so the local hook + CI backstop pair is sufficient. No multi-contributor onboarding friction to design for in v0.x.
- The existing 6 non-conventional commits stay as-is (forward-only per shape). No history rewrite.
- `setup-gradle@v4`'s default cache scope works for a single-module Android project. The freshness sub-agent confirmed no v4-specific multi-module collision bug; we are single-module so no issue.
- No additional lint surface (`golangci-lint`, `ktlint`, ESLint) is added in this slice. They are out of scope; this slice is purely commit-hygiene + pre-merge smoke.

## Blockers

None. All inputs are confirmed; all tooling is current.

## Freshness Research

Inherited from shape's freshness research, augmented by plan-stage sub-agent (web research run 2026-05-23). Pinned versions as of today:

- **lefthook v2.1.8** — released 2026-05-19, latest stable. `lefthook install` is the documented install path. `commit-msg.commands.commitlint.run: 'npx --no -- commitlint --edit {1}'` matches the official Lefthook commitlint example.
- **@commitlint/cli v21.0.1** + **@commitlint/config-conventional v21** — released 2026-05-17. Pure ESM but `commitlint.config.cjs` still loads via cosmiconfig (loader is dynamic). Node ≥ 20 floor (documented via `engines`).
- **wagoid/commitlint-github-action@v6** (latest v6.2.1) — no v7 has shipped. Stable. Inputs: `configFile`, `failOnWarnings`, `firstParent`, `helpURL`.
- **actions/setup-java@v4** — Temurin is the recommended distribution. JDK 17 matches AGP 8.x minimum.
- **gradle/actions/setup-gradle@v4** — current. Built-in caching uses Gradle's own cache primitives; `cache-read-only` input controls write behavior. `cache-read-only: ${{ github.ref != 'refs/heads/main' }}` is the recommended idiom to keep main's cache pristine.
- **actions/checkout@v4, actions/setup-go@v5** — both still current. setup-go@v5 auto-caches `~/go/pkg/mod` keyed on `go.sum`.

Anti-pattern avoided: the freshness sub-agent flagged that running `npm install` inside Go/Android CI jobs is unnecessary because `wagoid/commitlint-github-action@v6` ships its own Node. The `commitlint-backstop` job therefore does NOT call `npm ci` — only the local developer flow uses npm.

Supply-chain note: floating tags accept the residual `tj-actions/changed-files`-style risk (CVE-2025-30066). Accepted for v0.x; SHA-pinning deferred to a future SLSA-hardening workflow.

## Revision History

*(none — first revision)*

## Recommended Next Stage

- **Option A (default):** `/wf implement ship-plan-buildout commit-hygiene` — plan is execution-ready; all decisions locked; no blockers. Run `/compact` first to clear planning context before implementation.
- **Option B:** `/wf plan ship-plan-buildout all` — plan the remaining four slices in parallel. Viable since `nsis-installer`, `backend-version`, `android-versioning` are independent of each other after `commit-hygiene` lands. Trade-off: large context.
- **Option C:** `/wf plan ship-plan-buildout nsis-installer` — plan the next slice in risk-first order. Recommended if the maintainer wants to interleave planning across slices instead of front-loading all plans.
