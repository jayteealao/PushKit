---
schema: sdlc/v1
type: implement-index
slug: ship-plan-buildout
status: complete
stage-number: 5
created-at: "2026-05-25T11:54:43Z"
updated-at: "2026-05-27T18:24:55Z"
slices-implemented: 5
slices-total: 5
metric-total-files-changed: 20
metric-total-lines-added: 1512
metric-total-lines-removed: 17
tags:
  - ci-cd
  - lefthook
  - commitlint
  - github-actions
  - release
  - git-cliff
  - nsis
refs:
  index: 00-index.md
  plan-index: 04-plan.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout release-orchestration"
---

# Implement Index

## Slices

| Slice | Status | Artifact |
|-------|--------|----------|
| commit-hygiene | complete | [05-implement-commit-hygiene.md](05-implement-commit-hygiene.md) |
| backend-version | complete | [05-implement-backend-version.md](05-implement-backend-version.md) |
| nsis-installer | complete | [05-implement-nsis-installer.md](05-implement-nsis-installer.md) |
| android-versioning | complete | [05-implement-android-versioning.md](05-implement-android-versioning.md) |
| release-orchestration | complete | [05-implement-release-orchestration.md](05-implement-release-orchestration.md) |

## Cross-Slice Integration Notes

- `README.md` is a shared file. `commit-hygiene` added `## Development setup`. `release-orchestration` adds the shields.io badge at the top and appends `## Releasing` + `## Backend installer` after `## Testing`. No collisions across slices.
- `.github/workflows/ci.yml` (added by `commit-hygiene`) and `.github/workflows/release.yml` (rewritten by `release-orchestration`) are separate files — no conflict.
- `android/app/build.gradle.kts` was updated by `android-versioning` (providers.gradleProperty overrides). `release-orchestration` consumes these in `build-android-apk` via `-PversionCodeOverride` / `-PversionNameOverride` (project properties, not system properties).
- `backend/cmd/server/main.go` was updated by `backend-version` (`var Version` + `--version` flag + `printVersion`). `release-orchestration` consumes this via `-ldflags "-X main.Version=<stripped>"` in `build-backend-binary` and asserts `pushkit-server <ver>` in the `post-publish-windows` smoke-test.
- `backend/installer/pushkit.nsi` was added by `nsis-installer` (full lifecycle installer + `!ifndef VERSION` fallback). `release-orchestration` consumes via `makensis /DVERSION=<stripped>` in `build-backend-installer` and silently installs + smoke-tests the result.

## Recommended Next Stage

- **Option A (default):** `/wf verify ship-plan-buildout release-orchestration` — verify the integrator slice. Most of the AC verification is interactive (real tag push to validate the pipeline end-to-end) and human-in-the-loop (release notes inspection, shields.io badge cache).
- **Option B:** `/wf review ship-plan-buildout` — slug-wide review (per `review-scope: slug-wide` in `00-index.md`) before cutting the validation tag. With all five slices now implemented, a cumulative branch diff review is the cheapest place to catch cross-slice integration issues.
