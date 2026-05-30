---
schema: sdlc/v1
type: review
slug: ship-plan-buildout
review-scope: slug-wide
slice-slug: ""
status: complete
stage-number: 7
created-at: "2026-05-30T15:08:53Z"
updated-at: "2026-05-30T20:25:00Z"
verdict: ship-with-caveats
commands-run: [correctness, security, code-simplification, ci, release, supply-chain, infra-security, reliability, testing, maintainability, docs]
metric-commands-run: 11
metric-findings-total: 41
metric-findings-raw: 87
metric-findings-blocker: 0
metric-findings-high: 12
metric-findings-med: 19
metric-findings-low: 8
metric-findings-nit: 2
metric-issues-found-initial: 41
metric-issues-found-final: 8
metric-fix-decisions: 33
metric-fix-patched: 33
fix-rounds-run: 3
convergence: converged
review-owned-fix-commit: "5a8fb79, c8011d9, 21bf6c8, 78703a5"
tags: [release-engineering, ci-cd, security, supply-chain, nsis, android, go]
refs:
  index: 00-index.md
  shape: 02-shape.md
  slice-index: 03-slice.md
  implements: [05-implement-commit-hygiene.md, 05-implement-nsis-installer.md, 05-implement-backend-version.md, 05-implement-android-versioning.md, 05-implement-release-orchestration.md]
  verifies: [06-verify.md, 06-verify-commit-hygiene.md, 06-verify-nsis-installer.md, 06-verify-backend-version.md, 06-verify-android-versioning.md, 06-verify-release-orchestration.md]
  sub-reviews: [07-review-correctness.md, 07-review-security.md, 07-review-code-simplification.md, 07-review-ci.md, 07-review-release.md, 07-review-supply-chain.md, 07-review-infra-security.md, 07-review-reliability.md, 07-review-testing.md, 07-review-maintainability.md, 07-review-docs.md]
next-command: wf-handoff
next-invocation: "/wf handoff ship-plan-buildout"
---

# Review: ship-plan-buildout (slug-wide)

## Verdict

**Ship with caveats.**

No blockers. The branch implements the full tag-and-walk-away release pipeline plus three additional pieces of work that landed on the same branch (a CLI refactor adding `--json`/env-var support and the `push`→`pushkit` rename, an S3-compatibility SigV4 middleware fix, and an Android network-security config). 11 review dimensions surfaced 87 raw findings (41 after dedup): 0 BLOCKER, 12 HIGH, 19 MED, 8 LOW, 2 NIT. All 12 HIGH findings and the 4 highest-value MED groups were triaged **Fix** and patched in this round (commit `5a8fb79`); backend and CLI build/vet/test are green after the fixes. The remaining caveats are deferred MED/LOW/NIT polish items plus the still-open pre-tag runtime verification (the release pipeline has never been exercised by a real `v*` tag — every release AC remains deferred from verify).

The most consequential fixes: SHA-pinned every GitHub Action (the pipeline holds PyPI OIDC `id-token: write` + `contents: write`), made released Android builds HTTPS-only (the prior config shipped cleartext to hardcoded personal IPs), closed an unquoted-service-path privilege-escalation vector in the Windows installer, and fixed a post-publish step that could install/verify the wrong PyPI package.

## Domain Coverage

| Domain | Command | Status |
|--------|---------|--------|
| Logic / invariants | `correctness` | Issues (3 HIGH) |
| Security | `security` | Issues (3 HIGH) |
| Reuse / simplicity | `code-simplification` | Issues (2 HIGH) |
| CI/CD pipelines | `ci` | Issues (1 HIGH) |
| Release engineering | `release` | Issues (3 HIGH) |
| Supply chain | `supply-chain` | Issues (3 HIGH) |
| Installer infra security | `infra-security` | Issues (3 HIGH) |
| Reliability | `reliability` | Issues (3 HIGH) |
| Test coverage | `testing` | Issues (3 HIGH) |
| Maintainability | `maintainability` | Issues (3 HIGH) |
| Documentation | `docs` | Issues (2 HIGH) |

## All Findings (Deduplicated)

| ID | Sev | Conf | Source | File:Line | Issue |
|----|-----|------|--------|-----------|-------|
| H-01 | HIGH | High | security/supply-chain/ci/release | `.github/workflows/*.yml` | All GitHub Actions on mutable tags; pipeline holds OIDC + contents:write — tag-move/compromise hijacks release |
| H-02 | HIGH | High | security | `android/.../xml/network_security_config.xml` | Cleartext HTTP to hardcoded personal IPs ships in the public-release APK; leaks network topology |
| H-03 | HIGH | High | supply-chain/security/infra-security | `backend/installer/plugins/SimpleSC.dll` | Vendored DLL executed with elevation; SHA documented but not verified in CI before makensis |
| H-04 | HIGH | High | correctness/ci/release | `release.yml` post-publish | `packaging` imported before install + version string-interpolated; empty version → wrong package installed/verified |
| H-05 | HIGH | High | infra-security | `backend/installer/pushkit.nsi:156` | Unquoted service binPath (LocalSystem) — classic unquoted-service-path privilege escalation |
| H-06 | HIGH | High | reliability | `cli/internal/client/client.go:24`, `cli/cmd/download.go:73` | CLI HTTP client + bare http.Get have no timeout — hang forever on a dead endpoint |
| H-07 | HIGH | High | correctness/code-simplification/maintainability | `cli/main.go`, `cli/cmd/root.go` | `--json` failure prints two stderr lines (first not JSON); `outputError` dead code, logic re-inlined |
| H-08 | HIGH | High | release | `release.yml:303` | `download-artifact` brace pattern `{a,b,c}` not expanded by glob → post-publish download matches nothing |
| H-09 | HIGH | Med | release | `release.yml:328` | `make_latest` derived from a fragile string expression; a prerelease could be promoted to latest |
| H-10 | HIGH | High | testing | `backend/internal/s3/client.go` | S3 SigV4 header-strip/checksum middleware has zero tests; regressions only surface as runtime SignatureDoesNotMatch |
| H-11 | HIGH | High | testing | `cli/cmd/root.go` getClient | flag>env>config credential precedence untested; one misplaced `if` silently inverts priority |
| H-12 | HIGH | High | docs/maintainability | `README.md`, `backend/installer/README.md` | README builds/invokes `push` (binary is `pushkit`) and says installer uses `sc.exe` (it uses SimpleSC) |
| M-01 | MED | High | docs/maintainability | `README.md` | Go prerequisite says >=1.22 but go.mod/CI require 1.24 |
| M-02 | MED | High | docs/release | `README.md` | No runnable `pip install --pre pushkit==0.1.0rc1` prerelease example |
| M-03 | MED | High | maintainability/docs | `cli/internal/config/config.go`, `cli/cmd/config.go` | Config dir still named `s3push` after rename to `pushkit` |
| M-04 | MED | Med | correctness/reliability/maintainability | `backend/internal/s3/client.go:75` | Middleware-insert error swallowed via `Contains(err,"not found")` magic string |
| M-05 | MED | Med | correctness/reliability | `backend/internal/s3/client.go:136` | HeadObject not-found via bare `Contains(err,"404")` — too broad, can mask real errors |
| M-06 | MED | High | ci/maintainability | `release.yml` build-cli-wheel | Wheel built on go 1.22 (go-version-file) while tested on 1.24 |
| M-07 | MED | Med | correctness | `release.yml` prerelease regex | `-(rc|alpha|beta)` matches mid-tag (e.g. `v1.0.0-backport-rc-fix`) |
| M-08 | MED | Med | correctness | `backend/cmd/server/main.go` | `flag.Parse()` exits 2 on any unknown server flag — breaks service-manager extra args |
| M-09 | MED | Med | correctness | `android/app/build.gradle.kts` | Malformed versionCode override silently defaults to 1 (probe only checks `[0-9]+`) |
| M-10 | MED | Med | security | `cli/cmd/root.go` | `--api-key` flag exposes the key in the process list |
| M-11 | MED | Med | security | backend HTTP server | Serves plaintext HTTP on all interfaces; no TLS option |
| M-12 | MED | Low | security | `release.yml` pypi-publish | `attestations: false` opts out of PEP 740 provenance |
| M-13 | MED | Med | ci | both workflows | No dependency scanning / SAST (govulncheck/gosec) before PyPI publish |
| M-14 | MED | High | code-simplification | `release.yml` vs `ci.yml` | `retest-*` jobs are verbatim copies of ci.yml test jobs (no composite action) |
| M-15 | MED | Med | code-simplification/maintainability | `release.yml` | `${GITHUB_REF_NAME#v}` strip copy-pasted 3x; `VERSION` vs `VERSION_STRIPPED` naming drift |
| M-16 | MED | Med | code-simplification | `cli/cmd/upload.go`, `download.go` | Progress-reader conditional block duplicated |
| M-17 | MED | Low | code-simplification/maintainability | backend vs cli version handling | Two patterns for the same concern (stdlib flag vs Cobra .Version) |
| M-18 | MED | Med | security/ci | `release.yml` | Step outputs interpolated directly into `run:` (actionlint flag; injection-pattern) |
| M-19 | MED | Med | reliability | `release.yml` graph | publish-pypi success + create-github-release failure → orphan PyPI release, no rollback |
| L-01 | LOW | Med | correctness | `cli/cmd/download.go` | Server `Content-Disposition` filename used as write path without `filepath.Base` |
| L-02 | LOW | Med | correctness/infra-security/reliability | `backend/installer/pushkit.nsi:209` | Uninstaller ignores `StopService` return before `RemoveService` |
| L-03 | LOW | Low | security | `android/.../network_security_config.xml` | `includeSubdomains="true"` on bare IPs is meaningless (subsumed by H-02 fix) |
| L-04 | LOW | Low | ci/code-simplification | `release.yml` gradle cache | `cache-read-only` always true on tag pushes — release builds never refresh cache |
| L-05 | LOW | Low | ci | both workflows | No `timeout-minutes` on jobs (6h default applies) |
| L-06 | LOW | Low | testing | `cli/internal/config/config_test.go` | Test bypasses real Load/Save (writes/reads raw JSON) |
| L-07 | LOW | Low | infra-security | `backend/installer/pushkit.nsi` SecService | Default-checked service section means silent `/S` registers the service (intentional/documented) |
| L-08 | LOW | Low | maintainability | `backend/internal/s3/client.go` | Deeply nested middleware closure reduces readability |
| N-01 | NIT | Low | maintainability | `cliff.toml` | Undocumented HTML-comment sort trick |
| N-02 | NIT | Low | ci | both workflows | `ubuntu-latest`/runner image drift; no cheap pre-gate before the Android build |

**Total:** BLOCKER: 0 | HIGH: 12 | MED: 19 | LOW: 8 | NIT: 2
*(After dedup: 41 findings merged from 87 raw findings across 11 commands.)*

## Triage Decisions

| ID | Sev | Source | Decision | Notes |
|----|-----|--------|----------|-------|
| H-01 | HIGH | supply-chain | fix | SHA-pin all actions + DLL/wrapper hash verification |
| H-02 | HIGH | security | fix | HTTPS-only release; cleartext debug-only; personal IPs removed |
| H-03 | HIGH | supply-chain | fix | Build-time Get-FileHash check before makensis (bundled with H-01 fix) |
| H-04 | HIGH | correctness | fix | Install packaging, env-pass version, non-empty guard |
| H-05 | HIGH | infra-security | fix | Quote binPath via NSIS `$\"` escapes |
| H-06 | HIGH | reliability | fix | Timeouts on shared client + presigned download |
| H-07 | HIGH | correctness | fix | SilenceErrors + single outputError path |
| H-08 | HIGH | release | fix | Download artifacts by name |
| H-09 | HIGH | release | fix | Covered by H-04/H-08 release-step hardening pass |
| H-10 | HIGH | testing | fix | Extract + unit-test the SigV4 middleware |
| H-11 | HIGH | testing | fix | Extract resolveCredentials + table test |
| H-12 | HIGH | docs | fix | README binary name + installer mechanism corrected |
| M-01 | MED | docs | fix | Go 1.24 prerequisite (Docs group) |
| M-02 | MED | docs | fix | Added pip --pre example (Docs group) |
| M-03 | MED | maintainability | fix | Config dir → pushkit with legacy fallback (Config-dir group) |
| M-04 | MED | reliability | fix | Typed/prefix check (Robust-error group) |
| M-05 | MED | reliability | fix | Typed `errors.As` ResponseError 404 check (Robust-error group) |
| M-06 | MED | ci | fix | Pin go 1.24 in build-cli-wheel (Toolchain group) |
| M-07 | MED | correctness | fix (R2) | Anchored prerelease regex to suffix `-(rc\|alpha\|beta)(\.[0-9]+)?$` |
| M-08 | MED | correctness | fix (R2) | Server tolerates unknown flags (ContinueOnError FlagSet; no exit 2) |
| M-09 | MED | correctness | fix (R2) | Malformed versionCode/versionName override now fails the build |
| M-10 | MED | security | fix (R2) | Warn on `--api-key` CLI use (process-list); steer to env/config |
| M-11 | MED | security | fix (R2) | Optional TLS listen mode (off by default; TLS_CERT_FILE/TLS_KEY_FILE) |
| M-12 | MED | security | fix (R2) | Re-enabled PyPI PEP 740 / Sigstore attestations |
| M-13 | MED | ci | fix (R2) | Added govulncheck pre-merge gate for both modules |
| M-14 | MED | code-simplification | fix (R2) | Composite action `.github/actions/go-test` dedups retest/ci jobs |
| M-15 | MED | code-simplification | fix (R2) | New `version` job single-sources the version-strip |
| M-16 | MED | code-simplification | fix (R2) | Extracted `progress.MaybeNewReader` shared helper |
| M-17 | MED | maintainability | fix (R2) | Documented intentional server/CLI version-output divergence |
| M-18 | MED | security | fix (R2) | Moved step outputs into `env:` indirection |
| M-19 | MED | reliability | fix (R2) | Operator recovery note for PyPI-published-without-Release |
| L-01 | LOW | correctness | fix (R2) | `filepath.Base` sanitization on server-provided download filename |
| L-02 | LOW | infra-security | fix (R2) | Uninstaller checks StopService result before RemoveService |
| L-03..L-08 | LOW | various | defer | Polish; not prompted (subsumed/low-value) |
| N-01..N-02 | NIT | various | defer | Polish; not prompted |

## Fix Status

| ID | Sev | Source | Sub-agent outcome | Notes |
|----|-----|--------|-------------------|-------|
| H-01 | HIGH | supply-chain | Patched | All 12 actions SHA-pinned (`# <ref>` comments); install-nsis pinned to v1.2.0 SHA (no floating `v1` exists) |
| H-02 | HIGH | security | Patched | main config `cleartextTrafficPermitted=false`; new `src/debug/res/xml` allows only 10.0.2.2/localhost; personal IPs removed |
| H-03 | HIGH | supply-chain | Patched | PowerShell Get-FileHash gate before makensis against README SHA `1620CDF7…` |
| H-04 | HIGH | correctness | Patched | `pip install packaging`, version via `$STRIPPED` env, non-empty guard |
| H-05 | HIGH | infra-security | Patched | binPath now `"$\"$INSTDIR\pushkit-server.exe$\""` (8 args preserved) |
| H-06 | HIGH | reliability | Patched | client Timeout 60s; download via NewRequestWithContext + 10m client |
| H-07 | HIGH | correctness | Patched | `SilenceErrors`/`SilenceUsage`; Execute() calls outputError; dup removed from main.go |
| H-08 | HIGH | release | Patched | Three named `download-artifact` steps into `release-assets/` |
| H-09 | HIGH | release | Patched | Resolved by the H-04/H-08 release-step hardening |
| H-10 | HIGH | testing | Patched | `stripSDKHeaders`/`stripSDKHeadersMiddleware` extracted; 3 deterministic tests added |
| H-11 | HIGH | testing | Patched | `resolveCredentials` extracted; 7-case table test in cli/cmd/root_test.go |
| H-12 | HIGH | docs | Patched | README `push`→`pushkit`; installer mechanism → SimpleSC plugin |
| M-01 | MED | docs | Patched | Go >=1.24 |
| M-02 | MED | docs | Patched | `pip install --pre pushkit==0.1.0rc1` example added |
| M-03 | MED | maintainability | Patched | configDir → pushkit; legacyConfigDir fallback on read; docs updated |
| M-04 | MED | reliability | Patched | `HasPrefix` on a named smithy constant |
| M-05 | MED | reliability | Patched | `errors.As(*smithyhttp.ResponseError)` HTTPStatusCode==404 |
| M-06 | MED | ci | Patched | build-cli-wheel pinned to go 1.24 |

**Round count:** 1
**Convergence:** converged
**Initial findings:** 41 → **Final findings:** 29 (12 Fix decisions, all patched)
**Commit:** `5a8fb79` — `fix: harden release pipeline and resolve pre-release review findings` (19 files; not pushed)

Post-fix gates: backend `go build`/`go vet`/`go test ./...` green (S3 middleware + version-flag tests pass); CLI `go build`/`go test ./...` green; both workflows parse as valid YAML; all `uses:` SHA-pinned.

Adjudicated conflict: the security reviewer reported the NSIS service binPath as safe, but a direct read of `pushkit.nsi` confirmed it was unquoted (NSIS `"…"` are string delimiters, not embedded quotes) — infra-security's H-05 stands and was fixed.

## Recommendations

### Must Fix (triaged "fix") — DONE this round
All 12 HIGH findings (H-01…H-12) and 6 MED findings (M-01…M-06) patched in `5a8fb79`.

### Deferred (triaged "defer") — revisit via `/wf review ship-plan-buildout triage`
None remaining — all previously-deferred MED findings (M-07…M-19) were promoted to Fix and patched in round 2 (`c8011d9`). The only un-actioned items are the LOW/NIT polish in **Consider** below.

### Consider (LOW/NIT — not triaged)
L-03 includeSubdomains (subsumed by H-02) · L-04 gradle cache-read-only · L-05 job timeout-minutes · L-06 config_test through real Load/Save · L-07 silent-install service registration (documented) · L-08 middleware nesting · N-01 cliff.toml comment · N-02 runner-image drift. *(L-01 path sanitization and L-02 uninstaller StopService check were fixed in round 2.)*

### Still open from verify (not a code defect)
Every release acceptance criterion (AC4–AC10, AC12, AC13) remains runtime-unverified — the pipeline has never run against a real `v*` tag. Pre-tag checklist (from `06-verify-release-orchestration.md`): confirm the PyPI Trusted Publisher quartet, the `pypi` GitHub Environment, and the `v*` tag-protection ruleset; run `actionlint` (not available locally); then push `v0.1.0-rc.1` from `main` and observe all jobs green.

## Recommended Next Stage
- **Option A (default):** `/wf handoff ship-plan-buildout` — converged, no blockers; all 5 slices implemented and the slug-wide review is clean. Aggregates the branch into the PR description. ([PR #1](https://github.com/jayteealao/PushKit/pull/1) already exists.)
- **Option B:** `/wf review ship-plan-buildout triage` — only 8 LOW/NIT polish items remain deferred; re-triage is optional and low-value before handoff.
- **Option C:** `/wf review ship-plan-buildout` — fresh full review round (compact first) if you want re-coverage after the fixes.
- **Option E:** `/wf ship ship-plan-buildout` — skip handoff and go straight to release notes (only after the pre-tag checklist + the real `v0.1.0-rc.1` tag clears the deferred runtime ACs).

## Fix Status — Round 3 (post-handoff CI blocker, 2026-05-30)

A live blocker surfaced after handoff pushed the branch: all five pre-merge CI
jobs fast-failed because the action-pinning pass had assigned commit SHAs to the
wrong actions (a 4-cycle across checkout/setup-java/setup-go/setup-python, a
2-cycle across wagoid-commitlint/pypa-publish, and a non-existent gradle SHA).
This was not caught by the original review's "all uses: SHA-pinned" check, which
confirmed the *form* of the pins but not that each SHA resolved to the named
action.

Once the pins were corrected and pushed, the five fast-failing jobs went green,
which then let the `vuln-scan` security gate actually run — and it failed,
surfacing a second, independent blocker (SEC-01).

| ID | Severity | Status | Notes |
|----|----------|--------|-------|
| CI-01 | BLOCKER | Fixed | All 12 action pins re-resolved to the canonical SHA for their version tag across ci.yml, release.yml, go-test composite. Every pin verified against the GitHub commits API (HTTP 200). Confirmed green: backend-test, cli-test, android-build, commitlint-backstop all pass on the re-pinned run. |
| NODE-01 | MED | Fixed | `package.json` engines `>=20` → `>=22.12.0`; README prerequisite `Node ≥ 20` → `Node ≥ 22.12` (matches `@commitlint/cli@21` requirement, addresses the open reviewer threads). |
| SEC-01 | BLOCKER | Fixed | `govulncheck` gate flagged 7 called Go standard-library CVEs (net/crypto·x509/net·http/crypto·tls/os/net·url), all fixed in Go 1.25 patches. Bumped the CI/release Go toolchain `1.24` → `1.25` in all four `go-version` spots; `setup-go` resolves to ≥ go1.25.10, clearing every called vuln and shipping patched release binaries. README Go prerequisite bumped to `Go ≥ 1.25`. |

**Verification:** ci.yml, release.yml, go-test action.yml parse as valid YAML;
package.json parses as valid JSON; no unresolved action SHA remains; all five
pre-merge jobs green after the pin fix; toolchain bump pushed to clear the
security gate (CI re-run observed after push).
