---
schema: sdlc/v1
type: slice
slug: ship-plan-buildout
slice-slug: android-versioning
status: defined
stage-number: 3
created-at: "2026-05-22T22:46:55Z"
updated-at: "2026-05-22T22:46:55Z"
complexity: s
depends-on:
  - commit-hygiene
tags:
  - android
  - gradle
  - versioning
refs:
  index: 00-index.md
  slice-index: 03-slice.md
  siblings:
    - 03-slice-commit-hygiene.md
    - 03-slice-nsis-installer.md
    - 03-slice-backend-version.md
    - 03-slice-release-orchestration.md
  plan: 04-plan-android-versioning.md
  implement: 05-implement-android-versioning.md
---

# Slice: android-versioning

## Goal

The Android APK built in CI carries `versionName = <tag-without-v>` (e.g., `0.1.0-rc.1`) and `versionCode = git rev-list --count HEAD` (full-depth checkout required). The rewrite happens at build time and is NOT committed back to `android/app/build.gradle.kts`. A small, well-scoped mechanism (Gradle property OR an inline `sed` invoked from CI) drives the rewrite so local dev builds continue to use the hardcoded `versionName = "1.0"`/`versionCode = 1` defaults.

## Why This Slice Exists

Multi-source-of-truth versioning is a classic Android footgun. The plan must pick exactly one mechanism — either:

- (a) Gradle reads `-PversionName` and `-PversionCode` properties at the top of `build.gradle.kts` with fallback defaults, OR
- (b) CI runs `sed -i` (or PowerShell `(Get-Content) -replace`) on the two lines before `./gradlew assembleDebug`.

The freshness research notes industry convention favors (a) but the simplicity of (b) is tempting for a single tag-driven pipeline. Plan stage picks one.

This slice exists separately from `release-orchestration` because the Gradle mechanism (option a) is a build-config change that should be tested independently — a broken Gradle property handler would silently produce APKs with `versionCode = 1` forever.

`complexity: s` — single file change in `android/app/build.gradle.kts` (option a) or a CI step (option b), plus a small validation script that `aapt dump badging` confirms versionName/versionCode after a local build.

## Scope

### In

- Update `android/app/build.gradle.kts` to support per-build version overrides:
  - Read `versionNameOverride` and `versionCodeOverride` from Gradle properties (`project.findProperty(...)`); fall back to hardcoded defaults when unset.
  - Defaults stay `versionName = "1.0"` / `versionCode = 1` for local dev (preserves current behavior).
- Document the local-dev invocation in `android/app/README.md` (or `android/README.md`): `./gradlew assembleDebug -PversionNameOverride=0.1.0-test -PversionCodeOverride=42`.
- Local validation: build a debug APK with overrides, run `aapt dump badging app-debug.apk`, confirm `versionName='0.1.0-test'` and `versionCode='42'`.

### Out (handled by other slices)

- Wiring the version derivation into `release.yml` (passing `git rev-list --count HEAD` as `-PversionCodeOverride=$COUNT` to Gradle, and the tag-stripped string as `-PversionNameOverride=$NAME`) — `release-orchestration` slice.
- Full-depth checkout (`fetch-depth: 0`) in CI — `release-orchestration` slice (it's a CI workflow concern).
- Android release signing — out of scope for v0.x.

## Acceptance Criteria

- **Given** a local invocation `./gradlew assembleDebug -PversionNameOverride=0.1.0-rc.1 -PversionCodeOverride=42`, **when** the build completes, **then** `aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk` reports `versionName='0.1.0-rc.1'` and `versionCode='42'`. *(AC8 partial from shape — automated locally.)*
- **Given** a local invocation `./gradlew assembleDebug` (no overrides), **when** the build completes, **then** `aapt dump badging` reports the hardcoded defaults (`versionName='1.0'`, `versionCode='1'`). Preserves local-dev behavior.
- **Given** the property mechanism is in place, **when** the `release-orchestration` slice later wires `-PversionCodeOverride=$(git rev-list --count HEAD)` into the CI job, **then** the CI APK carries a monotonic versionCode matching the git commit count at the tagged ref.

## Dependencies on Other Slices

- **`commit-hygiene`** — commits must be conventional.

The `release-orchestration` slice depends on this slice's property mechanism existing.

## Risks

- **Gradle property name collisions.** `versionName` and `versionCode` are Android-DSL identifiers; `versionNameOverride` / `versionCodeOverride` are safer to avoid surprising shadowing.
- **Property type coercion.** Gradle properties arrive as strings; `versionCode` is an Int. Plan stage will spec the `.toInt()` conversion + a clear error if the property is set but not numeric.
- **Shallow clone in CI.** `git rev-list --count HEAD` requires full-depth. The `release-orchestration` slice owns the `fetch-depth: 0` decision but this slice's AC depends on it for the end-to-end test.
- **Monotonicity guarantee.** `git rev-list --count HEAD` is monotonic only if history is never rewritten on `main`. Documented as a constraint on the maintainer (no force-push to main).
- **APK install conflict.** If a maintainer installs an APK with `versionCode = 9999` from a local test build, then later installs a CI-built APK with `versionCode = 8` (current commit count), Android will refuse to install (lower versionCode). Documented; mitigation is `adb uninstall com.pushkit.app` between manual tests.
- **versionName format quirks.** PEP-440-normalized `0.1.0rc1` differs from Android `versionName = "0.1.0-rc.1"`. They're allowed to differ — PyPI is one system, Android is another. The CI passes the raw tag-without-v string to Gradle, hyphens and all.
