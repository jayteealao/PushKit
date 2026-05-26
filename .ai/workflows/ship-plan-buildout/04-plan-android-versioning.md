---
schema: sdlc/v1
type: plan
slug: ship-plan-buildout
slice-slug: android-versioning
status: complete
stage-number: 4
created-at: "2026-05-26T12:44:00Z"
updated-at: "2026-05-26T12:44:00Z"
metric-files-to-touch: 2
metric-step-count: 5
has-blockers: false
revision-count: 0
stack-source: confirmed
tags:
  - android
  - gradle
  - versioning
refs:
  index: 00-index.md
  plan-index: 04-plan.md
  slice-def: 03-slice-android-versioning.md
  siblings:
    - 04-plan-commit-hygiene.md
    - 04-plan-nsis-installer.md
    - 04-plan-backend-version.md
    - 04-plan-release-orchestration.md   # not yet written
  implement: 05-implement-android-versioning.md
next-command: wf-implement
next-invocation: "/wf implement ship-plan-buildout android-versioning"
---

# Plan: android-versioning

## Current State

`android/app/build.gradle.kts` (77 lines) uses AGP 8.2.2 / Gradle 8.5 / Kotlin 1.9.22. The relevant `defaultConfig` block:

```kotlin
defaultConfig {
    applicationId = "com.pushkit.app"
    minSdk = 26
    targetSdk = 34
    versionCode = 1          // line 15
    versionName = "1.0"      // line 16
}
```

Both values are hard-coded literals. There is no property override mechanism, no use of `findProperty`, `providers.gradleProperty`, or `-P` flags anywhere in the Android build files. There are also zero existing test files in the Android project (`src/test/` does not exist).

The CI `android-build` job in `.github/workflows/ci.yml` calls `./gradlew assembleDebug lint testDebugUnitTest` with no `-P` flags. The `release.yml` workflow has no Android step at all (that is added by the `release-orchestration` slice).

No `android/README.md` or `android/app/README.md` exists.

## Reuse Opportunities

- No version injection pattern exists anywhere in the project — implement fresh.
- `providers.gradleProperty` is available natively in all Kotlin DSL `build.gradle.kts` files as an extension on `Project` — no new dependency or import required.
- Pattern is confirmed idiomatic for AGP 8.2.2 / Gradle 8.5 (lazy evaluation, configuration-cache-safe, clean null handling via `.orNull`).

## Likely Files / Areas to Touch

| File | Why |
|---|---|
| `android/app/build.gradle.kts` | Add `providers.gradleProperty` overrides for `versionCode` and `versionName` in `defaultConfig` |
| `android/README.md` | New — document local-dev override invocation and aapt validation |

No other files touched. CI workflow changes (`fetch-depth: 0`, `-P` flag injection) are owned by the `release-orchestration` slice.

## Proposed Change Strategy

Single-track, mechanical. Use the `providers.gradleProperty` API (modern, lazy, configuration-cache-safe for AGP 8.2.2 / Gradle 8.5) to read optional build-time properties with safe fallback defaults. CI passes overrides via `-P` flags; local dev builds with no flags get the unchanged defaults.

**Why `providers.gradleProperty` over `project.findProperty`:**
- Returns `Provider<String>` (lazy) vs `Any?` (eager). Avoids capturing values at configuration time.
- Configuration-cache-safe (Gradle 8.5's upcoming default; no extra work needed when cache is enabled).
- `.orNull` returns `String?` cleanly; no `?.toString()` dance on `Any?`.
- No deprecation warnings in AGP 8.2.2 / Gradle 8.5.

**Why `toIntOrNull()` not `toInt()`:**
If `-PversionCodeOverride` is accidentally passed without a value, Gradle sets the property to an empty string `""`. `toInt()` throws `NumberFormatException`; `toIntOrNull()` returns `null` and falls back to the default `1`.

**Why `versionCodeOverride` / `versionNameOverride` and not `versionCode` / `versionName`:**
`versionCode` and `versionName` are existing DSL property names on `DefaultConfig` and on the `Project` object. Using these as Gradle property names risks lookup ambiguity. The `Override` suffix is the community-standard safe choice.

## Step-by-Step Plan

### Step 1 — Update `android/app/build.gradle.kts`

Replace the two hard-coded lines inside `defaultConfig {}`:

**Before (lines 15–16):**
```kotlin
versionCode = 1
versionName = "1.0"
```

**After:**
```kotlin
versionCode = providers.gradleProperty("versionCodeOverride").orNull?.toIntOrNull() ?: 1
versionName = providers.gradleProperty("versionNameOverride").orNull ?: "1.0"
```

No other changes to the file. No new imports needed — `providers` is an extension on `Project` in all Kotlin DSL build scripts.

### Step 2 — Create `android/README.md`

New file documenting:
- How to build with version overrides (local dev / CI contract)
- The default fallback behavior
- How to verify the output with `aapt dump badging`

See the **README content** section at the end of this plan for the exact file content.

### Step 3 — Local validation with overrides

From the repo root:

```bash
cd android
./gradlew assembleDebug \
  -PversionCodeOverride=42 \
  -PversionNameOverride=0.1.0-rc.1
```

Then verify:

```bash
# From repo root:
# Linux/macOS:
aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk \
  | grep -E "versionCode|versionName"
# Windows (aapt is in $ANDROID_HOME/build-tools/<version>/aapt):
& "$env:ANDROID_HOME\build-tools\34.0.0\aapt.exe" dump badging \
  android\app\build\outputs\apk\debug\app-debug.apk | Select-String "versionCode|versionName"
```

**Expected output:**
```
package: name='com.pushkit.app' versionCode='42' versionName='0.1.0-rc.1' ...
```

### Step 4 — Local validation with defaults (preserve local-dev behavior)

```bash
cd android
./gradlew assembleDebug
aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk \
  | grep -E "versionCode|versionName"
```

**Expected output:**
```
package: name='com.pushkit.app' versionCode='1' versionName='1.0' ...
```

This confirms local dev builds are unaffected when no overrides are passed.

### Step 5 — Commit

Conventional commit: `feat(android): add versionCode/versionName Gradle property overrides`

This lands on `feat/ship-plan-buildout` after `backend-version`.

## CI Contract (for release-orchestration slice)

When the `release-orchestration` slice wires Android into `release.yml`, the build step MUST:

1. Use `fetch-depth: 0` in `actions/checkout` (required for `git rev-list --count HEAD` to return the full commit count, not `1`).
2. Pass the version properties to Gradle:

```bash
VERSION_CODE=$(git rev-list --first-parent --count HEAD)
VERSION_NAME="${GITHUB_REF_NAME#v}"   # strips leading 'v' from e.g. v0.1.0-rc.1
./gradlew assembleDebug \
  -PversionCodeOverride="$VERSION_CODE" \
  -PversionNameOverride="$VERSION_NAME"
```

3. The APK output path is `android/app/build/outputs/apk/debug/app-debug.apk`.

**`-P` vs `-D` gotcha (document in release-orchestration plan):** `-DversionCodeOverride=N` sets a JVM system property; `-PversionCodeOverride=N` sets a Gradle project property. `providers.gradleProperty` only reads the `-P` namespace. A `-D` flag is silently invisible and falls through to the default.

## Test / Verification Plan

### Automated checks

- **lint/typecheck:** `./gradlew lint` passes (no Kotlin type errors from the property change — `providers.gradleProperty(...).orNull?.toIntOrNull()` is fully type-safe).
- **build:** `./gradlew assembleDebug` passes with no overrides (default values compile correctly).
- **build with overrides:** `./gradlew assembleDebug -PversionCodeOverride=42 -PversionNameOverride=0.1.0-rc.1` passes without error.
- **unit tests:** No new unit tests added. The Android project has zero existing test infrastructure; adding test infra for a two-line build-config change would exceed the slice scope. The `testDebugUnitTest` Gradle task continues to report 0 tests (no-op, as before).

### Interactive verification (human-in-the-loop)

**AC1 — Override behavior confirmed via `aapt dump badging`**

- **What to verify:** Build with explicit overrides, inspect APK manifest, confirm versionCode and versionName match the passed values.
- **Platform & tool:** Developer machine with Android SDK. Stack: `platforms: [android]`, `testing: [android-lint]`. Tool chain: Gradle + `aapt` from Android SDK build-tools.
- **Steps:**
  1. `cd android && ./gradlew assembleDebug -PversionCodeOverride=42 -PversionNameOverride=0.1.0-rc.1`
  2. `aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk | grep -E "versionCode|versionName"`
  3. Confirm output: `versionCode='42'` and `versionName='0.1.0-rc.1'`
- **Evidence capture:** Paste terminal output into `06-verify-android-versioning.md`.
- **Pass criteria:** `versionCode='42'` and `versionName='0.1.0-rc.1'` appear in `aapt dump badging` output. Exit code 0 for both the Gradle build and the aapt command.

**AC2 — Default behavior preserved**

- **What to verify:** Build without overrides uses hardcoded defaults.
- **Steps:**
  1. `cd android && ./gradlew assembleDebug` (no `-P` flags)
  2. `aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk | grep -E "versionCode|versionName"`
  3. Confirm output: `versionCode='1'` and `versionName='1.0'`
- **Pass criteria:** `versionCode='1'` and `versionName='1.0'`. Local dev behavior unchanged.

## Risks / Watchouts

- **`-P` vs `-D` flag confusion.** The most common Gradle CI gotcha: `-DversionCodeOverride=N` (JVM system property) is silently ignored by `providers.gradleProperty`. The `release-orchestration` plan MUST document this and use `-P`. Flagged here as a cross-slice contract item.
- **`toIntOrNull()` handles edge cases.** If `-PversionCodeOverride=` is accidentally set without a value, the empty string falls through to `?: 1` rather than crashing the build. This is intentional defensive behavior.
- **Shallow clone breaks `git rev-list --count HEAD`.** `fetch-depth: 1` returns `1`, not the real commit count. The `release-orchestration` slice owns adding `fetch-depth: 0` to the release workflow's Android job. This slice's acceptance criteria are purely local (no CI involvement) so this risk is inherited, not owned.
- **AGP version lock.** `providers.gradleProperty` is not deprecated in AGP 8.2.2 / Gradle 8.5. If AGP is upgraded in the future, re-verify the provider API is still the recommended path (it is expected to remain stable through AGP 9.x per current roadmap).
- **aapt path on Windows.** `aapt` is in `$ANDROID_HOME/build-tools/<version>/aapt.exe`. The `android/README.md` documents both Linux and Windows invocations.

## Dependencies on Other Slices

- **Inbound:** `commit-hygiene` must land first so this slice's commits are Conventional Commits. ✅ Already implemented on the branch.
- **Outbound:** `release-orchestration` depends on this slice's property mechanism. The CI Android build step must pass `-PversionCodeOverride=$(git rev-list --count HEAD)` and `-PversionNameOverride=<tag-without-v>`. See **CI Contract** section above for the exact invocation.
- **No file overlap with any other slice.** `android/app/build.gradle.kts` and `android/README.md` are not touched by any other slice.

## Assumptions

- Android SDK is installed locally (required for `./gradlew assembleDebug` and `aapt dump badging` validation).
- `ANDROID_HOME` is set correctly on the developer's machine.
- `aapt` from Android SDK build-tools 34.0.0 (matching `compileSdk = 34`) is accessible on PATH or via the full `$ANDROID_HOME/build-tools/34.0.0/aapt` path.
- Gradle 8.5 + AGP 8.2.2 (confirmed from project files): `providers.gradleProperty` is available and non-deprecated at these versions.

## Blockers

None.

## Freshness Research

### Source: Gradle Documentation — Properties and Providers (docs.gradle.org)

**Relevance:** API selection for `providers.gradleProperty` vs `project.findProperty`.
**Takeaway:** `providers.gradleProperty("key")` is the official recommended lazy API for build configuration (Gradle 8.x+). Returns `Provider<String>`. Configuration-cache-safe. `project.findProperty` is the legacy eager API; still works but not configuration-cache-safe. No deprecation warning in Gradle 8.5, but the roadmap steers toward `providers` for all configuration-cache-enabled builds. This project uses Gradle 8.5, which supports the provider API without issues.

### Source: Gradle Build Environment docs + community

**Relevance:** `-P` vs `-D` flag disambiguation.
**Takeaway:** `-PversionCodeOverride=N` sets a Gradle project property (read by `providers.gradleProperty`). `-DversionCodeOverride=N` sets a JVM system property (invisible to `findProperty` and `providers.gradleProperty`). This is the #1 source of silent CI breakage for Gradle property injection.

### Source: Kotlin `String.toIntOrNull()` vs `toInt()` — Kotlin stdlib

**Relevance:** Safe type coercion of string-typed Gradle properties to Int.
**Takeaway:** `toIntOrNull()` returns `null` on empty string or non-numeric input; `toInt()` throws `NumberFormatException`. Use `toIntOrNull()` everywhere Gradle property strings are converted to Int to protect against the empty-string edge case (Gradle sets an unvalued `-P` argument to `""`).

### Source: Google Issue Tracker #242730594 — `android.injected.version.code`

**Relevance:** Considered using the AGP-internal injection property instead of a custom one.
**Takeaway:** `android.injected.version.code` / `android.injected.version.name` are unstable internal AGP APIs used by Fastlane and Android Studio. They do not work with configuration cache enabled. Use a custom `-PversionCodeOverride` instead (community-standard safe choice that avoids DSL name shadowing).

### Source: GitHub Actions Android CI/CD patterns (2024–2025)

**Relevance:** Confirming `-P` flag injection is the dominant CI pattern.
**Takeaway:** Passing override properties via `./gradlew <task> -PversionCodeOverride=$VERSION_CODE` is the dominant pattern across community Android CI guides. Writing to `gradle.properties` at CI time is a common secondary pattern; `sed`/inline file edit is considered fragile and platform-specific (sed syntax differs between macOS and Linux, fails on Windows without Git Bash). Gradle property injection is the idiomatic choice.

---

## android/README.md content

```markdown
# Android

## Building

```bash
# Default build (uses hardcoded versionCode=1, versionName="1.0")
cd android
./gradlew assembleDebug

# Build with version overrides (for testing CI behavior locally)
./gradlew assembleDebug \
  -PversionCodeOverride=42 \
  -PversionNameOverride=0.1.0-rc.1
```

The `versionCodeOverride` and `versionNameOverride` Gradle properties are optional. When unset, the build uses the hardcoded defaults (`versionCode = 1`, `versionName = "1.0"`). Local dev builds require no flags.

## Verifying version metadata

After building, inspect the APK manifest with `aapt` from the Android SDK build-tools:

**Linux/macOS:**
```bash
aapt dump badging android/app/build/outputs/apk/debug/app-debug.apk \
  | grep -E "versionCode|versionName"
```

**Windows (PowerShell):**
```powershell
& "$env:ANDROID_HOME\build-tools\34.0.0\aapt.exe" dump badging `
  android\app\build\outputs\apk\debug\app-debug.apk | Select-String "versionCode|versionName"
```

Expected with overrides: `versionCode='42' versionName='0.1.0-rc.1'`
Expected without overrides: `versionCode='1' versionName='1.0'`

## CI integration

In `release.yml`, the Android build step passes:
```
-PversionCodeOverride=$(git rev-list --first-parent --count HEAD)
-PversionNameOverride=${GITHUB_REF_NAME#v}
```

Full-depth checkout (`fetch-depth: 0`) is required for an accurate commit count.
```
```

## Revision History

*(none — first revision)*

## Recommended Next Stage

- **Option A (default):** `/wf implement ship-plan-buildout android-versioning` — plan is execution-ready; two files, two property lines, one README. Zero blockers. Consider running `/compact` first to clear planning context.
- **Option B:** `/wf plan ship-plan-buildout release-orchestration` — plan the final integrator slice before implementing. Useful if the maintainer wants all five slice plans complete before any further implementation.
