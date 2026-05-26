---
schema: sdlc/v1
type: implement-index
slug: ship-plan-buildout
status: in-progress
stage-number: 5
created-at: "2026-05-25T11:54:43Z"
updated-at: "2026-05-26T12:55:33Z"
slices-implemented: 4
slices-total: 5
metric-total-files-changed: 17
metric-total-lines-added: 935
metric-total-lines-removed: 2
tags:
  - ci-cd
  - lefthook
  - commitlint
  - github-actions
refs:
  index: 00-index.md
  plan-index: 04-plan.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout commit-hygiene"
---

# Implement Index

## Slices

| Slice | Status | Artifact |
|-------|--------|----------|
| commit-hygiene | complete | [05-implement-commit-hygiene.md](05-implement-commit-hygiene.md) |
| backend-version | complete | [05-implement-backend-version.md](05-implement-backend-version.md) |
| nsis-installer | complete | [05-implement-nsis-installer.md](05-implement-nsis-installer.md) |
| android-versioning | complete | [05-implement-android-versioning.md](05-implement-android-versioning.md) |
| release-orchestration | not-started | — |

## Cross-Slice Integration Notes

- `README.md` is a shared file. `commit-hygiene` added `## Development setup`. Sibling slices (`backend-version`, `nsis-installer`, `release-orchestration`) will add their own README sections (`## Backend installer`, `## Releasing`, shields.io badge). Each slice is responsible for its own section; no coordination needed until `release-orchestration` adds the badge at the top.
- `.github/workflows/ci.yml` (added by `commit-hygiene`) and `.github/workflows/release.yml` (modified by `release-orchestration`) are separate files — no conflict.
- `android/app/build.gradle.kts` was updated by `android-versioning` (providers.gradleProperty overrides). `android/README.md` is exclusively owned by `android-versioning`. No other slice touches these files.

## Recommended Next Stage

- **Option A (default):** `/wf verify ship-plan-buildout commit-hygiene` — verify the commit-hygiene slice before proceeding to the next slice.
- **Option B:** `/wf plan ship-plan-buildout nsis-installer` — plan the next slice in parallel while the PR is in review.
