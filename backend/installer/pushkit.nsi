; =============================================================================
; PushKit Server Installer
; =============================================================================
; Build: makensis /V3 /DVERSION=x.y.z backend/installer/pushkit.nsi
; Output: backend/installer/pushkit-server-setup.exe
; Requires: NSIS 3.12+, SimpleSC Unicode 1.30 (vendored in plugins/)
; =============================================================================

; --- Defines ---

!define APP_NAME           "PushKit Server"
!define APP_PUBLISHER      "jayteealao"
!define APP_URL            "https://github.com/jayteealao/PushKit"
!define APP_REG_KEY        "Software\Microsoft\Windows\CurrentVersion\Uninstall\PushKit"
!define SERVICE_NAME       "PushKitServer"
!define SERVICE_DISPLAY    "PushKit Server"
!define SERVICE_DESCRIPTION "PushKit backend service. See https://github.com/jayteealao/PushKit"

; VERSION supplied at compile time: makensis /DVERSION=x.y.z
; Falls back to dev sentinel if omitted — compile succeeds, metadata will be wrong.
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

; --- Plugin + includes ---

!addplugindir "plugins"
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"   ; for ${GetSize} (EstimatedSize registry value)

; --- Globals ---

Var WasInstalled        ; "1" if upgrade, "0" if fresh install

; --- MUI2 pages ---

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; =============================================================================
; .onInit — admin check + upgrade detection
; Runs before any UI is shown. Quit (not Abort) here exits cleanly with no UI.
; =============================================================================

Function .onInit
  SetRegView 64

  ; Require administrator. Belt-and-suspenders check in addition to
  ; RequestExecutionLevel admin — covers EnableLua=0 edge cases where UAC
  ; is bypassed but the process still runs non-elevated.
  UserInfo::GetAccountType
  Pop $0
  ${If} $0 != "Admin"
    MessageBox MB_ICONSTOP \
      "PushKit installer requires administrator rights.$\nRight-click and choose 'Run as administrator'." \
      /SD IDOK
    Quit
  ${EndIf}

  ; Upgrade detection: SimpleSC::ExistsService returns 0 when the service
  ; EXISTS, non-zero when it does NOT. This inverted semantics is documented
  ; on the NSIS Wiki SimpleSC page — the ${If} $0 == 0 test is intentional.
  StrCpy $WasInstalled "0"
  SimpleSC::ExistsService "${SERVICE_NAME}"
  Pop $0
  ${If} $0 == 0
    StrCpy $WasInstalled "1"
  ${EndIf}
FunctionEnd

; =============================================================================
; Section "PushKit Server (required)" — always installed, cannot be unchecked
; =============================================================================

Section "PushKit Server (required)" SecCore
  SectionIn RO          ; read-only — cannot be unchecked on components page

  ; Upgrade path: stop the running service before replacing the binary.
  ; SimpleSC::StopService with poll=1 waits until the process releases file
  ; handles (up to 30 s), preventing "file in use" errors on binary replace.
  ; Error code 1062 = service not running — treat as success.
  ${If} $WasInstalled == "1"
    SimpleSC::StopService "${SERVICE_NAME}" 1 30
    Pop $0
    ${If} $0 != 0
    ${AndIf} $0 != 1062
      MessageBox MB_ICONSTOP \
        "Cannot stop the existing PushKitServer service (code $0).$\nStop the service manually and retry." \
        /SD IDOK
      Abort
    ${EndIf}
  ${EndIf}

  ; Install binary. Path is relative to this .nsi file: ".." resolves to
  ; backend/, so the source is backend/pushkit-server.exe — the same path
  ; produced by `go build -o backend/pushkit-server.exe` locally and by the
  ; CI cross-compile job that places the artifact there before calling makensis.
  SetOutPath "$INSTDIR"
  File "..\pushkit-server.exe"

  ; Start Menu shortcuts
  CreateDirectory "$SMPROGRAMS\PushKit"
  CreateShortcut  "$SMPROGRAMS\PushKit\PushKit Server.lnk"    "$INSTDIR\pushkit-server.exe"
  CreateShortcut  "$SMPROGRAMS\PushKit\Uninstall PushKit.lnk" "$INSTDIR\uninstall.exe"

  ; Apps & Features registry entry (64-bit hive, SetRegView 64 already active)
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegStr   HKLM "${APP_REG_KEY}" "DisplayName"          "${APP_NAME}"
  WriteRegStr   HKLM "${APP_REG_KEY}" "DisplayVersion"       "${VERSION}"
  WriteRegStr   HKLM "${APP_REG_KEY}" "Publisher"            "${APP_PUBLISHER}"
  WriteRegStr   HKLM "${APP_REG_KEY}" "URLInfoAbout"         "${APP_URL}"
  WriteRegStr   HKLM "${APP_REG_KEY}" "InstallLocation"      "$INSTDIR"
  WriteRegStr   HKLM "${APP_REG_KEY}" "DisplayIcon"          "$INSTDIR\pushkit-server.exe,0"
  WriteRegStr   HKLM "${APP_REG_KEY}" "UninstallString"      "$\"$INSTDIR\uninstall.exe$\""
  WriteRegStr   HKLM "${APP_REG_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
  WriteRegDWORD HKLM "${APP_REG_KEY}" "EstimatedSize"        $0
  WriteRegDWORD HKLM "${APP_REG_KEY}" "NoModify"             1
  WriteRegDWORD HKLM "${APP_REG_KEY}" "NoRepair"             1

  WriteUninstaller "$INSTDIR\uninstall.exe"
SectionEnd

; =============================================================================
; Section "Register Windows service" — optional, default-checked
; =============================================================================
; NOTE: Because this section is default-checked, silent install (/S) WILL
; register the service. This diverges from an earlier draft of AC3 that said
; silent install would skip service registration. The current behaviour is
; intentional — see backend/installer/README.md "Silent install" for details.

Section "Register Windows service" SecService

  ${If} $WasInstalled == "0"
    ; Fresh install: create the service.
    ; Service type 16 = SERVICE_WIN32_OWN_PROCESS
    ; Start type  3  = SERVICE_DEMAND_START (manual / demand)
    SimpleSC::InstallService "${SERVICE_NAME}" "${SERVICE_DISPLAY}" \
      "16" "3" "$INSTDIR\pushkit-server.exe" "" "" ""
    Pop $0
    ${If} $0 != 0
      MessageBox MB_ICONEXCLAMATION \
        "Service registration failed (code $0).$\npushkit-server.exe is installed and usable without the service.$\nRun 'sc create' manually or re-run the installer as administrator." \
        /SD IDOK
      ; Non-fatal: binary is on disk and can be used without the service.
    ${Else}
      SimpleSC::SetServiceDescription "${SERVICE_NAME}" "${SERVICE_DESCRIPTION}"
      Pop $0
    ${EndIf}
  ${Else}
    ; Upgrade: service already exists. Restart it now that the binary is replaced.
    ; Non-zero return is non-fatal — user can start manually.
    SimpleSC::StartService "${SERVICE_NAME}" "" 30
    Pop $0
  ${EndIf}

SectionEnd

; --- Section descriptions for MUI Components page ---

LangString DESC_SecCore    ${LANG_ENGLISH} \
  "Installs pushkit-server.exe to %ProgramFiles%\PushKit and creates Start Menu shortcuts. Required."
LangString DESC_SecService ${LANG_ENGLISH} \
  "Registers PushKitServer as a Windows service (start type: demand / manual). Deselect to skip service registration."

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecCore}    $(DESC_SecCore)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecService} $(DESC_SecService)
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; =============================================================================
; Uninstaller
; =============================================================================

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
  ; ExistsService inverted semantics: 0 = exists.
  SimpleSC::ExistsService "${SERVICE_NAME}"
  Pop $0
  ${If} $0 == 0
    SimpleSC::StopService   "${SERVICE_NAME}" 1 30
    Pop $0
    SimpleSC::RemoveService "${SERVICE_NAME}"
    Pop $0
  ${EndIf}

  ; Delete files by name — never RMDir /r $INSTDIR.
  ; Plain RMDir only succeeds if the directory is empty, which protects
  ; against $INSTDIR being set to an unexpected path.
  Delete "$INSTDIR\pushkit-server.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir  "$INSTDIR"

  Delete "$SMPROGRAMS\PushKit\PushKit Server.lnk"
  Delete "$SMPROGRAMS\PushKit\Uninstall PushKit.lnk"
  RMDir  "$SMPROGRAMS\PushKit"

  DeleteRegKey HKLM "${APP_REG_KEY}"

SectionEnd
