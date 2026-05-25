---
schema: sdlc/v1
type: sync-report
slug: ship-plan-buildout
created-at: "2026-05-22T21:45:10Z"
health: in-sync
refs:
  - 00-index.md
  - 01-intake.md
  - po-answers.md
---

# Sync Report: ship-plan-buildout

**Health:** ✓ in-sync | **Checked:** 2026-05-22T21:45:10Z | **Workflow updated:** 2026-05-22T21:28:25Z

## Summary

| Category | Total | ✓ OK | ⚠ Drifted | ✗ Missing (planned) | ✗ Missing (unexpected) |
|----------|-------|------|-----------|---------------------|------------------------|
| Code Files | 9 | 6 | 0 | 3 | 0 |
| Test Files | 0 | 0 | 0 | 0 | 0 |
| Git State | 4 | 4 | 0 | 0 | 0 |
| Dependencies | 4 | 4 | 0 | 0 | 0 |
| External | 2 | 2 | 0 | 0 | 0 |

Workflow is at **stage 1 (intake)** — the only references in stage files are forward-looking plans, not claims of completed change. Drift detection is therefore trivially clean: every "missing" file is something this workflow is meant to create later.

## Code Files

| Reference | Status | Source stage | Notes |
|---|---|---|---|
| `.github/workflows/release.yml` | ✓ exists | 01-intake.md | Currently single-job; workflow plans to expand. No drift. |
| `Makefile` | ✓ exists | 01-intake.md | URL fix is in scope; file present. |
| `backend/Dockerfile` | ✓ exists | 01-intake.md | Marked out-of-scope; file present. |
| `backend/docker-compose.yml` | ✓ exists | 01-intake.md | Marked out-of-scope; file present. |
| `cli/go.mod` | ✓ exists | 01-intake.md | Present. |
| `backend/go.mod` | ✓ exists | 01-intake.md | Present. |
| `android/app/build.gradle.kts` | ✓ exists | 01-intake.md | Target of CI-driven versionName/versionCode rewrite (planned). |
| `backend/installer/pushkit.nsi` | ✗ missing (planned) | 01-intake.md | This workflow creates it. Not drift. |
| `cliff.toml` | ✗ missing (planned) | 01-intake.md | This workflow creates it. Not drift. |
| `.github/workflows/ci.yml` | ✗ missing (planned) | 01-intake.md | Open question whether it's a new file or release.yml is extended. Not drift. |

## Test Files

None referenced at intake stage. Existing test suites (`go test ./...` in backend + cli, Gradle unit tests) will be invoked by the planned pre-merge workflow but no specific test paths are claimed yet.

## Git State

- **Branch `feat/ship-plan-buildout`:** ✓ not yet created (expected — branch is created at handoff, not intake).
- **Current branch:** `main` ✓.
- **Base divergence from `main`:** 0 ahead, 0 behind ✓.
- **PR:** ✓ not yet created (`pr-number: 0` in index — expected).

No git drift.

## Dependencies

| Reference | Status | Notes |
|---|---|---|
| `pypa/gh-action-pypi-publish@release/v1` | ✓ ok | Referenced in `.github/workflows/release.yml`; intake plans no change to the action version pin. |
| `actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-python@v5` | ✓ ok | All referenced in `release.yml`; intake plans no change. |
| `go-to-wheel`, `twine` | ✓ ok | Pip-installed in CI on demand; not local manifest deps. Behavior unchanged. |
| `git-cliff`, `makensis`, commitlint runner | ✓ ok (not yet introduced) | Not yet referenced in any manifest; planned additions. Choice of commitlint runner (lefthook / husky / vendored) is an open question, not drift. |

## External References

| Reference | Status | Notes |
|---|---|---|
| `.ai/ship-plan.md` | ✓ exists | Required input contract. Authored 2026-05-22T21:14:08Z; intake created later. No drift. |
| PyPI Trusted Publisher entry (`pushkit`) | ✓ confirmed by PO | Out-of-scope to (re)create; assumed live. |

## Drift Details

None. Every reference is either present-and-stable or explicitly planned-to-be-created by this workflow.

## Recommended Actions

1. Continue to `/wf shape ship-plan-buildout` — eight open questions captured at intake (commitlint runner, Android `versionCode` source, NSIS scope, pre-merge CI file layout, installer bundle contents, SHA256SUMS, README badge format, CI backstop for commitlint) are best resolved in shape's structured Q&A.
2. Re-run `/wf-meta sync ship-plan-buildout` after `/wf implement` lands files — at that point the "✗ missing (planned)" entries should flip to "✓ exists" or surface real drift.
