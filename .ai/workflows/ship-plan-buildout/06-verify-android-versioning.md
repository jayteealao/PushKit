---
schema: sdlc/v1
type: verify
slug: ship-plan-buildout
slice-slug: android-versioning
status: complete
stage-number: 6
created-at: "2026-05-26T15:51:15Z"
updated-at: "2026-05-26T15:51:15Z"
result: pass
metric-checks-run: 3
metric-checks-passed: 3
metric-acceptance-met: 3
metric-acceptance-total: 3
metric-acceptance-user-observable: 2
metric-acceptance-code-only: 1
metric-interactive-checks-run: 2
metric-interactive-checks-passed: 2
metric-issues-found: 0
metric-issues-found-initial: 0
metric-issues-found-final: 0
fix-rounds-run: 0
convergence: not-needed
verify-owned-fix-commit: null
interactive-verification: required
interactive-verification-defer-reason: ""
adapters-used: [android]
bootstrap-failures: []
stack-source: confirmed
evidence-dir: ".ai/workflows/ship-plan-buildout/verify-evidence/android-versioning/"
tags:
  - android
  - gradle
  - versioning
refs:
  index: 00-index.md
  verify-index: 06-verify.md
  slice-def: 03-slice-android-versioning.md
  plan: 04-plan-android-versioning.md
  implement: 05-implement-android-versioning.md
  review: 07-review-android-versioning.md
  adapters: ${CLAUDE_PLUGIN_ROOT}/skills/wf/reference/runtime-adapters.md
next-command: wf-review
next-invocation: "/wf review ship-plan-buildout"
---

# Verify: android-versioning

## Verification Summary

All 3 checks passed. Both user-observable acceptance criteria confirmed via interactive `aapt dump badging` inspection of the built APK. No issues found; convergence not needed.

- **Branch:** `feat/ship-plan-buildout` (confirmed — correct dedicated branch)
- **Commit verified:** `8aefb9c` (feat(android): add versionCode/versionName Gradle property overrides)
- **Android SDK:** `/c/Users/jayte/AppData/Local/Android/Sdk/build-tools/34.0.0/aapt`

## Automated Checks Run

| # | Check | Command | Result |
|---|-------|---------|--------|
| 1 | Lint + unit tests | `./gradlew lint testDebugUnitTest --no-daemon` | PASS — `BUILD SUCCESSFUL` (1m 28s, 27 tasks, 10 executed) |
| 2 | Build with defaults | `./gradlew assembleDebug --no-daemon` | PASS — `BUILD SUCCESSFUL` (50s, 35 tasks) |
| 3 | Build with overrides | `./gradlew assembleDebug -PversionCodeOverride=42 -PversionNameOverride=0.1.0-rc.1 --no-daemon` | PASS — `BUILD SUCCESSFUL` (29s, 35 tasks, 13 executed) |

Unit tests: `testDebugUnitTest NO-SOURCE` — zero test files in the Android project; consistent with the plan noting zero existing test infrastructure. Not a new failure.

## Interactive Verification Results

**AC1 — Override build + aapt validation**

- **Criterion:** `./gradlew assembleDebug -PversionNameOverride=0.1.0-rc.1 -PversionCodeOverride=42` → `aapt dump badging` reports `versionName='0.1.0-rc.1'` and `versionCode='42'`
- **Platform & tool:** Android SDK 34.0.0 `aapt` on developer machine (`feat/ship-plan-buildout` branch)
- **Steps performed:**
  1. Ran `./gradlew assembleDebug -PversionCodeOverride=42 -PversionNameOverride=0.1.0-rc.1 --no-daemon` — BUILD SUCCESSFUL (29s)
  2. Ran `aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk | grep -E "package:|versionCode|versionName"`
- **Evidence (terminal output):**
  ```
  package: name='com.pushkit.app' versionCode='42' versionName='0.1.0-rc.1' platformBuildVersionName='14' platformBuildVersionCode='34' compileSdkVersion='34' compileSdkVersionCodename='14'
  ```
- **Observation:** `versionCode='42'` and `versionName='0.1.0-rc.1'` present exactly as specified.
- **Result:** PASS

---

**AC2 — Default build + aapt validation**

- **Criterion:** `./gradlew assembleDebug` (no overrides) → `aapt dump badging` reports `versionName='1.0'` and `versionCode='1'`
- **Platform & tool:** Android SDK 34.0.0 `aapt`
- **Steps performed:**
  1. Ran `./gradlew assembleDebug --no-daemon` — BUILD SUCCESSFUL (50s)
  2. Ran `aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk | grep -E "package:|versionCode|versionName"`
- **Evidence (terminal output):**
  ```
  package: name='com.pushkit.app' versionCode='1' versionName='1.0' platformBuildVersionName='14' platformBuildVersionCode='34' compileSdkVersion='34' compileSdkVersionCodename='14'
  ```
- **Observation:** `versionCode='1'` and `versionName='1.0'` — hardcoded defaults preserved exactly.
- **Result:** PASS

## Acceptance Criteria Status

| # | Criterion (summary) | Kind | Status | Method | Evidence |
|---|---|---|---|---|---|
| AC1 | Override build → aapt shows `versionCode='42'` and `versionName='0.1.0-rc.1'` | user-observable | met | interactive — aapt dump badging | terminal output above |
| AC2 | Default build → aapt shows `versionCode='1'` and `versionName='1.0'` | user-observable | met | interactive — aapt dump badging | terminal output above |
| AC3 | Property mechanism exists so release-orchestration can pass `-PversionCodeOverride` for monotonic CI versionCode | code-only | met | static — `build.gradle.kts:15` read confirms `providers.gradleProperty("versionCodeOverride").orNull?.toIntOrNull() ?: 1` is wired | code inspection + AC1 build confirms the property channel works end-to-end |

## Issues Found

None.

## Gaps / Unverified Areas

- **AC3 end-to-end CI path** is owned by the `release-orchestration` slice. The mechanism exists and works locally (AC1 confirms the `-P` channel is live); the full-depth checkout (`fetch-depth: 0`) and `git rev-list --count HEAD` wiring remain to be verified when that slice is implemented.

## Freshness Research

No freshness pass required — this is a two-line build-config change with no external API calls or version-sensitive dependencies. `providers.gradleProperty` is non-deprecated in AGP 8.2.2 / Gradle 8.5 (confirmed at plan time 2026-05-26; no new AGP releases in the intervening hours require a re-check).

## Recommendation

All acceptance criteria met. No issues found. Ready for review.

`review-scope` in `00-index.md` is `slug-wide` — review runs across the full branch diff (`git diff main...HEAD`), not per-slice. Invoke `/wf review ship-plan-buildout` to dispatch the slug-wide review.

## Recommended Next Stage

- **Option A (default):** `/wf review ship-plan-buildout` — convergence not-needed, result pass; ready for slug-wide review across all completed slices
- **Option D:** `/wf handoff ship-plan-buildout` — skip review (solo project / trivial change); only valid given `result: pass`
