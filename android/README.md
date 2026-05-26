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

```bash
-PversionCodeOverride=$(git rev-list --first-parent --count HEAD)
-PversionNameOverride=${GITHUB_REF_NAME#v}
```

Full-depth checkout (`fetch-depth: 0`) is required for an accurate commit count.

> **Note:** Use `-P` (Gradle project property), not `-D` (JVM system property). `-D` flags are silently ignored by `providers.gradleProperty`.
