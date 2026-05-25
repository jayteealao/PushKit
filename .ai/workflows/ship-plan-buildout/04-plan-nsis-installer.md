---
schema: sdlc/v1
type: plan
slug: ship-plan-buildout
slice-slug: nsis-installer
status: complete
stage-number: 4
created-at: "2026-05-25T23:33:45Z"
updated-at: "2026-05-25T23:33:45Z"
metric-files-to-touch: 4
metric-step-count: 12
has-blockers: true
revision-count: 0
stack-source: confirmed
tags:
  - nsis
  - windows
  - installer
  - service-registration
  - simplesc
refs:
  index: 00-index.md
  plan-index: 04-plan.md
  slice-def: 03-slice-nsis-installer.md
  siblings:
    - 04-plan-commit-hygiene.md
    - 04-plan-backend-version.md          # not yet written
    - 04-plan-android-versioning.md       # not yet written
    - 04-plan-release-orchestration.md    # not yet written
  implement: 05-implement-nsis-installer.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout nsis-installer"
---

# Plan: nsis-installer

## Current State

- `backend/installer/` does not exist. No `.nsi`, `.iss`, `.wxs`, or any other installer-authoring artifacts exist anywhere in the repo (confirmed via repo-wide glob).
- `backend/pushkit-server.exe` exists at repo root (untracked, ~27.2 MB / 28,536,345 bytes, build timestamp Mar 12 14:03). This is the candidate input binary for local NSIS iteration. The 27.2 MB figure already exceeds shape's NFR ceiling of 25 MB for the installer total, before any compression or NSIS overhead — explicitly flagged in `## Risks` below.
- `backend/cmd/server/main.go` currently has no `Version` var and no `--version` flag. The sibling `backend-version` slice will add those. This slice is indifferent — the installer packages whatever PE binary is provided at the input path.
- The current `.github/workflows/release.yml` (52 lines) is a Python-wheel-publish stub; it does NOT yet reference `backend/installer/`. The `release-orchestration` slice will wire `makensis backend/installer/pushkit.nsi /DVERSION=<tag>` on a `windows-2022` runner; the contract is documented in `03-slice-release-orchestration.md:68`.
- No prior `sc.exe`, `nssm`, `winsw`, `installutil`, or `New-Service` usage in any executable script (`.ps1`, `.sh`, `.yml`, `.go`). All references to service registration are in planning docs (`.ai/workflows/ship-plan-buildout/02-shape.md`, `03-slice-nsis-installer.md`). This slice is greenfield.
- No `.editorconfig`, `.prettierrc`, or NSIS formatter exists. No in-repo convention for NSIS source style.
- `backend/installer/README.md` will be the **first** `README.md` inside `backend/` (only root `README.md` exists today).

## Reuse Opportunities

The affected-code sub-agent's reuse scan found **no executable code to reuse**. Specifically:

- **NSIS scaffolding** — none in repo. Implement-fresh-no-precedent.
- **Service-registration code** — none in repo. Implement-fresh.
- **Installer authoring docs** — none. The new `backend/installer/README.md` defines the contract.
- **Vendored plugins** — none. This slice introduces `backend/installer/plugins/SimpleSC.dll` (Unicode 1.30) as the first vendored installer plugin.
- **Build helpers** — root `Makefile` has only `build-wheels`, `publish`, `clean`. **No new Makefile target in this slice** (per same discipline as `commit-hygiene` — adding `make build-installer` is deferred to `release-orchestration` if CI parity in local Make recipes is wanted).
- **`backend/pushkit-server.exe` (untracked, 27.2 MB)** — exists on disk and works as immediate iteration input. Not committed; not reusable across fresh clones.

Net: 3 new tracked files (`pushkit.nsi`, `plugins/SimpleSC.dll`, `README.md`) + 1 modified (`.gitignore`).

## Likely Files / Areas to Touch

New files (tracked):

- `backend/installer/pushkit.nsi` — main installer script. Target ~250 lines (MUI2 boilerplate + install/uninstall sections + service-lifecycle logic + Apps-&-Features registry writes).
- `backend/installer/plugins/SimpleSC.dll` — vendored NSIS plugin, Unicode 1.30 (~400 KB). Tracked in git. Source: `NSIS_Simple_Service_Plugin_Unicode_1.30.zip` from the NSIS Wiki SimpleSC page.
- `backend/installer/README.md` — installer-authoring reference for future maintainers. Covers required `/D` defines (`VERSION`), components, upgrade detection, silent-install command, local validation steps (`go build` → `makensis`).

Modified files:

- `.gitignore` (root) — add `backend/installer/pushkit-server-setup.exe` (build output) and `backend/pushkit-server.exe` (untracked local-iteration binary; user's existing exe is already ignored by being untracked, but ensuring it stays ignored is cheap insurance).

Build outputs (NOT tracked):

- `backend/installer/pushkit-server-setup.exe` — produced by `makensis`. Ignored.
- `backend/pushkit-server.exe` — produced by local `go build`. Ignored.

## Proposed Change Strategy

Single-track, all-in-one:

1. **Vendor `SimpleSC.dll`** under `backend/installer/plugins/` and `!addplugindir "plugins"` at the top of `pushkit.nsi`. Plugin choice locked per discovery (Round 1) — raw `nsExec + sc.exe` is unsafe on upgrade because `sc.exe` returns before the service process exits, causing "file in use" errors when the install section overwrites the binary. `SimpleSC::StopService "name" 1 30` polls SCM state until SERVICE_STOPPED with a 30-second timeout.
2. **Use MUI2 with a `MUI_PAGE_COMPONENTS` page** (no Directory page — install dir pinned to `$PROGRAMFILES64\PushKit`). Components page exposes the service component (default-checked per Round 2). Welcome, Components, InstFiles, Finish pages for install; Confirm + InstFiles for uninstall. Single English language.
3. **`.onInit` does the admin check** (`UserInfo::GetAccountType`) and the upgrade-detection (`SimpleSC::ExistsService "PushKitServer"`). Aborts loudly on non-admin (per Round 2). Sets a global flag (`$WasInstalled`) so the install section knows whether to stop+replace+restart or fresh-install.
4. **Install section does the heavy lifting**: stop service if it was previously installed (`SimpleSC::StopService` with file-release wait), write files (`SetOutPath` + `File`), write Apps-&-Features registry, write the uninstaller. Then if the service component is selected, `SimpleSC::InstallService` (if fresh) or `SimpleSC::StartService` (if upgrade).
5. **Uninstall section** stops + removes the service if present, deletes files explicitly (not `RMDir /r` — research anti-pattern #4 documents `$INSTDIR` deletion can wipe user data if the install dir was tampered with), removes Start Menu shortcut, deletes the Apps-&-Features registry key.
6. **`SetCompressor /SOLID lzma`** at the top of the script. Solid LZMA on a 27.2 MB Go binary typically lands ~17–20 MB installer. Compile time adds ~5–10 s; trivial vs CI wall time. If first build still exceeds 25 MB, the plan flags it in handoff.
7. **Local iteration loop**: `go build -o backend/pushkit-server.exe ./backend/cmd/server` → `makensis /V3 /DVERSION=0.0.1-dev backend/installer/pushkit.nsi` → `.\backend\installer\pushkit-server-setup.exe` (interactive) or `pushkit-server-setup.exe /S` (silent, from elevated PowerShell). All steps documented in `backend/installer/README.md`.

The first commit of this slice on `feat/ship-plan-buildout` will land after `commit-hygiene` is merged (per slice index sequencing). Commit shape (Conventional Commits): `feat(installer): add NSIS Windows installer with optional service component` — single commit for the full slice, or split into vendor+script+docs commits if size warrants.

## Step-by-Step Plan

1. **Create `backend/installer/` directory + `plugins/` subdir.** No commits yet; structure only.

2. **Vendor SimpleSC plugin.** Download `NSIS_Simple_Service_Plugin_Unicode_1.30.zip` from the NSIS Wiki SimpleSC page (or the SourceForge mirror). Extract `SimpleSC.dll` (Unicode build — required for NSIS 3.x) into `backend/installer/plugins/SimpleSC.dll`. Verify file size ~400 KB and that it's the Unicode variant (the zip contains both Ansi and Unicode subdirs — use Unicode). Commit the DLL as a tracked binary; document in `backend/installer/README.md` where it came from and what version.

3. **Author `backend/installer/pushkit.nsi`.** Structure (in order):

   ```nsis
   ; --- Header ---
   !define APP_NAME           "PushKit Server"
   !define APP_PUBLISHER      "jayteealao"
   !define APP_URL            "https://github.com/jayteealao/PushKit"
   !define APP_REG_KEY        "Software\Microsoft\Windows\CurrentVersion\Uninstall\PushKit"
   !define SERVICE_NAME       "PushKitServer"
   !define SERVICE_DISPLAY    "PushKit Server"
   !define SERVICE_DESCRIPTION "PushKit backend service. See https://github.com/jayteealao/PushKit"
   ; VERSION supplied at compile time: makensis /DVERSION=x.y.z
   !ifndef VERSION
     !define VERSION "0.0.0-dev"
   !endif

   Name "${APP_NAME} ${VERSION}"
   OutFile "pushkit-server-setup.exe"
   InstallDir "$PROGRAMFILES64\PushKit"
   RequestExecutionLevel admin
   SetCompressor /SOLID lzma
   Unicode true
   ShowInstDetails show
   ShowUninstDetails show

   !addplugindir "plugins"
   !include "MUI2.nsh"
   !include "LogicLib.nsh"
   !include "FileFunc.nsh"   ; for ${GetSize} (EstimatedSize)

   ; --- Globals ---
   Var WasInstalled        ; "1" if upgrade, "0" if fresh

   ; --- MUI2 pages ---
   !define MUI_ABORTWARNING
   !insertmacro MUI_PAGE_WELCOME
   !insertmacro MUI_PAGE_COMPONENTS
   !insertmacro MUI_PAGE_INSTFILES
   !insertmacro MUI_PAGE_FINISH

   !insertmacro MUI_UNPAGE_CONFIRM
   !insertmacro MUI_UNPAGE_INSTFILES
   !insertmacro MUI_LANGUAGE "English"

   ; --- .onInit: admin check + upgrade detection ---
   Function .onInit
     SetRegView 64

     UserInfo::GetAccountType
     Pop $0
     ${If} $0 != "Admin"
       MessageBox MB_ICONSTOP \
         "PushKit installer requires administrator rights. Right-click and Run as administrator." \
         /SD IDOK
       Quit
     ${EndIf}

     StrCpy $WasInstalled "0"
     SimpleSC::ExistsService "${SERVICE_NAME}"
     Pop $0
     ; ExistsService inversion: 0 = service EXISTS, non-zero = does NOT exist.
     ${If} $0 == 0
       StrCpy $WasInstalled "1"
     ${EndIf}
   FunctionEnd

   ; --- Install section (required component) ---
   Section "PushKit Server (required)" SecCore
     SectionIn RO          ; cannot be unchecked

     ${If} $WasInstalled == "1"
       SimpleSC::StopService "${SERVICE_NAME}" 1 30
       Pop $0
       ; 1062 = service not running (OK); 0 = stopped successfully (OK); other = real error
       ${If} $0 != 0
       ${AndIf} $0 != 1062
         MessageBox MB_ICONSTOP "Cannot stop existing PushKitServer service (code $0). Aborting upgrade." /SD IDOK
         Abort
       ${EndIf}
     ${EndIf}

     SetOutPath "$INSTDIR"
     File "..\pushkit-server.exe"

     CreateDirectory "$SMPROGRAMS\PushKit"
     CreateShortcut  "$SMPROGRAMS\PushKit\PushKit Server.lnk" "$INSTDIR\pushkit-server.exe"
     CreateShortcut  "$SMPROGRAMS\PushKit\Uninstall PushKit.lnk" "$INSTDIR\uninstall.exe"

     ; --- Apps & Features registry ---
     ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
     WriteRegStr   HKLM "${APP_REG_KEY}" "DisplayName"          "${APP_NAME}"
     WriteRegStr   HKLM "${APP_REG_KEY}" "DisplayVersion"       "${VERSION}"
     WriteRegStr   HKLM "${APP_REG_KEY}" "Publisher"            "${APP_PUBLISHER}"
     WriteRegStr   HKLM "${APP_REG_KEY}" "URLInfoAbout"         "${APP_URL}"
     WriteRegStr   HKLM "${APP_REG_KEY}" "InstallLocation"      "$INSTDIR"
     WriteRegStr   HKLM "${APP_REG_KEY}" "DisplayIcon"          "$INSTDIR\pushkit-server.exe,0"
     WriteRegStr   HKLM "${APP_REG_KEY}" "UninstallString"      "$\"$INSTDIR\uninstall.exe$\""
     WriteRegStr   HKLM "${APP_REG_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
     WriteRegDWORD HKLM "${APP_REG_KEY}" "EstimatedSize"        "$0"
     WriteRegDWORD HKLM "${APP_REG_KEY}" "NoModify"             1
     WriteRegDWORD HKLM "${APP_REG_KEY}" "NoRepair"             1

     WriteUninstaller "$INSTDIR\uninstall.exe"
   SectionEnd

   ; --- Optional service component (default-checked) ---
   Section "Register Windows service" SecService
     ${If} $WasInstalled == "0"
       ; Fresh install: create service. binPath="exe", start type=demand (3).
       SimpleSC::InstallService "${SERVICE_NAME}" "${SERVICE_DISPLAY}" \
         "16" "3" "$INSTDIR\pushkit-server.exe" "" "" ""
       Pop $0
       ${If} $0 != 0
         MessageBox MB_ICONSTOP "Cannot register PushKitServer service (code $0)." /SD IDOK
         ; Don't abort the whole install — binary is already on disk and useful even without the service.
         ; Surface the failure but allow the install to complete.
       ${Else}
         SimpleSC::SetServiceDescription "${SERVICE_NAME}" "${SERVICE_DESCRIPTION}"
         Pop $0
       ${EndIf}
     ${Else}
       ; Upgrade: service already exists; just restart it after binary replace.
       SimpleSC::StartService "${SERVICE_NAME}" "" 30
       Pop $0
       ; non-zero is non-fatal: user can start manually.
     ${EndIf}
   SectionEnd

   ; --- Section descriptions for MUI Components page ---
   LangString DESC_SecCore    ${LANG_ENGLISH} "Installs pushkit-server.exe and Start Menu shortcuts (required)."
   LangString DESC_SecService ${LANG_ENGLISH} "Registers PushKitServer as a Windows service (start type: demand)."

   !insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
     !insertmacro MUI_DESCRIPTION_TEXT ${SecCore}    $(DESC_SecCore)
     !insertmacro MUI_DESCRIPTION_TEXT ${SecService} $(DESC_SecService)
   !insertmacro MUI_FUNCTION_DESCRIPTION_END

   ; --- Uninstaller ---
   Function un.onInit
     SetRegView 64
     UserInfo::GetAccountType
     Pop $0
     ${If} $0 != "Admin"
       MessageBox MB_ICONSTOP "Uninstaller requires administrator rights." /SD IDOK
       Quit
     ${EndIf}
   FunctionEnd

   Section "Uninstall"
     ; Stop + remove the service if it exists.
     SimpleSC::ExistsService "${SERVICE_NAME}"
     Pop $0
     ${If} $0 == 0
       SimpleSC::StopService   "${SERVICE_NAME}" 1 30
       Pop $0
       SimpleSC::RemoveService "${SERVICE_NAME}"
       Pop $0
     ${EndIf}

     ; Delete files explicitly — never RMDir /r $INSTDIR (anti-pattern #4).
     Delete "$INSTDIR\pushkit-server.exe"
     Delete "$INSTDIR\uninstall.exe"
     RMDir  "$INSTDIR"            ; only succeeds if empty — safe.

     Delete   "$SMPROGRAMS\PushKit\PushKit Server.lnk"
     Delete   "$SMPROGRAMS\PushKit\Uninstall PushKit.lnk"
     RMDir    "$SMPROGRAMS\PushKit"

     DeleteRegKey HKLM "${APP_REG_KEY}"
   SectionEnd
   ```

   Notes for implementation:
   - The `File "..\pushkit-server.exe"` directive uses a path **relative to the `.nsi` file**, not the runner cwd. From `backend/installer/pushkit.nsi`, `..` is `backend/`, so the file resolves to `backend/pushkit-server.exe`. This is the same path both locally (after `go build -o backend/pushkit-server.exe`) and in CI (after the cross-compile job places the artifact there).
   - SimpleSC `ExistsService` returns `0` when the service **exists**, non-zero when it does **not** — the inverted semantics is a documented gotcha. The script uses `${If} $0 == 0` accordingly. Comment in the script reinforces this.
   - Service start type `3` = `SERVICE_DEMAND_START` (manual). Per slice spec ("`start= demand`") and shape AC line 170 (`STATE: 1 STOPPED` after install). Service type `16` = `SERVICE_WIN32_OWN_PROCESS`.
   - `Abort` (inside a section) triggers the NSIS abort handler and reports failure. `Quit` (inside `.onInit`) exits cleanly before any UI is shown. Used correctly above.

4. **Author `backend/installer/README.md`** covering:

   - **Purpose**: this file is the reference for future installer maintainers; not for end users (end-user installer docs go in the root `README.md`'s `## Backend installer` section, owned by `release-orchestration`).
   - **Required defines**: `VERSION` is mandatory. Default is `0.0.0-dev` if omitted (compile won't fail; metadata will be wrong).
   - **Components**:
     - `SecCore` — required, read-only (`SectionIn RO`). Cannot be unchecked.
     - `SecService` — optional, default-checked. Registers the Windows service.
   - **Upgrade detection**: `.onInit` uses `SimpleSC::ExistsService`. If service exists, the install section stops it before file replace, then restarts after.
   - **Silent install**: `pushkit-server-setup.exe /S`. NOTE: since `SecService` is default-checked, silent install REGISTERS the service. This diverges from the slice's original AC3 — see `## Blockers` in the plan. To install binary-only via silent, the maintainer would need to pass an additional flag (not implemented in this iteration; flagged for future).
   - **Local validation steps**:
     ```powershell
     # From repo root:
     go build -o backend/pushkit-server.exe ./backend/cmd/server
     & "C:\Program Files (x86)\NSIS\makensis.exe" /V3 /DVERSION=0.0.1-dev backend/installer/pushkit.nsi
     # Interactive test:
     .\backend\installer\pushkit-server-setup.exe
     # Silent test (from elevated PowerShell):
     .\backend\installer\pushkit-server-setup.exe /S
     # Verify service:
     sc query PushKitServer
     # Uninstall:
     & "$env:ProgramFiles\PushKit\uninstall.exe" /S
     ```
   - **NSIS version requirement**: 3.12 minimum. Install via the official NSIS .exe installer; `makensis.exe` ends up at `%PROGRAMFILES(X86)%\NSIS\makensis.exe`.
   - **Vendored plugin**: `plugins/SimpleSC.dll` (Unicode 1.30). Re-vendoring: download `NSIS_Simple_Service_Plugin_Unicode_1.30.zip` from the NSIS Wiki SimpleSC page; replace the DLL.
   - **What this file does NOT cover**: PyPI CLI, Android, backend API — those live elsewhere.

5. **Update root `.gitignore`** to add:
   ```
   backend/installer/pushkit-server-setup.exe
   backend/pushkit-server.exe
   ```
   Both are build outputs that should not be tracked. (The root `.gitignore` currently doesn't exist — `commit-hygiene` creates it; this slice extends it. If `commit-hygiene` has not yet merged when this slice's implementation begins, that's a sequencing problem flagged in `## Dependencies on Other Slices`.)

6. **Local validation — interactive path on maintainer's Windows 11.**
   - Run the build sequence from step 4's "Local validation steps" — produce a fresh `pushkit-server-setup.exe`.
   - Interactive install: launch the .exe (no `/S`), confirm UAC, accept default components (both ticked), finish.
   - Verify: `Test-Path "$env:ProgramFiles\PushKit\pushkit-server.exe"` → True. `sc query PushKitServer` → `STATE: 1 STOPPED`, `START_TYPE: 3 DEMAND_START`. Start Menu has "PushKit" folder with both shortcuts. Apps & Features shows "PushKit Server 0.0.1-dev" with publisher "jayteealao".
   - Capture: PowerShell transcript + Apps & Features screenshot.

7. **Local validation — silent install path.**
   - From an already-elevated PowerShell: `.\backend\installer\pushkit-server-setup.exe /S`.
   - Verify: same `Test-Path` and `sc query` checks. NOTE: per Round 2 decision, silent install now REGISTERS the service by default (divergence from slice AC3). Document this in the verify artifact.
   - Try from a non-admin shell: should UAC-prompt. If the maintainer denies elevation, installer exits non-zero. Capture transcript.

8. **Local validation — upgrade path.**
   - Bump VERSION (e.g., `0.0.2-dev`), rebuild `pushkit-server.exe` (touch it or `go build` again), rebuild installer.
   - Manually start the service: `sc start PushKitServer`. Wait. `sc query PushKitServer` → `STATE: 4 RUNNING`.
   - Run the new installer (interactive or silent).
   - Verify: installer completes without "file in use" error. Post-install: `sc query` shows service still in RUNNING state (restarted). Binary on disk has the new timestamp. No `*.old` or `*.bak` files in install dir.
   - Capture transcript.

9. **Local validation — uninstall path.**
   - Settings → Apps & Features → PushKit Server → Uninstall (or run `& "$env:ProgramFiles\PushKit\uninstall.exe"`).
   - Verify: `Test-Path "$env:ProgramFiles\PushKit"` → False. `Test-Path "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\PushKit"` → False. `sc query PushKitServer` → service-not-found. `Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PushKit" -ErrorAction SilentlyContinue` → empty.
   - Capture: PowerShell transcript + Apps & Features screenshot (no PushKit entry).

10. **Sanity-check installer size.** `Get-Item backend\installer\pushkit-server-setup.exe | Select-Object Length`. Expect ~17–20 MB with LZMA on a 27.2 MB Go binary. If > 25 MB, flag in handoff (NFR break); the plan does NOT block on this.

11. **Optional follow-up after sibling slices land**: when `backend-version` adds the `--version` flag, re-run step 6 and additionally run `& "$env:ProgramFiles\PushKit\pushkit-server.exe" --version` — should print `pushkit-server 0.0.1-dev` (or whatever VERSION was passed to makensis). This is partial AC11 coverage that the smoke-test in `release-orchestration` will do automatically in CI.

12. **Commit + push.** Single commit (or 2–3 if size warrants): `feat(installer): add NSIS Windows installer with optional service component`. Push to `feat/ship-plan-buildout`. Open the PR (or leave for `release-orchestration` to expand later — branch strategy is dedicated, one PR per slice index).

## Test / Verification Plan

### Automated checks

- **`makensis` exit code** is the only mechanical gate for the .nsi script. Successful compile (exit 0) confirms syntactic validity, all `!include` resolutions, and all `File` directive paths.
- **Lint/typecheck**: not applicable — NSIS is its own language with no in-repo linter; `markdownlint` could check the README but no precedent in this repo.
- **Unit tests**: not applicable. NSIS scripts have no unit-test framework in `stack:`. The `smoke-test` job in `release.yml` (added by `release-orchestration`) will be the downstream automated check; out of scope here.
- **Integration tests**: not applicable to this slice.

### Interactive verification (human-in-the-loop)

Stack context: `stack.platforms: [service, cli, android]` does not list Windows explicitly; the NSIS installer is platform-adjacent to `service` (it packages the service binary). `stack.testing` lists `[go-testing, gradle-junit, android-lint]` — no Windows-installer test framework. All verification routes to the **maintainer's Windows 11 machine** using **native Windows tooling** (PowerShell, `sc.exe`, Apps & Features). No companion skills from `stack.available-skills` apply (none cover Windows installer UI). This is consistent with the `commit-hygiene` sibling plan's pattern of "native tooling + transcript + screenshot."

**AC1 — `makensis` produces `pushkit-server-setup.exe`**

- **What to verify**: `makensis -DVERSION=0.0.1-dev backend/installer/pushkit.nsi` exits 0 and writes `backend/installer/pushkit-server-setup.exe`.
- **Platform & tool**: Maintainer's Windows 11. Tool: `makensis.exe` (NSIS 3.12 CLI). NSIS is in `stack.build`.
- **Companion skills**: None.
- **Steps**:
  1. `go build -o backend/pushkit-server.exe ./backend/cmd/server` from repo root.
  2. `& "C:\Program Files (x86)\NSIS\makensis.exe" /V3 /DVERSION=0.0.1-dev backend/installer/pushkit.nsi`.
  3. Confirm exit code 0.
  4. `Get-Item backend\installer\pushkit-server-setup.exe | Select-Object Length, LastWriteTime`.
- **Evidence capture**: PowerShell transcript (`Start-Transcript` → `Stop-Transcript`) showing the build output and the file listing. Paste into `06-verify-nsis-installer.md`.
- **Pass criteria**: makensis exit code 0; file present; size > 0; size < 25 MB (target NFR; if larger, flag but don't block).

**AC2 — Interactive install with service component**

- **What to verify**: Interactive run with the service component ticked produces a registered service in `STOPPED, DEMAND_START` state.
- **Platform & tool**: Windows 11; the installer .exe; PowerShell + `sc.exe`.
- **Steps**:
  1. `.\backend\installer\pushkit-server-setup.exe` (no `/S`).
  2. Accept UAC. On Components page, both sections checked (default).
  3. Click through to completion.
  4. `sc query PushKitServer` → expect `STATE : 1  STOPPED`.
  5. `sc qc PushKitServer` → expect `START_TYPE : 3   DEMAND_START`.
  6. `Test-Path "$env:ProgramFiles\PushKit\pushkit-server.exe"` → True.
  7. Apps & Features → confirm "PushKit Server 0.0.1-dev" listed with publisher "jayteealao".
- **Evidence capture**: PowerShell transcript of steps 4–6 + screenshot of Apps & Features entry.
- **Pass criteria**: All checks pass; service exists and is STOPPED; install dir is `%ProgramFiles%\PushKit\`.

**AC3 — Silent install (`/S`)** *(slice spec divergence — see Blockers)*

- **What to verify**: `/S` produces files + Start Menu shortcut + (PER ROUND 2 DECISION) registered service. This diverges from `03-slice-nsis-installer.md` AC3 ("silent skips service component").
- **Platform & tool**: Elevated Windows 11 PowerShell.
- **Steps**:
  1. From elevated PowerShell at repo root: `.\backend\installer\pushkit-server-setup.exe /S`.
  2. `Test-Path "$env:ProgramFiles\PushKit\pushkit-server.exe"` → True.
  3. `Test-Path "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\PushKit"` → True.
  4. `sc query PushKitServer` → **now expects `STATE : 1  STOPPED`** (service registered per default-checked component).
- **Evidence capture**: PowerShell transcript.
- **Pass criteria**: Binary + shortcut present; service registered. (If the slice AC is left unchanged, this AC fails by design. Resolution: update the slice doc — see Blockers.)
- **Additional check (non-admin /S)**: from a NON-elevated PowerShell, `.\backend\installer\pushkit-server-setup.exe /S` → expect UAC prompt; if denied, exit code is non-zero (no silent install proceeds).

**AC4 — Upgrade lifecycle (stop + replace + restart)**

- **What to verify**: Re-running the installer over an existing install with a running service stops it, replaces the binary, restarts. No orphan files.
- **Platform & tool**: Windows 11; PowerShell + `sc.exe`.
- **Steps**:
  1. Install `0.0.1-dev` interactively per AC2.
  2. `sc start PushKitServer` → wait → `sc query PushKitServer` → STATE: RUNNING.
  3. Rebuild with `VERSION=0.0.2-dev`: `go build` + `makensis /DVERSION=0.0.2-dev ...`.
  4. Run the new installer (interactive, both components checked).
  5. Post-install: `sc query PushKitServer` → STATE: RUNNING (auto-restarted by `SimpleSC::StartService` in install section's upgrade branch).
  6. `Get-Item "$env:ProgramFiles\PushKit\pushkit-server.exe" | Select LastWriteTime` → newer than step 1's timestamp.
  7. `Get-ChildItem "$env:ProgramFiles\PushKit"` → only `pushkit-server.exe` + `uninstall.exe`; no `*.bak`, `*.old`, or duplicate files.
- **Evidence capture**: PowerShell transcript bracketing the upgrade.
- **Pass criteria**: Upgrade completes without "file in use" error; service is running post-upgrade; binary has new timestamp; no orphan files.

**AC5 — Uninstall removes everything**

- **What to verify**: Uninstall via Apps & Features (or `/S` on uninstall.exe) stops + deregisters the service, deletes files, deletes Start Menu folder, deletes registry key. No residue.
- **Platform & tool**: Windows 11 Settings + PowerShell + Registry.
- **Steps**:
  1. Open Settings → Apps & Features → PushKit Server → Uninstall.
  2. Accept UAC.
  3. After uninstall: `sc query PushKitServer` → `[SC] EnumQueryServicesStatus:OpenService FAILED 1060` (service not found).
  4. `Test-Path "$env:ProgramFiles\PushKit"` → False.
  5. `Test-Path "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\PushKit"` → False.
  6. `Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PushKit" -ErrorAction SilentlyContinue` → nothing returned.
  7. Apps & Features → confirm "PushKit Server" no longer listed.
- **Evidence capture**: PowerShell transcript of checks 3–6 + screenshot of Apps & Features with no PushKit entry.
- **Pass criteria**: All four `Test-Path`/registry/service checks return the "not present" result; no residue.

If no interactive verification needed: N/A — this slice is almost entirely interactive verification.

## Risks / Watchouts

- **CVE-2025-43715 in NSIS ≤ 3.10.** Patched in 3.11; second elevation fix in 3.12. Plan locks 3.12 minimum (per Round 1). CI install step is required (`release-orchestration` adds the marketplace step). Maintainer must also upgrade local NSIS 3.10 → 3.12 before iterating. If CI continues to use pre-installed 3.10 by accident, surface as a verify-stage blocker.
- **Installer size NFR.** Go binary is 27.2 MB; shape NFR is ≤25 MB. `SetCompressor /SOLID lzma` typically nets 30–40% reduction → expected ~17–20 MB. If actual result exceeds 25 MB after first build, flag in handoff and propose raising the NFR to 30 MB (Go-binary baseline is what it is; UPX is risky for service binaries due to antivirus false positives).
- **Service component default-checked breaks slice AC3.** PO chose "default-checked" knowing this diverges from `03-slice-nsis-installer.md` line 67's AC ("silent skips service component"). The plan honors the PO choice and proposes updating the slice doc. See `## Blockers` for the explicit list.
- **`SimpleSC::ExistsService` return-value inversion.** Returns `0` when service EXISTS, non-zero when NOT. Plan's script has the correct `${If} $0 == 0` test; reviewer must check this carefully.
- **`File "..\pushkit-server.exe"` path is relative to the `.nsi` file.** From `backend/installer/pushkit.nsi`, `..` is `backend/`. Both local (`go build -o backend/pushkit-server.exe`) and CI (artifact placed at `backend/pushkit-server.exe`) must put the binary at this exact path. The `release-orchestration` slice's plan must match this contract.
- **Path with spaces / SetRegView 64.** Installer is 64-bit (`InstallDir "$PROGRAMFILES64\PushKit"` + `SetRegView 64` in both `.onInit` and `un.onInit`). Skipping `SetRegView 64` in the uninstall path leaves the Apps & Features key in the wrong hive view and leaves a phantom entry — research anti-pattern #3.
- **`Abort` vs `Quit`.** Inside `.onInit` (before UI), `Quit` is used. Inside Sections, `Abort` is used so the NSIS abort handler fires. Mixing them up causes orphaned partial installs.
- **UAC denied silently when `EnableLua=0`.** Belt-and-suspenders `UserInfo::GetAccountType` check in `.onInit` covers this. If skipped, `SimpleSC::InstallService` would silently fail with ERROR_ACCESS_DENIED (5) on a non-elevated session with UAC disabled.
- **`RMDir /r $INSTDIR` is forbidden** (research anti-pattern #4). The uninstall section deletes files by name then `RMDir` (no `/r`); only succeeds if empty. Protects against $INSTDIR pointing somewhere unexpected.
- **`SimpleSC::StopService` timeout.** Set to 30 s. If the service genuinely takes longer to shut down, the installer aborts with a clear error. Maintainer can manually stop the service before re-running.
- **Vendored DLL supply chain.** `SimpleSC.dll` is vendored binary; SHA256 should be recorded in `backend/installer/README.md` so a future maintainer can verify integrity. Per intake's note that SLSA hardening is future scope, no signature verification at build time.
- **Local-iteration binary freshness.** If the maintainer forgets to re-run `go build` after a backend code change, the installer wraps a stale binary. The README warns about this; no scripted check in this slice (could add a `just` recipe in a future tooling slice).
- **No CHANGELOG entry.** Per workflow contract (`commit-hygiene` plan, cross-cutting concerns), CHANGELOG is generated at release time by `release-orchestration`. This slice contributes Conventional Commits but not changelog text directly.

## Dependencies on Other Slices

Inbound:

- **`commit-hygiene` must land first.** `.gitignore` and `package.json` etc are created by that slice. This slice extends `.gitignore`. If `commit-hygiene` has not been merged into `feat/ship-plan-buildout` when this slice's implementation begins, the modify-.gitignore step is a sequencing problem — either rebase, or wait.
- The conventional-commits commit-msg hook must be active so this slice's commits land as `feat(installer): ...` etc.

Outbound:

- **`release-orchestration` consumes**:
  - The relative path contract: `backend/installer/pushkit.nsi` exists and its `File` directive expects the input binary at `backend/pushkit-server.exe`. The CI's cross-compile job MUST place the artifact at exactly that path before invoking makensis.
  - The output contract: makensis produces `backend/installer/pushkit-server-setup.exe`. The CI uploads from this path.
  - The makensis invocation: `makensis /V3 /DVERSION=<tag-without-v> backend/installer/pushkit.nsi` from repo root.
  - The NSIS version requirement: 3.12 minimum. CI step installs via marketplace action (e.g., `repolevedavaj/install-nsis@v1` or `negrutiu/nsis-install@v1` — final pin chosen in `release-orchestration` plan).
- **`backend-version` consumes**: nothing directly. The installer is indifferent to whether the binary has a `--version` flag. The shape's smoke-test in `release-orchestration` is what cares about `--version` post-install.
- **`android-versioning`**: independent.

No file-level conflicts between this slice and any sibling. The `release-orchestration` slice will modify `.github/workflows/release.yml` (this slice does not). Root `.gitignore` is touched by both `commit-hygiene` (creates) and this slice (extends) — clean append, no conflict.

## Assumptions

- The maintainer is on Windows 11 with NSIS 3.12 installed locally (or willing to install it). NSIS install path is the default `%PROGRAMFILES(X86)%\NSIS\`.
- The maintainer has Go installed (per `backend/go.mod` toolchain — 1.24). `go build` from `backend/cmd/server/` produces a working binary.
- `SimpleSC.dll` Unicode 1.30 from the NSIS Wiki is the correct DLL. SHA256 recorded in `README.md` after vendoring.
- The decision to default-check the service component **overrides** slice AC3 ("silent skips service"). The slice doc will be updated in the same PR or as a follow-up annotation; this plan does not block on the doc change.
- Local iteration with the pre-existing untracked `backend/pushkit-server.exe` works for syntax/structural validation. End-to-end behavior (especially the `--version` smoke-test path) requires the `backend-version` slice's binary, but the installer-authoring iteration does not depend on that.
- The shape NFR of ≤25 MB installer is a target, not a hard gate. If LZMA result is 27 MB, the plan does not block — handoff documents the overage and proposes raising the NFR.
- No code-signing in scope for v0.x (per shape Out-of-Scope). Installer is unsigned; SmartScreen warning is accepted and documented in the end-user README section (owned by `release-orchestration`).
- The `windows-2022` runner will continue to be available on GitHub Actions until at least v0.2. If it's retired before then, the CI install-NSIS step works on `windows-2025` as well (per research, `windows-2025` ships no NSIS at all, so the install step is required regardless).

## Blockers

1. **Slice AC3 contradiction.** `03-slice-nsis-installer.md` line 67 asserts `/S` "skips service component"; PO's Round 2 answer makes silent install REGISTER the service by default. Resolution paths:
   - **Recommended**: Update the slice AC in this PR or as a follow-up edit to `03-slice-nsis-installer.md` to read: "Given the installer is run silent (`/S`), then `pushkit-server.exe` exists, Start Menu shortcut exists, and (per default-checked component) `sc query PushKitServer` returns the registered service in STOPPED state." This plan proceeds on that basis.
   - Alternative: revisit Round 2 with the PO; switch to "default-checked but `/S` skips" (hybrid). Would require an `IfSilent` guard in the service section. NOT chosen.

   This is the only material blocker. Plan otherwise proceeds and the AC update can be a single-line edit during implement or as a separate commit.

## Freshness Research

Web sub-agent run 2026-05-25. Headline findings:

| Topic | Decision pin | Source |
|---|---|---|
| NSIS version | 3.12 (CI + local). 3.10 has CVE-2025-43715 (CVSS 8.1, SYSTEM privilege escalation; fixed 3.11). 3.12 (2026-04-19) adds a second elevation fix. | [NSIS Changelog](https://nsis.sourceforge.io/Docs/AppendixF.html), [CVE-2025-43715 (Ameeba)](https://www.ameeba.com/blog/cve-2025-43715-privilege-escalation-vulnerability-in-nullsoft-scriptable-install-system-nsis/) |
| Service plugin | SimpleSC Unicode 1.30 (vendored). Raw `nsExec + sc.exe` is unsafe on upgrade (sc.exe returns before service exits and releases file handles). SimpleSC has `StopService "name" 1 30` with wait-for-file-release. | [NSIS SimpleSC wiki](https://nsis.sourceforge.io/NSIS_Simple_Service_Plugin), [auto-update-windows-service](https://github.com/omaha-consulting/auto-update-windows-service/blob/main/installer/Installer.nsi) |
| Upgrade idiom | `.onInit` → `SimpleSC::ExistsService` (note inverted return); install section → `StopService 1 30` before `File`; `.onInstSuccess` not needed for restart, do it inline. | [NSIS Forum SimpleSC](https://nsis-dev.github.io/NSIS-Forums/html/t-270562.html) |
| `/S` + UAC | `RequestExecutionLevel admin` triggers UAC even with `/S`. Non-interactive non-admin session: elevation denied → installer exits non-zero (safe fail). Belt-and-suspenders `UserInfo::GetAccountType` check in `.onInit`. | [Microsoft Q&A UAC+EnableLua](https://learn.microsoft.com/en-us/answers/questions/3852725/nsis-install-program-requir-admin-when-enablelua-i), [NSIS check admin](https://nsis.sourceforge.io/Check_if_the_current_user_is_an_Administrator) |
| UI module | MUI2 with COMPONENTS page (no DIRECTORY page — install dir pinned). ~12 lines of MUI boilerplate vs raw. | [NSIS MUI2 docs](https://nsis.sourceforge.io/Docs/Modern%20UI%202/Readme.html) |
| Apps & Features registry | `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PushKit` (64-bit view; `SetRegView 64`). 8 values: DisplayName, DisplayVersion, Publisher, UninstallString, QuietUninstallString, InstallLocation, DisplayIcon, EstimatedSize, NoModify, NoRepair. Delete the key on uninstall. | [NSIS Add uninstall info](https://nsis.sourceforge.io/Add_uninstall_information_to_Add/Remove_Programs) |
| `RequestExecutionLevel` | `admin` (not `highest`, not `user`, not `none`). | [NSIS RequestExecutionLevel](https://nsis.sourceforge.io/Reference/RequestExecutionLevel) |
| Local iteration | `makensis.exe /V3 /DVERSION=... pushkit.nsi`. CLI is the primary tool; MakeNSISw / VS Code extension are convenience layers. | [NSIS Chapter 3 CLI](https://nsis.sourceforge.io/Docs/Chapter3.html) |
| Anti-patterns avoided | (1) `sc.exe stop` without sync wait → SimpleSC instead. (2) Orphan registry keys on uninstall → explicit `DeleteRegKey`. (3) `RMDir /r $INSTDIR` → delete by name then plain `RMDir`. (4) Plugin Ansi build under NSIS 3.x → Unicode build pinned. | [bojankomazec NSIS locked files](https://www.bojankomazec.com/2011/08/nsis-installer-and-locked-library-files.html), [NSIS Best Practices](https://nsis.sourceforge.io/Best_practices) |

No CVEs in SimpleSC 1.30 (last release 2021-05-02; mature, low-change-rate plugin). NSIS 3.12 changelog notes preliminary ARM64 support; not relevant for this slice (PushKit ships x64 only).

Supply-chain note: `SimpleSC.dll` is a vendored binary. Per intake/shape, SLSA hardening is future scope. Record SHA256 in `backend/installer/README.md` so a maintainer can verify the DLL hasn't been tampered with. No CI signature verification this round.

## Revision History

*(none — first revision)*

## Recommended Next Stage

- **Option A (default):** `/wf implement ship-plan-buildout nsis-installer` — plan is execution-ready. One material blocker (slice AC3 contradiction) flagged; resolution is a single-line edit during implement. Run `/compact` first to clear planning context (sub-agent transcripts, freshness research) before implementation.
- **Option B:** `/wf plan ship-plan-buildout backend-version` — plan the next slice in risk-first order. `backend-version` is xs/mechanical; quick plan. Recommended if the maintainer wants all plans before any implementation.
- **Option C:** Revisit Round 2 service-default question. Switch to "default-checked but `/S` skips" (hybrid) to preserve slice AC3 verbatim. Would require adding an `IfSilent` guard to `SecService`. Not recommended — Option A's path (update slice AC) is cheaper.
- **Option D:** `/wf slice ship-plan-buildout` — revisit slice. Not warranted; the slice scope is clear and only AC3 needs adjustment, which is too small for a slice rewrite.
