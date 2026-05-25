---
schema: sdlc/v1
type: implement
slug: ship-plan-buildout
slice-slug: commit-hygiene
status: complete
stage-number: 5
created-at: "2026-05-25T11:54:43Z"
updated-at: "2026-05-25T11:54:43Z"
metric-files-changed: 7
metric-lines-added: 511
metric-lines-removed: 0
metric-deviations-from-plan: 0
metric-review-fixes-applied: 0
commit-sha: ""
tags:
  - lefthook
  - commitlint
  - github-actions
  - ci
refs:
  index: 00-index.md
  implement-index: 05-implement.md
  slice-def: 03-slice-commit-hygiene.md
  plan: 04-plan-commit-hygiene.md
  siblings: []
  verify: 06-verify-commit-hygiene.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout commit-hygiene"
---

# Implement: commit-hygiene

## Summary of Changes

- Added root `.gitignore` covering `node_modules/` and `dist/`.
- Added `package.json` with `@commitlint/cli`, `@commitlint/config-conventional`, `lefthook` devDeps and `prepare: lefthook install` script.
- Generated `package-lock.json` via `npm install` (78 packages; 0 vulnerabilities). Lefthook auto-installed the `commit-msg` hook on install.
- Added `commitlint.config.cjs` extending `@commitlint/config-conventional` with `scope-enum` at warning severity (promotion to error deferred per plan decision).
- Added `lefthook.yml` with `commit-msg` hook delegating to `npx --no -- commitlint --edit {1}`.
- Added `.github/workflows/ci.yml` with four parallel jobs: `backend-test`, `cli-test`, `android-build`, `commitlint-backstop`.
- Updated `README.md` with a `## Development setup` section covering the one-time `npm install` step, Conventional Commits enforcement, scope vocabulary, and the `--no-verify` bypass note.

## Files Changed

- `.gitignore` — new; ignores `node_modules/` and `dist/`
- `package.json` — new; devDeps + prepare hook
- `package-lock.json` — new; lockfile committed per plan
- `commitlint.config.cjs` — new; conventional commits config with scope-enum warning
- `lefthook.yml` — new; commit-msg hook → commitlint
- `.github/workflows/ci.yml` — new; 4-job pre-merge CI
- `README.md` — modified; added `## Development setup` section

## Shared Files (also touched by sibling slices)

None for this slice. `README.md` may also be touched by `backend-version` (for `--version` flag docs) and `nsis-installer` (for installer docs) but those slices will be responsible for their own README sections.

## Notes on Design Choices

- Single Go version `1.24` across both Go jobs — CLI module declares `1.22.0` in `go.mod` but a newer toolchain compiling an older-declared module is supported. Noted as a follow-up tidy-up.
- `cache-read-only: ${{ github.ref != 'refs/heads/main' }}` on Gradle cache keeps main's cache write-capable while PRs are read-only, per plan recommendation.
- `commitlint-backstop` uses `fetch-depth: 0` so `wagoid/commitlint-github-action@v6` can walk the full PR commit range.
- `wagoid/commitlint-github-action@v6` ships its own Node runtime, so no `npm ci` in CI — only the local developer flow uses npm.

## Deviations from Plan

None. All 10 plan steps implemented as specified.

## Anything Deferred

- Promote `scope-enum` from warning (severity 1) to error (severity 2) once the vocabulary is stable — noted in plan as a future slice.
- SHA-pinning of GitHub Actions to specific SHAs — deferred to SLSA-hardening pass per plan decision. Currently using floating major-version tags (`@v4`, `@v5`, `@v6`) consistent with existing `release.yml`.
- Align `cli/go.mod` go version from `1.22.0` to match `backend/go.mod`'s `1.24` — noted as a follow-up tidy-up.

## Known Risks / Caveats

- **Windows `prepare` script:** `lefthook` npm package ships a per-platform binary; on Windows it resolves to `lefthook.exe`. Needs verification on the maintainer's Windows 11 (AC3). If `prepare` fails on Windows, documented fallback is manual `lefthook install`.
- **Gradle cold-cache time:** First-run `assembleDebug + lint + testDebugUnitTest` on cold runner may approach the 8-minute p95 budget. `gradle/actions/setup-gradle@v4` built-in caching should bring subsequent runs well under budget.
- **First commit ambiguity:** The `commit-msg` hook cannot validate the commit that introduces itself (it runs after `npm i` which happens on a subsequent clone). The first commit is authored conventionally by hand; CI backstop validates the full PR commit range.

## Freshness Research

No additional freshness research required at implement stage. All version pins and action choices were confirmed by the plan-stage freshness sub-agent (run 2026-05-23) and matched the current output of `npm install` (commitlint v21, lefthook v2.x, 0 vulnerabilities).

## Recommended Next Stage

- **Option A (default):** `/wf verify ship-plan-buildout commit-hygiene` — implementation touches testable behavior (Go tests, Gradle build, commitlint rejection). Run `/compact` first to clear implementation noise from context.
- **Option B:** `/wf review ship-plan-buildout commit-hygiene` — skip verify if the CI run on the PR serves as verification evidence.
