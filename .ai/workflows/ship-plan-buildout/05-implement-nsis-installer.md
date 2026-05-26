---
schema: sdlc/v1
type: implement
slug: ship-plan-buildout
slice-slug: nsis-installer
status: complete
stage-number: 5
created-at: "2026-05-25T23:53:56Z"
updated-at: "2026-05-25T23:53:56Z"
metric-files-changed: 4
metric-lines-added: 298
metric-lines-removed: 0
metric-deviations-from-plan: 1
metric-review-fixes-applied: 0
commit-sha: "4fd1fac"
tags:
  - nsis
  - windows
  - installer
  - service-registration
  - simplesc
refs:
  index: 00-index.md
  implement-index: 05-implement.md
  slice-def: 03-slice-nsis-installer.md
  plan: 04-plan-nsis-installer.md
  siblings:
    - 05-implement-commit-hygiene.md
  verify: 06-verify-nsis-installer.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout nsis-installer"
---

# Implement: nsis-installer

## Summary of Changes

- Created `backend/installer/pushkit.nsi` — full-lifecycle NSIS 3.12 installer script with MUI2 UI, upgrade detection via SimpleSC, optional Windows service component (default-checked), Apps & Features registry integration, and clean uninstaller.
- Vendored `backend/installer/plugins/SimpleSC.dll` — Unicode 1.30 build (SHA-256: `1620CDF739F459D1D83411F93648F29DCF947A910CC761E85AC79A69639D127D`).
- Created `backend/installer/README.md` — maintainer reference covering required defines, components, upgrade detection gotchas, silent install behaviour, local validation steps, and plugin provenance.
- Updated root `.gitignore` — added `backend/installer/pushkit-server-setup.exe` and `backend/pushkit-server.exe` as ignored build outputs.
- Updated `03-slice-nsis-installer.md` AC3 — resolved the blocker: silent install now correctly documented to register the service (default-checked component respected by `/S`).

## Files Changed

- `backend/installer/pushkit.nsi` — new; 172-line MUI2 NSIS script. Admin check, upgrade detection, SecCore (required), SecService (optional/default-checked), uninstaller.
- `backend/installer/plugins/SimpleSC.dll` — new binary; Unicode 1.30, 1,110,016 bytes.
- `backend/installer/README.md` — new; maintainer reference doc.
- `.gitignore` — modified; +3 lines adding build output ignores.

## Shared Files (also touched by sibling slices)

- `.gitignore` — created by `commit-hygiene` (ignored `node_modules/`, `dist/`). This slice appended two NSIS build-output ignores. Clean append, no conflict.

## Notes on Design Choices

- **MUI2 with COMPONENTS page, no DIRECTORY page.** Install dir is pinned to `$PROGRAMFILES64\PushKit`. Removing the Directory page prevents the user from installing to an unexpected path (e.g., a directory shared with other software), which would make the uninstaller's explicit-delete-by-name strategy unsafe.
- **`SectionIn RO` on SecCore.** The binary and Start Menu shortcuts are not optional. A components page that let the user skip the binary while still registering the service would leave an unusable service pointing at nothing.
- **Default-checked SecService.** Per PO Round 2 decision. Most users installing PushKit Server want the service. Deselecting on the Components page skips service registration.
- **Non-fatal service registration failure.** `SimpleSC::InstallService` failure (e.g., SCM access denied) shows a warning but does not abort the install. The binary is on disk and usable directly. This avoids a confusing "install rolled back" experience when the binary install succeeded but SCM access failed.
- **No `.onInstSuccess` function.** The service start-on-upgrade is handled inline in `SecService` (upgrade branch calls `SimpleSC::StartService`). Using `.onInstSuccess` would run after both sections, losing the context of whether the service component was selected.
- **`SetRegView 64` in both `.onInit` and `un.onInit`.** Without this, on a 64-bit OS the registry writes land in the 32-bit WOW6432Node hive, causing a phantom uninstall entry that persists even after the app is gone (research anti-pattern #3).

## Deviations from Plan

1. **SimpleSC.dll actual size vs plan estimate.** Plan said "~400 KB"; the extracted DLL is 1,110,016 bytes (~1.06 MB). The plan was citing the compressed zip size (405 KB), not the uncompressed DLL. This is a plan doc inaccuracy — the DLL is the correct Unicode 1.30 build. No functional impact.

## Anything Deferred

- **`IfSilent` guard for SecService.** Would allow silent install to skip service registration. Not implemented — PO chose default-checked and the current behaviour is intentional. Future slice if the requirement changes.
- **SHA-256 CI verification of SimpleSC.dll.** Supply chain hardening is out of scope for v0.x per intake. SHA-256 is recorded in `backend/installer/README.md` for manual verification.
- **`--version` smoke test.** AC11 partial coverage (run `pushkit-server.exe --version` post-install) deferred to `release-orchestration` slice's CI smoke-test job. The binary in scope here may not yet have the `--version` flag (`backend-version` slice).
- **`make build-installer` Makefile target.** Deferred to `release-orchestration` per plan discipline (same pattern as `commit-hygiene`).
- **Root README `## Backend installer` section.** Owned by `release-orchestration` slice per plan cross-reference.

## Known Risks / Caveats

- **NSIS version requirement.** Script requires NSIS 3.12+ (CVE-2025-43715 fixed in 3.11; second elevation fix in 3.12). Local builds on NSIS ≤ 3.10 are a security risk; CI must install 3.12 via marketplace action (`release-orchestration` slice).
- **Installer size NFR.** Input binary `backend/pushkit-server.exe` is 27.2 MB (28,536,345 bytes). LZMA solid compression typically nets 30–40% reduction → expected ~17–20 MB. Actual size must be checked during verify (AC1). If > 25 MB, flag in handoff — not a hard gate.
- **SimpleSC::ExistsService inversion.** Returns 0 when service EXISTS. Script has the correct `${If} $0 == 0` guard with a comment. Reviewer must verify this carefully.
- **Local validation requires Windows 11 + NSIS 3.12.** Cannot be verified in the current Claude Code session. All interactive ACs (AC1–AC5) require the maintainer to run on their Windows machine.
- **`File "..\pushkit-server.exe"` path contract.** The `release-orchestration` CI job must place the compiled binary at `backend/pushkit-server.exe` before invoking `makensis backend/installer/pushkit.nsi`. Path mismatch will cause a compile-time error (no binary found).

## Freshness Research

Plan-stage freshness sub-agent (run 2026-05-25) is current. Key pins:
- NSIS 3.12 (CVE-2025-43715 patched). No new advisories since.
- SimpleSC Unicode 1.30 (last release 2021-05-02; no CVEs found).
No re-search needed at implement stage — no new external APIs or version-sensitive behaviour introduced.

## Recommended Next Stage

- **Option A (default):** `/wf verify ship-plan-buildout nsis-installer` — all ACs require interactive Windows validation on the maintainer's machine. Run `/compact` before verify to clear implementation context.
- **Option B:** `/wf review ship-plan-buildout nsis-installer` — skip verify if the maintainer wants to defer the Windows validation to a CI run on the PR.
