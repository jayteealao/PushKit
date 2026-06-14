# PushKit Installer — Maintainer Reference

This file is the reference for maintainers building or modifying the Windows installer. It is **not** end-user documentation — end-user installer docs belong in the root `README.md`.

## Requirements

- **NSIS 3.12 or later** (CVE-2025-43715 was fixed in 3.11; 3.12 adds a second elevation fix).  
  Install from the official NSIS installer; `makensis.exe` ends up at `%PROGRAMFILES(X86)%\NSIS\makensis.exe`.
- **Go 1.25.** Declared in `backend/go.mod` and used by CI to build every release binary. Required to build the input binary.
- **Windows 11** (or Windows Server 2022 for CI). The installer itself targets any 64-bit Windows ≥ 10.

## Required defines

| Define | Default | Notes |
|--------|---------|-------|
| `VERSION` | `0.0.0-dev` | Set at compile time: `makensis /DVERSION=1.2.3`. Omitting it produces a compilable installer with wrong version metadata. |

## Components

| Section | ID | Behaviour |
|---------|-----|-----------|
| PushKit Server *(required)* | `SecCore` | `SectionIn RO` — cannot be unchecked. Copies `pushkit-server.exe`, creates Start Menu shortcuts, writes Apps & Features registry entry, writes uninstaller. |
| Register Windows service *(optional, default-checked)* | `SecService` | Registers `PushKitServer` as `SERVICE_WIN32_OWN_PROCESS`, `SERVICE_DEMAND_START`. On upgrade, restarts the service after binary replace. |

## Silent install (`/S`)

Because `SecService` is **default-checked**, a silent install (`/S`) **will** register the Windows service. To install the binary only without service registration, the user must run the interactive installer and uncheck the service component. A future iteration could add an `IfSilent` guard to `SecService` to change this behaviour.

## Upgrade detection

`.onInit` calls `SimpleSC::ExistsService "${SERVICE_NAME}"`. If the service exists, `$WasInstalled` is set to `"1"`. The install section then:

1. Calls `SimpleSC::StopService` with a 30-second poll loop (waits for the process to release file handles before the `File` directive overwrites the binary).
2. Copies the new binary.
3. After `SecCore` completes, `SecService` calls `SimpleSC::StartService` to restart.

**Gotcha**: `SimpleSC::ExistsService` returns `0` when the service **exists** and non-zero when it does **not**. This inversion is documented on the NSIS Wiki SimpleSC page. The script uses `${If} $0 == 0` accordingly.

## Local validation steps

Run from the **repository root**:

> **Note:** this local build omits the version `-ldflags` that CI uses, so the
> resulting `pushkit-server.exe --version` prints the default (`dev`). To mirror
> the release binary, add `-ldflags "-X main.Version=<version>"`. See
> [How CI builds the installer](#how-ci-builds-the-installer) below.

```powershell
# 1. Build the binary
go build -o backend/pushkit-server.exe ./backend/cmd/server

# 2. Compile the installer (requires NSIS 3.12 in PATH or use the full path)
& "C:\Program Files (x86)\NSIS\makensis.exe" /V3 /DVERSION=0.0.1-dev backend/installer/pushkit.nsi

# 3a. Interactive install
.\backend\installer\pushkit-server-setup.exe

# 3b. Silent install (from an already-elevated PowerShell)
.\backend\installer\pushkit-server-setup.exe /S

# 4. Verify service registration
sc query PushKitServer
# Expect: STATE : 1  STOPPED,  START_TYPE : 3  DEMAND_START

# 5. Check install directory
Test-Path "$env:ProgramFiles\PushKit\pushkit-server.exe"   # → True

# 6. Silent uninstall
& "$env:ProgramFiles\PushKit\uninstall.exe" /S
sc query PushKitServer   # → [SC] OpenService FAILED 1060 (not found)
```

### Upgrade validation

```powershell
# After a v1 install with service registered:
sc start PushKitServer
sc query PushKitServer   # → STATE : 4  RUNNING

# Rebuild with a new version and re-run installer:
go build -o backend/pushkit-server.exe ./backend/cmd/server
& "C:\Program Files (x86)\NSIS\makensis.exe" /V3 /DVERSION=0.0.2-dev backend/installer/pushkit.nsi
.\backend\installer\pushkit-server-setup.exe   # or /S

# Post-upgrade checks:
sc query PushKitServer   # → STATE : 4  RUNNING (auto-restarted)
Get-Item "$env:ProgramFiles\PushKit\pushkit-server.exe" | Select LastWriteTime
Get-ChildItem "$env:ProgramFiles\PushKit"   # → only pushkit-server.exe + uninstall.exe
```

### Installer size check

```powershell
Get-Item backend\installer\pushkit-server-setup.exe | Select-Object Length
# Target: < 25 MB. LZMA solid compression on the Go binary typically lands 17–20 MB.
# If > 25 MB, flag in handoff — do not block the release.
```

## Vendored plugin

`plugins/SimpleSC.dll` — Unicode build, version 1.30.

| Field | Value |
|-------|-------|
| Source | [NSIS Wiki SimpleSC](https://nsis.sourceforge.io/NSIS_Simple_Service_Plugin) |
| Download URL | `https://nsis.sourceforge.io/mediawiki/images/e/ef/NSIS_Simple_Service_Plugin_Unicode_1.30.zip` |
| SHA-256 | `1620CDF739F459D1D83411F93648F29DCF947A910CC761E85AC79A69639D127D` |
| Size | 1,110,016 bytes |
| Last release | 2021-05-02 |

To re-vendor: download the zip above, extract `SimpleSC.dll` from the archive root (the Unicode-only zip has the DLL at root level), replace `plugins/SimpleSC.dll`, update the SHA-256 in this table.

## How CI builds the installer

The installer is produced by two chained jobs in `.github/workflows/release.yml`:

1. **`build-backend-binary`** (Linux) cross-compiles the server with
   `GOOS=windows GOARCH=amd64 CGO_ENABLED=0` and
   `-ldflags "-X main.Version=<version>"`, baking the release version into the
   binary, then uploads `pushkit-server.exe` as an artifact.
2. **`build-backend-installer`** (`windows-2022`) downloads that artifact,
   installs NSIS 3.12, verifies the bundled `SimpleSC.dll` SHA-256 against the
   value in the [Vendored plugin](#vendored-plugin) table, and runs
   `makensis /DVERSION=<version>`.

So the `.exe` that ships is always cross-compiled on Linux with its version
injected at link time — the local build command above is for installer-logic
testing only, not for producing a release artifact.

## Out of scope

- PyPI CLI packaging — `cli/` directory, separate packaging story.
- Android — `android/` directory.
- Windows code-signing — deferred to post-v0.x; the installer is unsigned and will trigger a SmartScreen warning on first run (documented in the root README's installer section).
- Wiring `makensis` into the GitHub Actions release workflow — handled by the release workflow (`.github/workflows/release.yml`).
