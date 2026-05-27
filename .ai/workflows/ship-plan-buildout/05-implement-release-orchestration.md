---
schema: sdlc/v1
type: implement
slug: ship-plan-buildout
slice-slug: release-orchestration
status: complete
stage-number: 5
created-at: "2026-05-27T18:24:55Z"
updated-at: "2026-05-27T18:24:55Z"
metric-files-changed: 3
metric-lines-added: 577
metric-lines-removed: 15
metric-deviations-from-plan: 1
metric-review-fixes-applied: 0
commit-sha: ""
tags:
  - github-actions
  - release
  - git-cliff
  - pypi
  - github-release
  - nsis
refs:
  index: 00-index.md
  implement-index: 05-implement.md
  slice-def: 03-slice-release-orchestration.md
  plan: 04-plan-release-orchestration.md
  siblings:
    - 05-implement-commit-hygiene.md
    - 05-implement-nsis-installer.md
    - 05-implement-backend-version.md
    - 05-implement-android-versioning.md
  verify: 06-verify-release-orchestration.md
next-command: wf-verify
next-invocation: "/wf verify ship-plan-buildout release-orchestration"
---

# Implement: release-orchestration

## Summary of Changes

Wired the tag-driven release pipeline that turns a `v*` tag push into a complete GitHub Release plus PyPI publish. Expanded `.github/workflows/release.yml` from a 52-line single-job stub into a 13-job graph: `tag-guard` → 3× `retest-*` → 4 parallel build jobs (CLI wheel, backend binary, Android APK, changelog) → `build-backend-installer` (Windows, depends on backend binary) → `publish-pypi` → `create-github-release` → 2 parallel post-publish smoke tests. Added `cliff.toml` for `git-cliff` Keep-a-Changelog rendering with our tag scheme. Added a shields.io release badge, a `## Releasing` reference section, and a `## Backend installer` how-to section to `README.md`.

The pipeline is fail-loud and all-or-nothing: any of the four build jobs failing skips PyPI; PyPI failing skips the GitHub Release. Concurrency lock on `release-${{ github.ref_name }}` with `cancel-in-progress: false` prevents two simultaneous runs on the same tag from colliding without dropping either.

## Files Changed

- `cliff.toml` (new, 57 lines) — Keep-a-Changelog body template, conventional-commits grouping (11 parsers covering feat/fix/perf/refactor/style/test/docs/build/ci/chore/revert), `tag_pattern = "^v[0-9]+\\.[0-9]+\\.[0-9]+(?:-(?:rc|alpha|beta)\\.[0-9]+)?$"` matching our scheme, `filter_unconventional = true` so the 6 pre-existing non-conventional commits are dropped from the first changelog.
- `.github/workflows/release.yml` (rewrite, 408 / 15) — full 13-job graph, see Job graph notes below.
- `README.md` (+112, 0 removed) — shields.io badge under the title; jump-link line pointing to the two new sections; `## Releasing` (tagging conventions, single push command, pipeline overview, watching the run, rollback procedure, required GitHub repo settings, OIDC break-glass path); `## Backend installer` (download links, interactive + silent install commands, uninstall, SmartScreen note).

## Shared Files (also touched by sibling slices)

- `README.md` — `commit-hygiene` previously added `## Development setup`. This slice adds the shields.io badge at the top and appends `## Releasing` + `## Backend installer` after `## Testing`. No content collisions; both new sections are net-new.
- `.github/workflows/release.yml` — `backend-version` previously added `--url "https://github.com/jayteealao/PushKit"` in the wheel build step. That step is preserved (now inside `build-cli-wheel` via `make build-wheels VERSION=…`, which inherits the URL from the Makefile).

## Notes on Design Choices

**Job graph (final shape, 13 jobs):**

```
tag-guard ──> retest-backend ─┐
              retest-cli ─────┼──> build-cli-wheel ──────┐
              retest-android ─┘    build-backend-binary ─┼──> publish-pypi ──> create-github-release ──┬──> post-publish-linux
                                   build-android-apk ───┤                                             └──> post-publish-windows
                                   generate-changelog ──┤
                                                         │
                              build-backend-binary ─> build-backend-installer ─┘
```

`build-backend-installer` depends solely on `build-backend-binary` (not on the other retests) since it needs the cross-compiled exe before NSIS-wrap. The four `publish-pypi` `needs:` (`build-cli-wheel`, `build-backend-installer`, `build-android-apk`, `generate-changelog`) enforce all-or-nothing: any artifact build failure aborts the publish.

**`make build-wheels` is the source of truth for the wheel build.** The job calls `make build-wheels VERSION=${VERSION}` instead of re-inlining `pip install go-to-wheel && go-to-wheel ./cli …`. Keeps CI in lockstep with local-dev parity (the maintainer can run the same command locally to debug).

**Action pins (per plan freshness research, re-verified at implement time):**

| Action | Pin | Notes |
|---|---|---|
| `actions/checkout` | `@v4` | Matches existing ci.yml + the prior release.yml. |
| `actions/setup-go` | `@v5` | Matches existing. |
| `actions/setup-java` | `@v4` | Temurin 17 (matches ci.yml). |
| `actions/setup-python` | `@v5` | Matches existing release.yml. |
| `gradle/actions/setup-gradle` | `@v4` | Matches existing ci.yml; `cache-read-only: ${{ github.ref != 'refs/heads/main' }}` evaluates `true` on tag refs (read-only on release runs, correct). |
| `actions/upload-artifact` / `actions/download-artifact` | `@v4` | v3 hard-deprecated 2025-01-30. |
| `pypa/gh-action-pypi-publish` | `@release/v1` | `attestations: false` per shape (v0.x posture; SLSA hardening deferred). |
| `orhun/git-cliff-action` | `@v4` | Latest is `v4.8.0` (2026-04-26); floating major pin matches repo style. |
| `softprops/action-gh-release` | `@v3` | Latest is `v3.0.0` (2026-04-12); built-in retry + glob support. |
| `repolevedavaj/install-nsis` | `@v1` | See deviation below. Installs NSIS to `C:\Program Files (x86)\NSIS\`, adds to PATH, applies the long-string-8192 patch. Latest is `v1.2.0` (2026-05-07). |

**Runner pins:** `windows-2022` explicit for `build-backend-installer` and `post-publish-windows` to avoid `windows-latest` drift into `windows-2025` mid-release (per shape NFR around runner stability). All other jobs use `ubuntu-latest`.

**`fetch-depth: 0` on three jobs** — `tag-guard` (needs history for `git merge-base --is-ancestor`), `build-android-apk` (needs history for `git rev-list --first-parent --count HEAD`), `generate-changelog` (`git-cliff` walks full history). Default `fetch-depth: 1` would silently degrade `versionCode` to `1` and produce an empty changelog.

**`-PversionCodeOverride` / `-PversionNameOverride` (project properties, not system properties).** Gradle `providers.gradleProperty(...)` only reads project properties; `-D` system properties are silently invisible. The Android-versioning plan flagged this as the #1 silent-failure mode and we honour it here.

**`-X main.Version=<stripped>`** in `build-backend-binary` — the backend slice's `printVersion` writes `pushkit-server <v>\n`, so injecting the *stripped* form (`0.1.0-rc.1`) makes the smoke-test assertion `pushkit-server 0.1.0-rc.1` correct.

**SHA256SUMS does NOT include the wheel.** PyPI is the canonical location for the wheel; the GitHub Release SHA256SUMS only covers the two binary assets (`pushkit-server-setup.exe`, `pushkit-android.apk`).

**`make_latest` driven from the prerelease detector.** For `-rc`/`-alpha`/`-beta` tags the release is marked prerelease AND not-latest, so `pip install pushkit` (no `--pre`) and the shields.io non-prerelease badge keep pointing at the prior stable release.

**`fail_on_unmatched_files: true`** on `softprops/action-gh-release@v3` — surfaces a missing asset as a job failure rather than silently shipping a partial release.

**`post-publish-linux` AAPT lookup is dynamic.** The plan suggested `apt-get install aapt`, but `aapt` is not in apt repos on ubuntu-24.04+. The implementation instead globs `$ANDROID_HOME/build-tools/*/aapt` and uses the highest version found. `ANDROID_HOME` is set to `/usr/local/lib/android/sdk` by default on `ubuntu-latest` runners.

## Visual Contract Honored (only if `02c-craft.md` was present)

n/a — no `02c-craft.md` in this workflow. The shields.io badge and README sections are reference-grade prose, no visual contract.

## Deviations from Plan

1. **NSIS install action: `repolevedavaj/install-nsis@v1` instead of `negrutiu/nsis-install@v2`.** The plan's freshness research named `negrutiu/nsis-install@v2`, but that repository does not exist. Verified via `gh api search/repositories` — `negrutiu/*` ships NSIS plugins (nscurl, taskbarprogress, shelllink, execdos) but no install action. `repolevedavaj/install-nsis@v1` is named in the plan's Assumptions section as the fallback; it is actively maintained (latest `v1.2.0` 2026-05-07), uses the same `nsis-version:` input name as planned, supports `3.12`, and additionally applies the official long-string (8192) patch. Input contract unchanged; deviation is invisible to the calling workflow.

## Anything Deferred

- **Pre-flight `actionlint` run** — not run because `actionlint` is not in the local environment. Cheap to install (`brew install actionlint` or `choco install actionlint`), and the workflow YAML was validated via `python -m yaml` (13 jobs parsed, no syntax errors). Recommend running actionlint pre-tag during verify.
- **Throwaway-tag validations (AC4, AC13)** — interactive verification per the slice's AC verification plan; belongs to the verify stage.
- **Validation tag push (`v0.1.0-rc.1`)** — happens after PR merge; belongs to verify (with explicit maintainer go/no-go gate).
- **Required GitHub repo settings** (PyPI Trusted Publisher quartet, tag-protection ruleset, `PYPI_API_TOKEN` sealed secret) — documented in the new `## Releasing` section; the maintainer must configure these one-time on github.com / pypi.org before the validation tag.

## Known Risks / Caveats

- **`packaging.version.Version` PEP 440 quirk.** `Version('0.1.0-rc.1')` normalises to `0.1.0rc1` (no dashes, no dots inside the prerelease segment). `post-publish-linux`'s `pip install --pre pushkit==0.1.0rc1` works because pip is forgiving — but if anyone manually pins `0.1.0-rc.1` they'll get a confusing miss. The `## Releasing` README section documents the normalized form.
- **First-tag git-cliff with `filter_unconventional = true`.** Only 1 of the last 6 commits prior to this branch was conventional. The first auto-generated changelog will surface only this branch's conventional commits (which are all conventional by lefthook+commitlint enforcement). Acceptable per shape Round 5 ("forward-only enforcement; thin v0.1.0-rc.1 changelog accepted").
- **`pypi` environment must exist on the repo.** OIDC Trusted Publishing matches on `(owner, repo, workflow_filename, environment)`. The `publish-pypi` job declares `environment: pypi`; if the GitHub repo doesn't have an `Environment` named exactly `pypi`, the job will fail before publishing. The maintainer is expected to create this environment as a one-time setup (documented in `## Releasing` → "Required GitHub repo settings").
- **`softprops/action-gh-release@v3` major-version pin.** Floating tag accepts upstream changes within v3. Acceptable supply-chain residual risk per plan freshness research (matched by the `negrutiu→repolevedavaj` deviation; both are third-party actions on floating major pins).
- **Concurrency with `cancel-in-progress: false`.** Two simultaneous `v*` tag pushes on the same tag (rare race) queue rather than cancel. The second run will likely fail at `create-github-release` because the tag already exists (`gh release create` / softprops can't create a duplicate). Acceptable.
- **Windows runner cold-start ~2–4 min** on `windows-2022` adds to the ≤ 15-min NFR. Both `build-backend-installer` and `post-publish-windows` pay this. Monitor in the first real release run.
- **The first `make build-wheels VERSION=...` in CI requires `pip install go-to-wheel` (pulled inline by the Makefile) and `go-to-wheel` accessing the network to build wheels.** No PyPI auth needed at this point (just to read).

## Freshness Research

Re-verified at implement time (2026-05-27):

- `softprops/action-gh-release` → latest `v3.0.0` (2026-04-12), input contract for `tag_name`, `body_path`, `prerelease`, `make_latest`, `files`, `fail_on_unmatched_files` unchanged from plan freshness research.
- `orhun/git-cliff-action` → latest `v4.8.0` (2026-04-26); `@v4` floating major pin is correct.
- `repolevedavaj/install-nsis` → latest `v1.2.0` (2026-05-07); input is `nsis-version:` (verified by reading action.yml); supports arbitrary version values resolved against the SourceForge `NSIS/3/<ver>/` directory. Confirmed `3.12` is downloadable.
- `negrutiu/nsis-install@v2` → does not exist (resolved via `gh api`); falling back to `repolevedavaj/install-nsis@v1` per plan Assumptions.

## Recommended Next Stage

- **Option A (default):** `/wf verify ship-plan-buildout release-orchestration` — verify the slice. Most of the AC verification is interactive (real tag push to validate the pipeline end-to-end) and human-in-the-loop (release notes inspection, shields.io badge cache).
- **Option B:** `/wf review ship-plan-buildout` — slug-wide review (per `review-scope: slug-wide` in `00-index.md`) before cutting `v0.1.0-rc.1`. With all five slices now implemented, a cumulative branch diff review is the cheapest place to catch cross-slice integration issues. Recommended if the maintainer wants reviewer eyes before paying the validation-tag cost.
