---
schema: sdlc/v1
type: implement
slug: ship-plan-buildout
slice-slug: android-versioning
status: complete
stage-number: 5
created-at: "2026-05-26T12:55:33Z"
updated-at: "2026-05-26T12:55:33Z"
metric-files-changed: 2
metric-lines-added: 33
metric-lines-removed: 2
metric-deviations-from-plan: 0
metric-review-fixes-applied: 0
commit-sha: "8aefb9c"
tags:
  - android
  - gradle
  - versioning
refs:
  index: 00-index.md
  implement-index: 05-implement.md
  slice-def: 03-slice-android-versioning.md
  plan: 04-plan-android-versioning.md
  siblings:
    - 05-implement-commit-hygiene.md
    - 05-implement-nsis-installer.md
    - 05-implement-backend-version.md
  verify: 06-verify-android-versioning.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout android-versioning"
---

# Implement: android-versioning

## Summary of Changes

Two-file mechanical change: `android/app/build.gradle.kts` gains `providers.gradleProperty` overrides for `versionCode` and `versionName` with safe fallback defaults; `android/README.md` is created documenting local dev and CI usage.

## Files Changed

- `android/app/build.gradle.kts`: Replaced hardcoded `versionCode = 1` and `versionName = "1.0"` with lazy provider lookups — `providers.gradleProperty("versionCodeOverride").orNull?.toIntOrNull() ?: 1` and `providers.gradleProperty("versionNameOverride").orNull ?: "1.0"`. No other lines changed.
- `android/README.md` (new): Documents default build, override invocation, aapt verification (Linux/macOS and Windows), and CI integration contract including the `-P` vs `-D` gotcha note.

## Shared Files (also touched by sibling slices)

None — both files are exclusively owned by this slice.

## Notes on Design Choices

- Used `providers.gradleProperty` (not `project.findProperty`) — lazy, configuration-cache-safe for Gradle 8.5, returns `Provider<String>` cleanly.
- Used `toIntOrNull()` (not `toInt()`) — guards against the empty-string edge case when `-PversionCodeOverride=` is accidentally passed without a value.
- Property names `versionCodeOverride` / `versionNameOverride` (not `versionCode` / `versionName`) avoid DSL name shadowing ambiguity on `DefaultConfig` and `Project`.
- Added a `-P` vs `-D` note to the README to prevent the #1 silent CI footgun.

## Deviations from Plan

None.

## Anything Deferred

- Wiring `-PversionCodeOverride`/`-PversionNameOverride` into `release.yml` — `release-orchestration` slice.
- `fetch-depth: 0` in CI checkout — `release-orchestration` slice.
- AC1/AC2 `aapt dump badging` interactive validation — `06-verify-android-versioning.md` (requires Android SDK on developer machine).

## Known Risks / Caveats

- `git rev-list --count HEAD` in CI requires `fetch-depth: 0` — shallow clone returns `1`. The `release-orchestration` slice owns this.
- APK install conflict: if a maintainer installs an APK with a higher `versionCode` from a manual test build, Android will block reinstall of a lower CI-built APK. Mitigation: `adb uninstall com.pushkit.app` between manual tests.
- `providers.gradleProperty` is non-deprecated at AGP 8.2.2 / Gradle 8.5. Re-verify on AGP upgrade.

## Freshness Research

Covered in `04-plan-android-versioning.md` — `providers.gradleProperty` confirmed idiomatic for AGP 8.2.2 / Gradle 8.5 at plan time (2026-05-26). No new external state to verify for this two-line build-config change.

## Recommended Next Stage

- **Option A (default):** `/wf verify ship-plan-buildout android-versioning` — run `./gradlew lint` and interactive AC1/AC2 `aapt dump badging` validation on a machine with Android SDK.
- **Option B:** `/wf review ship-plan-buildout` — skip verify and proceed to slug-wide review (only viable if maintainer accepts deferred AC1/AC2 at the verify stage).
