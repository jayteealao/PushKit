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

The `build-android-apk` job in `release.yml` computes the version in two steps rather than one inline command:

1. A **Compute version overrides** step derives the values and writes them to `$GITHUB_OUTPUT`:
   ```bash
   VERSION_CODE=$(git rev-list --first-parent --count HEAD)
   echo "VERSION_CODE=$VERSION_CODE" >> "$GITHUB_OUTPUT"
   echo "VERSION_NAME=$STRIPPED"     >> "$GITHUB_OUTPUT"   # STRIPPED = tag without the leading v
   ```
2. A **Build APK** step injects those outputs via an `env:` block and passes them to Gradle:
   ```bash
   ./gradlew assembleDebug \
     -PversionCodeOverride="$VERSION_CODE" \
     -PversionNameOverride="$VERSION_NAME"
   ```

The leading `v` is stripped once, upstream, by the canonical `version` job — the Android job consumes `needs.version.outputs.stripped` rather than re-stripping `GITHUB_REF_NAME` itself. Full-depth checkout (`fetch-depth: 0`) is required for an accurate commit count.

> **Note:** Use `-P` (Gradle project property), not `-D` (JVM system property). `-D` flags are silently ignored by `providers.gradleProperty`.

## App overview

The app is a small MVVM Jetpack Compose client for the PushKit backend (`minSdk = 26`, `compileSdk`/`targetSdk = 34`, Kotlin + Compose + Retrofit).

### Pointing the app at a backend

There is no compile-time backend URL. On launch, `AppNavigation` checks `CredentialStore.isConfigured` and routes to the **Settings** screen when no URL/key is stored, otherwise to the **file list**. In Settings you enter:

- **API URL** — e.g. `http://10.0.2.2:8000` for a local backend on the emulator host, or your deployed server's base URL. A trailing slash is added automatically by `RetrofitProvider`, and the stored value has its trailing slash trimmed.
- **API key** — one of the backend's `API_KEYS` values. It is sent on every request as the `X-API-Key` header by `ApiKeyInterceptor`.

Credentials are stored in `EncryptedSharedPreferences` (`pushkit_credentials`), not plain prefs.

### Architecture

| Layer | Type | Responsibility |
|-------|------|----------------|
| `ui/settings/` | `SettingsScreen` + `SettingsViewModel` | First-run / re-config of API URL and key |
| `ui/files/` | `FileListScreen` + `FileListViewModel` | List files; trigger downloads via the system `DownloadManager` |
| `ui/navigation/` | `AppNavigation` | `NavHost` with `settings` / `file_list` routes; picks the start destination from `isConfigured` |
| `data/` | `FileRepository`, `CredentialStore` | Domain access over the API; encrypted credential storage |
| `data/api/` | `PushKitApi`, `RetrofitProvider`, `ApiKeyInterceptor` | Retrofit interface, client wiring, auth header injection |

The app uses two of the backend's endpoints: `GET v1/files` (list, with `cursor`/`limit`/`q`/`sort`/`order` query params) and `GET v1/files/{fileId}/download` (presigned GET URL). Downloads are handed to Android's `DownloadManager`.

### Adding a screen

1. Add a route constant to `Routes` in `AppNavigation.kt`.
2. Add a `composable(Routes.X) { ... }` block to the `NavHost`, constructing the screen's `ViewModel` with `remember`.
3. Build the screen as a `@Composable` that takes its `ViewModel` and navigation callbacks (follow `FileListScreen` / `SettingsScreen`).

### Release builds

CI ships the **debug** APK (`assembleDebug`). The `release` build type exists with `isMinifyEnabled = false` and is **unsigned** — there is no signing config yet, so `assembleRelease` produces an unsigned artifact. Signing is out of scope for the v0.x line.
