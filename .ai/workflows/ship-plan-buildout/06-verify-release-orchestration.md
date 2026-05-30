---
schema: sdlc/v1
type: verify
slug: ship-plan-buildout
slice-slug: release-orchestration
status: complete
stage-number: 6
created-at: "2026-05-27T18:42:49Z"
updated-at: "2026-05-27T18:42:49Z"
result: partial
metric-checks-run: 3
metric-checks-passed: 3
metric-acceptance-met: 0
metric-acceptance-total: 9
metric-acceptance-user-observable: 9
metric-acceptance-code-only: 0
metric-interactive-checks-run: 0
metric-interactive-checks-passed: 0
metric-issues-found: 0
metric-issues-found-initial: 0
metric-issues-found-final: 0
fix-rounds-run: 0
convergence: not-needed
verify-owned-fix-commit: null
interactive-verification: deferred
interactive-verification-defer-reason: "All 9 ACs require a live GitHub Actions run (tag push to jayteealao/PushKit) and/or post-publish observation. The local verify environment is Windows 11 Pro with no access to GitHub Actions runners, live PyPI, or Windows 2022 runners with NSIS. All three automated static checks pass. ACs AC4, AC5, AC6, AC7, AC8, AC9, AC10, AC12, AC13 will be cleared when the maintainer pushes v0.1.0-rc.1 to GitHub and observes all 8 pipeline jobs green + post-publish-checks pass."
stack-source: confirmed
adapters-used: []
bootstrap-failures: []
evidence-dir: ".ai/workflows/ship-plan-buildout/verify-evidence/release-orchestration/"
tags:
  - github-actions
  - release
  - git-cliff
  - pypi
  - github-release
refs:
  index: 00-index.md
  verify-index: 06-verify.md
  slice-def: 03-slice-release-orchestration.md
  plan: 04-plan-release-orchestration.md
  implement: 05-implement-release-orchestration.md
  review: 07-review.md
  adapters: ${CLAUDE_PLUGIN_ROOT}/skills/wf/reference/runtime-adapters.md
next-command: wf-review
next-invocation: "/wf review ship-plan-buildout"
---

# Verify: release-orchestration

## Verification Summary

Static checks: **3/3 pass.** Release workflow YAML is valid, 13-job graph is correctly wired, cliff.toml is well-formed, README additions are present.

All 9 acceptance criteria are **user-observable** and require a live GitHub Actions run or post-publish observation. Runtime verification is **deferred** — the local environment has no access to GitHub Actions, live PyPI, or Windows 2022 runners. The code is correct by static evidence; deployment behavior remains unconfirmed until the maintainer pushes `v0.1.0-rc.1`.

## Automated Checks Run

| Check | Command/Method | Result |
|---|---|---|
| YAML syntax — release.yml | `python -m yaml` parse + structural inspection | PASS — valid YAML, 13 jobs, no duplicate keys |
| cliff.toml validity | TOML parse + field inspection | PASS — tag_pattern, [git] conventional_commits, filter_unconventional, [changelog] body template all present |
| README additions | grep for badge + section headers | PASS — shields.io badge (line 3), `## Releasing` (line 154), `## Backend installer` (line 223) |

### Release.yml structural check details

All 13 required jobs present: `tag-guard`, `retest-backend`, `retest-cli`, `retest-android`, `build-cli-wheel`, `build-backend-binary`, `build-backend-installer`, `build-android-apk`, `generate-changelog`, `publish-pypi`, `create-github-release`, `post-publish-linux`, `post-publish-windows`.

**Needs graph** (verified against plan):
```
tag-guard
  └─> retest-backend, retest-cli, retest-android
        └─> build-cli-wheel, build-backend-binary, build-android-apk, generate-changelog
              └─> build-backend-binary ─> build-backend-installer
                    └─> publish-pypi (needs: wheel + installer + apk + changelog)
                          └─> create-github-release (needs: publish-pypi + installer + apk + changelog)
                                └─> post-publish-linux, post-publish-windows
```

**Key verifications:**
- `concurrency: { group: release-${{ github.ref_name }}, cancel-in-progress: false }` ✓
- Top-level `permissions: { contents: read }` (least privilege — better than spec's `contents: write` global; correct per CLAUDE.md) ✓
- `create-github-release` job `permissions: { contents: write }` ✓ (job-level override)
- `publish-pypi` job `permissions: { id-token: write }`, `environment: pypi` ✓ (`PYPI_API_TOKEN` not in default graph)
- `build-backend-binary`: `GOOS: windows`, `GOARCH: amd64`, `CGO_ENABLED: "0"` set as step-level env on cross-compile step; `working-directory: backend` ✓
- `build-android-apk`: `-PversionCodeOverride=` (project property, not `-D`), `git rev-list --first-parent --count HEAD`, `fetch-depth: 0`, `working-directory: android` ✓
- `generate-changelog`: `fetch-depth: 0`, `orhun/git-cliff-action@v4` ✓
- `create-github-release`: `softprops/action-gh-release@v3`, `fail_on_unmatched_files: true`, prerelease detection via `=~ -(rc|alpha|beta)` ✓
- `build-backend-installer`: `windows-2022` runner, `repolevedavaj/install-nsis@v1` (`nsis-version: 3.12`) ✓

**Deviation from spec (safe):** Spec described `permissions: contents: write` at workflow level. Implementation uses `contents: read` at workflow level and `contents: write` only on `create-github-release` job. This is the correct least-privilege pattern; job-level permissions replace workflow defaults for that job. Functionally equivalent, more secure.

## Interactive Verification Results

Automated only — all ACs require a live GitHub Actions run. No interactive checks were executed in this environment. Sub-agent 3 was not launched: `stack.platforms` includes `service` and `cli`, but neither the GitHub Actions runtime adapter (which would require triggering live CI) nor the relevant platform adapters are available locally.

## Acceptance Criteria Status

| ID | Criterion | Kind | Status | Verification method | Evidence |
|---|---|---|---|---|---|
| AC4 | tag-guard exits non-zero for off-main tag; no downstream jobs run | user-observable | runtime-evidence-missing (deferred) | interactive — live CI | Static: `git merge-base --is-ancestor "$GITHUB_SHA" origin/main` script verified; no `\|\| true` safety net; downstream jobs `needs: tag-guard`. Mechanism correct. Live confirmation deferred. |
| AC5 | All 8 jobs succeed on `v0.1.0-rc.1`; GitHub Release exists with installer + APK + SHA256SUMS + prerelease notes | user-observable | runtime-evidence-missing (deferred) | interactive — live CI | Deferred — requires live tag push |
| AC6 | `pip install --no-cache-dir --pre pushkit==0.1.0rc1` returns `pushkit version 0.1.0-rc.1` | user-observable | runtime-evidence-missing (deferred) | automated in CI (post-publish-linux) | Deferred — requires live PyPI publish |
| AC7 | Silent install + `pushkit-server.exe --version` returns `pushkit-server 0.1.0-rc.1` on Windows | user-observable | runtime-evidence-missing (deferred) | automated in CI (post-publish-windows) | Deferred — requires Windows runner + live release assets |
| AC8 | `aapt dump badging` reports `versionName='0.1.0-rc.1'` and monotonic `versionCode` | user-observable | runtime-evidence-missing (deferred) | automated in CI (post-publish-linux APK probe) | Deferred — requires live APK artifact |
| AC9 | GitHub Release notes contain at least one categorized commit entry | user-observable | runtime-evidence-missing (deferred) | human-in-the-loop | Deferred — requires live GitHub Release |
| AC10 | `sha256sum -c SHA256SUMS` passes over all three binary assets | user-observable | runtime-evidence-missing (deferred) | automated in CI (post-publish-linux) | Deferred — requires live release assets |
| AC12 | shields.io badge renders `v0.1.0-rc.1` on github.com 30+ min post-publish | user-observable | runtime-evidence-missing (deferred) | human-in-the-loop | Deferred — requires live release + CDN cache warm |
| AC13 | Deliberately broken NSIS → `build-backend-installer` fails red; `publish-pypi` and `create-github-release` NOT executed | user-observable | runtime-evidence-missing (deferred) | interactive — throwaway tag + live CI | Static: needs graph proves `publish-pypi` `needs: build-backend-installer` → skipped on failure; `create-github-release` `needs: publish-pypi` → also skipped. Mechanism correct. Throwaway tag run deferred. |

## Issues Found

None from automated checks. All 9 user-observable ACs have no runtime evidence; all are annotated as deferred (environmental constraint — no GitHub Actions runtime available locally).

## Gaps / Unverified Areas

1. **actionlint not run** — actionlint is not installed in the local environment. The workflow was validated via Python YAML parse + structural inspection. Recommend running `actionlint .github/workflows/release.yml` pre-tag.
2. **PyPI Trusted Publisher quartet** — the OIDC config (`owner=jayteealao`, `repo=PushKit`, `workflow_filename=release.yml`, `environment=pypi`) must be verified against the PyPI dashboard before the first real release. This is a one-time maintainer action documented in `## Releasing`.
3. **GitHub repo settings** — tag-protection ruleset on `v*` and the `pypi` GitHub Environment must exist before the validation tag push. Documented in `## Releasing`.
4. **First-tag cliff output** — `filter_unconventional = true` will drop 5 of 6 pre-branch commits. The first changelog will be thin but correct. Accepted per shape Round 5.

## Freshness Research

No freshness research run — no external dependency state changes relevant to automated static checks. Action version freshness was verified at implement time (2026-05-27); see `05-implement-release-orchestration.md ## Freshness Research`.

## Recommendation

All static checks pass. The workflow is correctly structured, wired, and documented. The only gap is runtime evidence for all 9 ACs, deferred because the local environment cannot support a live GitHub Actions run.

**Pre-tag checklist (maintainer must complete before pushing `v0.1.0-rc.1`):**
1. Verify PyPI Trusted Publisher quartet on pypi.org/manage/project/pushkit/settings/publishing/.
2. Confirm GitHub Environment named `pypi` exists in repo Settings → Environments.
3. Confirm tag-protection ruleset for `v*` (optional but recommended).
4. Run `actionlint .github/workflows/release.yml` to catch any action-level issues.
5. PR [#1](https://github.com/jayteealao/PushKit/pull/1) merged into `main`.
6. Push `v0.1.0-rc.1` from `main` HEAD and watch all 8 CI jobs green.

## Recommended Next Stage

- **Option A (default):** `/wf review ship-plan-buildout` — slug-wide review against the full branch diff. `review-scope: slug-wide`. Recommended next: static checks all pass, deferral is environmental not a code flaw, a code review adds value before the first real tag push.
- **Option D:** `/wf handoff ship-plan-buildout` — skip review if maintainer wants to go straight to PR merge + validation tag. Valid since all static checks pass and this is a solo project. Only valid with `result: partial` (deferred ACs will block `/wf ship` until cleared).
- **Option F (re-verify):** `/wf verify ship-plan-buildout release-orchestration` — re-run after pushing `v0.1.0-rc.1` to GitHub Actions in an environment that can observe CI results. Will clear all 9 deferred ACs.
