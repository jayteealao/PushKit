---
schema: sdlc/v1
type: slice
slug: ship-plan-buildout
slice-slug: nsis-installer
status: implemented
stage-number: 3
created-at: "2026-05-22T22:46:55Z"
updated-at: "2026-05-22T22:46:55Z"
complexity: l
depends-on: []
tags:
  - nsis
  - windows
  - installer
  - service-registration
refs:
  index: 00-index.md
  slice-index: 03-slice.md
  siblings:
    - 03-slice-commit-hygiene.md
    - 03-slice-backend-version.md
    - 03-slice-android-versioning.md
    - 03-slice-release-orchestration.md
  plan: 04-plan-nsis-installer.md
  implement: 05-implement-nsis-installer.md
---

# Slice: nsis-installer

## Goal

Produce `backend/installer/pushkit.nsi` — a full-lifecycle NSIS script that wraps `pushkit-server.exe` into a Windows installer with an optional Windows service component, supporting silent install, upgrade with service stop/restart, and clean uninstall (including service deregistration). The installer can be built locally on a Windows machine with `makensis` for fast iteration, decoupled from the GitHub Actions pipeline.

## Why This Slice Exists

This is the highest-uncertainty piece of the workflow per Round 2 risk ordering. NSIS authoring is a niche skill, the optional-service component (Round 4) escalated this from "minimal silent installer" to "full lifecycle automation" which is non-trivial, and the artifact is hard to validate via CI alone — real verification requires a Windows machine and human eyes on `Apps & Features`. Landing this slice early gives the maintainer time to iterate locally before the `release-orchestration` slice tries to invoke `makensis` in CI.

`complexity: l` because: ~150–250 lines of NSIS spread across the main installer section, the service-component logic, an upgrade-detection function (`.onInit`), and the uninstaller; plus a `backend/installer/README.md` reference doc for future authors; plus local validation on the maintainer's Windows machine.

## Scope

### In

- New `backend/installer/pushkit.nsi` with:
  - Header: name, version (from `/DVERSION=`), output filename `pushkit-server-setup.exe`, install dir `$PROGRAMFILES64\PushKit`.
  - Required component (always installed): copy `pushkit-server.exe` to install dir, create Start Menu folder + shortcut, write uninstall registry keys for Apps & Features.
  - Optional component (default unchecked): register Windows service `PushKitServer` via `sc.exe create PushKitServer binPath= "..." start= demand`.
  - `Function .onInit`: detect existing install. If `PushKitServer` service exists, query state; if running, stop it before file replace. Store original component selection in a flag for post-install restart logic.
  - `Function .onInstSuccess`: if service component was selected (or was previously installed), start the service.
  - `Section "Uninstall"`: stop and `sc delete` the service if present, remove files, remove Start Menu shortcut, remove registry keys.
  - Silent install support (`/S`): default components only (service skipped). Skips service registration even if elevation is available, so silent install on non-admin shells doesn't fail.
  - Elevation: `RequestExecutionLevel admin` (NSIS will prompt for UAC on interactive install).
- New `backend/installer/README.md` reference doc covering: required defines (`VERSION`), components, upgrade detection logic, silent-install command, local validation steps (`makensis -DVERSION=0.1.0-test-1 backend/installer/pushkit.nsi`).
- A throwaway `pushkit-server.exe` (or a stub) is needed locally to validate the installer end-to-end. The implement stage will either build a real one (no scope creep — this is local-only) or instruct the maintainer to drop one in.

### Out (handled by other slices)

- Wiring `makensis` into the GitHub Actions release workflow — `release-orchestration` slice.
- The actual `pushkit-server.exe` binary's `--version` flag — `backend-version` slice. (The NSIS script doesn't care; it just packages whatever is at the input path.)
- Windows code-signing (out of scope for v0.x — accepts SmartScreen warning).

## Acceptance Criteria

- **Given** the maintainer runs `makensis -DVERSION=0.1.0-test-1 backend/installer/pushkit.nsi` on a Windows machine with NSIS 3.10 installed, **when** the build completes, **then** `pushkit-server-setup.exe` is produced in `backend/installer/` (or `dist/`, location decided in plan stage).
- **Given** the installer is run interactively with the service component ticked on a fresh Windows machine, **when** the install completes and the user runs `sc query PushKitServer`, **then** the service is registered with `STATE: 1 STOPPED` (start type demand). *(AC7 partial — interactive verification on maintainer's machine.)*
- **Given** the installer is run silent (`pushkit-server-setup.exe /S`), **when** the install completes, **then** `"%ProgramFiles%\PushKit\pushkit-server.exe"` exists, Start Menu shortcut exists, and `sc query PushKitServer` returns the registered service in `STATE: 1 STOPPED` (silent install respects the default-checked service component and registers the service). *AC updated from original draft which said silent skips service — PO chose default-checked knowing this divergence; see plan `## Blockers`.*
- **Given** an existing install with the service component registered and running, **when** the installer is re-run with a newer version, **then** the service is stopped, the binary replaced, and the service restarted automatically. No orphan files or registry keys.
- **Given** an existing install, **when** the uninstaller is run from Apps & Features, **then** the service is stopped, deregistered, files removed, Start Menu folder removed, registry keys removed. No residue.

## Dependencies on Other Slices

None on input. The `release-orchestration` slice depends on this slice's `pushkit.nsi` existing.

## Risks

- **NSIS dialect gotchas.** NSIS has multiple stdlib options (`MUI2`, `LogicLib`, `nsServices`). The plan stage will pin which headers are used and document them in `backend/installer/README.md`.
- **Service lifecycle on upgrade.** The `.onInit` detection logic is the trickiest part. If the user opts OUT of the service component on an upgrade that previously HAD the service, the installer must decide: deregister it, or leave it pointing at the (now-stale) old binary? Decision deferred to plan stage; likely "deregister if user explicitly unchecks" is safest.
- **UAC failure modes.** If the user clicks "No" on the UAC prompt, the installer should exit cleanly, not partially install. NSIS handles this by default with `RequestExecutionLevel admin`.
- **Silent install on non-admin shell.** `/S` from a non-elevated shell will fail UAC silently. Document as "silent install requires an already-elevated shell or auto-elevation policy."
- **`makensis` version drift on `windows-2022` runner.** Confirmed pre-installed at 3.10 per freshness research. If GitHub bumps the image and drops NSIS, the `release-orchestration` slice will install via `choco install nsis -y --version 3.10`.
- **Path with spaces.** `%ProgramFiles%\PushKit\pushkit-server.exe` contains no spaces in the project name; install path with spaces (e.g., user picks `C:\Program Files (x86)\My Apps\PushKit\`) needs quoted paths everywhere in the NSIS script.
- **Local-validation feedback loop.** Without a real `pushkit-server.exe` to stuff in the installer, the maintainer's iteration is paper-only. Implement stage will note that the maintainer should `go build` a local binary before `makensis` runs.
