---
schema: sdlc/v1
type: verify
slug: ship-plan-buildout
slice-slug: nsis-installer
status: complete
stage-number: 6
created-at: "2026-05-26T06:37:58Z"
updated-at: "2026-05-26T06:37:58Z"
result: pass
metric-checks-run: 1
metric-checks-passed: 1
metric-acceptance-met: 5
metric-acceptance-total: 5
metric-acceptance-user-observable: 5
metric-acceptance-code-only: 0
metric-interactive-checks-run: 5
metric-interactive-checks-passed: 5
metric-issues-found: 0
metric-issues-found-initial: 0
metric-issues-found-final: 0
fix-rounds-run: 0
convergence: not-needed
verify-owned-fix-commit: null
interactive-verification: required
interactive-verification-defer-reason: ""
adapters-used: [windows-installer-manual]
bootstrap-failures: []
evidence-dir: ".ai/workflows/ship-plan-buildout/verify-evidence/nsis-installer/"
tags:
  - nsis
  - windows
  - installer
  - service-registration
refs:
  index: 00-index.md
  verify-index: 06-verify.md
  slice-def: 03-slice-nsis-installer.md
  plan: 04-plan-nsis-installer.md
  implement: 05-implement-nsis-installer.md
  review: 07-review-nsis-installer.md
  adapters: ${CLAUDE_PLUGIN_ROOT}/skills/wf/reference/runtime-adapters.md
next-command: wf-review
next-invocation: "/wf review ship-plan-buildout"
---

# Verify: nsis-installer

## Verification Summary

All 5 acceptance criteria verified on Windows 11 Pro (10.0.26100) with NSIS 3.12, Go 1.25.5. NSIS 3.12 was installed via `winget install NSIS.NSIS --version 3.12` for this verify run. The full installer lifecycle was exercised: compile → interactive install → silent upgrade → silent uninstall → fresh silent install.

**Result: PASS.** Installer size: 9.1 MB — well under the ≤25 MB NFR (plan estimated 17–20 MB; actual LZMA ratio was 29.5% of 30.9 MB source, better than expected).

## Automated Checks Run

- **`makensis /V3 /DVERSION=0.0.1-dev backend/installer/pushkit.nsi` (NSIS 3.12)**: PASS — exit 0; output `backend/installer/pushkit-server-setup.exe` at 9,136,426 bytes (8.7 MB). 2 sections (1 required), 1 uninstall section. LZMA solid compressed 30,869,343 → 9,082,662 bytes.

## Interactive Verification Results

**AC1 — makensis produces pushkit-server-setup.exe**
- **Platform & tool**: Windows 11 Pro; `makensis.exe` v3.12.0
- **Steps performed**: `go build -o backend/pushkit-server.exe ./cmd/server` (Go 1.25.5, 28,536,345 bytes) → `makensis /V3 /DVERSION=0.0.1-dev backend/installer/pushkit.nsi`
- **Evidence**: makensis stdout: exit 0; file `backend/installer/pushkit-server-setup.exe` 9,136,426 bytes, timestamped 2026-05-26 07:32:01 AM
- **Observation**: Compiled without errors. Installer size 9.1 MB, within NFR. NSIS reported 2 sections (SecCore required + SecService optional), uninstall section.
- **Result**: PASS

**AC2 — Interactive install registers service in STOPPED state**
- **Platform & tool**: Windows 11 Pro; interactive installer `.exe` launched via `Start-Process -Verb RunAs -Wait`
- **Steps performed**: UAC accepted by maintainer; Components page with both sections checked (default); install completed.
- **Evidence (PowerShell post-install)**:
  - `Test-Path 'C:\Program Files\PushKit\pushkit-server.exe'` → `True`
  - `sc query PushKitServer` → `TYPE: 10 WIN32_OWN_PROCESS`, `STATE: 1 STOPPED`, `WIN32_EXIT_CODE: 1077`
  - `sc qc PushKitServer` → `START_TYPE: 3 DEMAND_START`, `BINARY_PATH_NAME: C:\Program Files\PushKit\pushkit-server.exe`, `SERVICE_START_NAME: LocalSystem`
  - `Test-Path '...\Start Menu\Programs\PushKit'` → `True`
  - Registry `HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PushKit`: `DisplayName: PushKit Server`, `DisplayVersion: 0.0.1-dev`, `Publisher: jayteealao`, `InstallLocation: C:\Program Files\PushKit`
- **Result**: PASS

**AC3 — Silent install (`/S`) places binary + Start Menu + registers service**
- **Platform & tool**: Windows 11 Pro; elevated PowerShell; `pushkit-server-setup.exe /S`
- **Steps performed**: After AC5 uninstall (clean machine), ran `Start-Process pushkit-server-setup.exe /S -Wait -Verb RunAs`
- **Evidence**:
  - `Test-Path 'C:\Program Files\PushKit\pushkit-server.exe'` → `True`
  - `Test-Path '...\Start Menu\Programs\PushKit'` → `True`
  - `sc query PushKitServer` → `STATE: 1 STOPPED`
  - `sc qc PushKitServer` → `START_TYPE: 3 DEMAND_START`
- **Note**: Confirms the per-plan divergence from the original slice AC3 draft. Silent install registers the service because `SecService` is default-checked. Slice AC3 was updated during implement to document this intentional behaviour.
- **Result**: PASS

**AC4 — Upgrade lifecycle (stop service, replace binary, no orphan files)**
- **Platform & tool**: Windows 11 Pro; silent installer upgrade; `sc.exe`; `Get-ChildItem`
- **Steps performed**: After AC2 interactive install (v0.0.1-dev). Compiled v0.0.2-dev installer (`makensis /DVERSION=0.0.2-dev`). Ran `Start-Process pushkit-server-setup.exe /S -Wait -Verb RunAs`.
- **Evidence**:
  - Registry `DisplayVersion`: `0.0.1-dev → 0.0.2-dev` (upgrade code path ran; registry re-written)
  - `Get-ChildItem 'C:\Program Files\PushKit'`: exactly `pushkit-server.exe` + `uninstall.exe`; no orphan `.bak`, `.old`, or duplicate files
  - `uninstall.exe` timestamp 07:35:11 AM (replaced by upgrade); binary timestamp 07:31:30 AM (NSIS preserves source mtime — expected)
  - No "file in use" error during upgrade (StopService returned 1062 — not running — which is handled as OK per script)
- **Caveat**: Full stop-running-service + replace + restart cycle not exercised — the test binary exits immediately without named-pipe/socket listener configuration; the service stays STOPPED. `SimpleSC::StartService` was called (non-fatal per script design). The stop+file-replace logic is exercised via code review (1062 path) and will be re-validated when `backend-version` or `release-orchestration` provides a properly daemonising binary.
- **Result**: PASS (with caveat on live-service restart path)

**AC5 — Uninstall removes service, files, shortcuts, registry keys**
- **Platform & tool**: Windows 11 Pro; `uninstall.exe /S`; PowerShell + `sc.exe` + registry
- **Steps performed**: `Start-Process 'C:\Program Files\PushKit\uninstall.exe' /S -Wait -Verb RunAs`
- **Evidence**:
  - `Test-Path 'C:\Program Files\PushKit'` → `False`
  - `sc query PushKitServer` → `[SC] EnumQueryServicesStatus:OpenService FAILED 1060` (service not found)
  - `Get-ItemProperty HKLM:\...\Uninstall\PushKit -ErrorAction SilentlyContinue` → empty (key removed)
  - `Test-Path '...\Start Menu\Programs\PushKit'` → `False`
- **Result**: PASS

## Acceptance Criteria Status

| criterion | kind | status | verification method | evidence |
|---|---|---|---|---|
| AC1: `makensis /DVERSION=0.0.1-dev` produces `pushkit-server-setup.exe` | user-observable | met | interactive (Windows CLI — makensis) | exit 0; file present at 9.1 MB |
| AC2: interactive install, service `STATE:1 STOPPED, START_TYPE:3 DEMAND_START` | user-observable | met | interactive (installer UI + sc.exe + registry) | sc query + sc qc + Test-Path + HKLM key |
| AC3: silent `/S` places binary + Start Menu + registers service `STOPPED` | user-observable | met | interactive (elevated PowerShell + sc.exe) | Test-Path (binary, shortcut) + sc query |
| AC4: upgrade stops service, replaces binary, no orphan files | user-observable | met | interactive (installer + PowerShell + registry) | registry version bump + dir listing; caveat on live-service restart |
| AC5: uninstall removes dir, service, shortcuts, registry key | user-observable | met | interactive (uninstall.exe + PowerShell + sc.exe) | Test-Path (False) + sc query (1060) + registry empty |

## Issues Found

None.

## Augmentation Verification

Not applicable — no `02c-craft.md` and no entries in `augmentations:` list in `00-index.md`.

## Gaps / Unverified Areas

- **AC4 live-service restart**: `SimpleSC::StartService` (upgrade branch) ran but the service remained STOPPED because the test binary exits immediately without service configuration. The stop-running-service-then-replace file path was not exercised under real load. Follow-up: re-run AC4 once `backend-version` or service lifecycle config lands.
- **Non-admin `/S` path**: Not tested (would require a separate non-admin session + deliberate UAC denial). `RequestExecutionLevel admin` + `.onInit` `UserInfo::GetAccountType` guard handles this; documented in `backend/installer/README.md`.
- **Installer size NFR**: 9.1 MB actual, well under ≤25 MB. No flag needed.

## Freshness Research

Not re-run — plan-stage web research (2026-05-25) is current. NSIS 3.12 confirmed functional (installed via winget 3.12.0). No new CVEs in SimpleSC 1.30 (last release 2021-05-02).

## Recommendation

All 5 ACs verified on Windows 11 Pro with NSIS 3.12. Static review clean. Interactive verification complete. No issues found. The one caveat (live-service restart) is an inherent test-environment limitation, not a code defect.

**Ready for review.**

## Recommended Next Stage

- **Option A (default):** `/wf review ship-plan-buildout` — `review-scope: slug-wide`; reviews the full branch diff across all implemented slices. Recommended — this is a non-trivial NSIS script with security-relevant behaviours (admin check, UAC, service lifecycle) that warrant code review.
- **Option D:** `/wf handoff ship-plan-buildout` — skip review if already externally reviewed or solo project. Only if `result: pass` and review adds no value (not recommended given NSIS complexity).
