---
schema: sdlc/v1
type: ship-plan
slug: pushkit
plan-version: 1
created-at: "2026-05-22T21:14:08Z"
updated-at: "2026-05-22T21:14:08Z"
project-name: "PushKit"
template-hint: none

# === Required core — read by /wf ship ===

# Block A — what ship means
ship-meaning: publish
ship-environments:
  - { name: "public", auto-promote: true }
ship-cadence: on-demand

# Block B — versioning contract
version-scheme: semver
version-source-of-truth:
  - { path: "<git-tag>", field: "vX.Y.Z" }
version-bump-rule: git-cliff
version-bump-cmd: "git cliff --bumped-version"
prerelease-suffix: "-rc.N"
post-release-version: none
post-release-version-cmd: ""

# Block C — CI/CD contract
ci-pipeline:
  pre-merge-checks:
    - "backend: cd backend && go vet ./... && go test ./..."
    - "cli: cd cli && go vet ./... && go test ./..."
    - "android: cd android && ./gradlew build"
    - "android: cd android && ./gradlew lint testDebugUnitTest"
  release-trigger: tag-on-main
  release-workflow-file: ".github/workflows/release.yml"
  release-jobs:
    - retest
    - build-cli-wheel
    - publish-pypi
    - build-backend-windows-installer
    - build-android-apk
    - generate-changelog
    - create-github-release
  publish-dry-run-cmd: "git cliff --bumped-version && twine check dist/* && gh release create vX.Y.Z-rc.0 --draft --notes-file CHANGELOG.md"
  publish-cmd: "git tag vX.Y.Z && git push origin vX.Y.Z"
  required-secrets:
    - { name: "GITHUB_TOKEN", purpose: "Auto-provided by Actions; used by create-github-release to upload assets. No rotation needed." }
  secrets-staleness-threshold-days: 90

# Block D — post-publish verification contract
post-publish-checks:
  - { kind: "fresh-resolve", cmd: "pip install --no-cache-dir pushkit==$VERSION && pushkit --version", expect: "$VERSION" }
  - { kind: "github-release", cmd: "gh release view v$VERSION --json assets --jq '.assets[].name'", expect: "pushkit-server-setup.exe, pushkit-android.apk, SHA256SUMS" }
  - { kind: "smoke-test", cmd: "pushkit-server-setup.exe /S && \"%ProgramFiles%\\PushKit\\pushkit-server.exe\" --version", expect: "$VERSION" }
  - { kind: "apk-probe", cmd: "aapt dump badging pushkit-android.apk | grep versionName", expect: "versionName='$VERSION'" }
propagation-window-min-minutes: 5
propagation-window-max-minutes: 30
poll-interval-seconds: 60

# Block E — rollout + rollback contract
rollout-strategy: immediate
rollout-stages: []
rollback-mechanism: yank-and-patch
rollback-time-estimate-min: 15
db-migrations-reversible: n/a

# Block F — recovery playbooks
recovery-playbooks:
  - id: pypi-trusted-publish-failure
    triggers:
      - "(?i)invalid-publisher"
      - "(?i)not a trusted publisher"
      - "(?i)trusted publisher.*not found"
      - "(?i)oidc.*token.*reject"
    steps:
      - "Open the PyPI project page → Settings → Publishing → confirm a Trusted Publisher entry exists for owner=jayteealao, repo=PushKit, workflow=release.yml, environment=pypi."
      - "Confirm the release workflow declares `environment: pypi` and `permissions: id-token: write` on the publishing job."
      - "Confirm the tag was pushed by an actor allowed on the `pypi` environment (Settings → Environments → pypi → required reviewers / branch policy)."
      - "If first-time setup, the PyPI Trusted Publisher entry must be created via the PyPI web UI; the GitHub side cannot bootstrap it."
      - "Retry the release by re-running the failed job, NOT by re-tagging (tag is already consumed)."

  - id: nsis-build-failure
    triggers:
      - "(?i)makensis.*error"
      - "(?i)cannot find file.*\\.exe"
      - "(?i)\\.nsi.*syntax error"
      - "(?i)!include.*not found"
    steps:
      - "Verify the backend Go build step produced `pushkit-server.exe` and the path referenced by the NSIS `File` directive matches."
      - "Run `makensis -V4 installer/pushkit.nsi` locally on a Windows machine to capture the full error context."
      - "If a plugin/include is missing, ensure the runner's NSIS install includes it (Chocolatey `nsis` ships with stdlib; extra plugins may need explicit install)."
      - "Common cause: wrong working directory — NSIS resolves `File` paths relative to the `.nsi` file, not the workflow cwd."

  - id: gh-release-asset-upload-failure
    triggers:
      - "(?i)release upload.*failed"
      - "(?i)HTTP 422"
      - "(?i)asset.*already exists"
      - "(?i)release.*not found"
    steps:
      - "Confirm the release was actually created before upload — `gh release view v$VERSION` should return JSON, not 404."
      - "If `asset already exists`, retry with `gh release upload v$VERSION <file> --clobber` (idempotent overwrite)."
      - "If 422 on size: check the file is under GitHub's 2 GB per-asset cap; split or compress."
      - "If 401/403: verify `permissions: contents: write` on the job and that GITHUB_TOKEN is not scoped down by org policy."

  - id: tag-on-wrong-branch
    triggers:
      - "(?i)tag.*not in main"
      - "(?i)release from non-main branch"
    steps:
      - "Identify the offending tag: `git tag --contains <tag-sha>` and `git branch --contains <tag-sha>`."
      - "Delete locally and remotely: `git tag -d vX.Y.Z && git push --delete origin vX.Y.Z`."
      - "Cancel any in-flight release workflow run for the bad tag via `gh run cancel`."
      - "Re-tag from a commit on `main`: `git checkout main && git tag vX.Y.Z && git push origin vX.Y.Z`."
      - "If PyPI publish already happened for the bad tag, yank the affected version with `pip yank pushkit $VERSION`."

# Block G — stakeholder + announcement contract
announcement:
  channels:
    - "github-release-notes"
    - "readme-latest-release-badge"
  template-path: ".ai/release-announcement-template.md"

# === Extensions — open schema, not read by /wf ship unless a consumer opts in by id ===

additional-contracts: []
---

# Ship Plan — PushKit

## What "ship" means here

A PushKit release is a **public multi-artifact publish** that goes out from a single git tag (`vX.Y.Z`) on `main`. Three artifacts are produced in lock-step:

1. **CLI** — Go binary repackaged as a Python wheel (`pushkit`) and published to **PyPI**.
2. **Backend** — Windows executable (`pushkit-server.exe`) wrapped in an **NSIS installer** (`pushkit-server-setup.exe`), attached to a **GitHub Release**.
3. **Android app** — debug-signed APK (`pushkit-android.apk`), attached to the same **GitHub Release**.

Cadence is `on-demand` — there's no fixed train. Pre-release candidates use `-rc.N` (e.g. `v0.2.0-rc.1`); the GitHub Release is marked as prerelease and PyPI uploads as prerelease metadata.

Discovery evidence:
- `.github/workflows/release.yml` triggers on `push: tags: ["v*"]` and currently publishes only the CLI to PyPI.
- `Makefile` defines the `go-to-wheel` invocation used by CI.
- `backend/Dockerfile` produces a Linux server binary today; the Windows installer pipeline is **not yet wired** — this plan documents the intent.
- `android/app/build.gradle.kts` has `versionName = "1.0"`, `versionCode = 1` hardcoded; CI will rewrite these from the tag on release.

## Versioning

The git tag is the only source of truth. Every artifact derives its version from `${GITHUB_REF_NAME#v}`:

- **CLI:** `go-to-wheel ./cli --version $VERSION --set-version-var main.Version` (already wired). `pushkit --version` reports the tag.
- **Backend:** `go build -ldflags "-X main.Version=$VERSION" -o pushkit-server.exe ./cmd/server`. `pushkit-server --version` reports the tag.
- **Android:** CI rewrites `versionName` to `$VERSION` and `versionCode` to a monotonic integer (e.g., `git rev-list --count HEAD`) in `android/app/build.gradle.kts` before the Gradle build. No file is committed back.

Conventional commits are required on `main` so `git cliff --bumped-version` can decide the next version automatically. `git-cliff` also generates `CHANGELOG.md` at release time; it is **not** committed back to the repo, it's published as the GitHub Release body.

Prereleases use `-rc.N` (e.g., `v0.2.0-rc.1`, `v0.2.0-rc.2`, then `v0.2.0`). No post-release SNAPSHOT / `-dev` suffix — `main` is always between releases.

## CI/CD pipeline

**Pre-merge gate** (required status checks on every PR to `main`):
- Backend: `cd backend && go vet ./... && go test ./...`
- CLI: `cd cli && go vet ./... && go test ./...`
- Android: `cd android && ./gradlew build`
- Android: `cd android && ./gradlew lint testDebugUnitTest`

**Release trigger:** push of a `v*` tag whose commit is in `main`'s ancestry. The workflow MUST guard against tags pushed from non-main branches (see playbook `tag-on-wrong-branch`).

**Release workflow:** `.github/workflows/release.yml` — currently single-job, must be expanded to seven jobs (listed in the schema). Job dependencies:

```
retest
  ├── build-cli-wheel ── publish-pypi
  ├── build-backend-windows-installer ──┐
  ├── build-android-apk ────────────────┤
  └── generate-changelog ───────────────┴── create-github-release
```

**Authentication:**
- **PyPI:** OIDC Trusted Publishing via `environment: pypi` + `permissions: id-token: write`. No PyPI token in CI. Trusted-publisher entry must exist on the PyPI side (one-time setup).
- **GitHub Releases:** `GITHUB_TOKEN` with `permissions: contents: write` on the release job.
- **Android signing:** debug keystore only for v0.x. When promoting to release signing, add `ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD` and amend this plan via `/wf-meta amend ship-plan`.

**Dry-run before tagging:**
```
git cliff --bumped-version           # see proposed next version
git cliff -o CHANGELOG.md            # preview release notes
twine check dist/*                   # if a wheel was built locally
gh release create vX.Y.Z-rc.0 --draft --notes-file CHANGELOG.md
```

**Actual publish:** push a tag.
```
git tag vX.Y.Z
git push origin vX.Y.Z
```

**Secret rotation review:** the only secret here is `GITHUB_TOKEN` (auto, no rotation). PyPI uses OIDC. Re-check every 90 days that Trusted Publisher config on PyPI still matches this repo and workflow.

## Post-publish verification

Every release runs four checks. The propagation window is 5–30 minutes (PyPI/Fastly caching) with 60s polling between attempts.

| Check | Command | Pass signal |
|---|---|---|
| `fresh-resolve` | `pip install --no-cache-dir pushkit==$VERSION && pushkit --version` | stdout contains `$VERSION` |
| `github-release` | `gh release view v$VERSION --json assets --jq '.assets[].name'` | list contains `pushkit-server-setup.exe`, `pushkit-android.apk`, `SHA256SUMS` |
| `smoke-test` | `pushkit-server-setup.exe /S && "%ProgramFiles%\PushKit\pushkit-server.exe" --version` (Windows runner) | exit 0, output contains `$VERSION` |
| `apk-probe` | `aapt dump badging pushkit-android.apk` | line matches `versionName='$VERSION'` |

A failed check does **not** auto-rollback — it surfaces the failure and human decides whether to yank.

## Rollout strategy

`immediate` — there's no canary, no staged %, no per-environment promotion. Every release is fully public the moment CI completes. Prereleases (`-rc.N`) provide the only "soft launch" mechanism: they sit on PyPI as prereleases (only installed with `pip install pushkit --pre`) and on GitHub Releases marked as prerelease (not shown as "Latest").

Vary from this default only when adding new artifact targets (e.g., Play Store phased rollout once Android moves to release signing — that decision goes into a new `mobile-app-store` additional-contract).

## Rollback playbook

Detection signals:
- Post-publish check fails (most likely first signal).
- User-reported install crash within the first hour after a release.
- Security disclosure.

Steps (`yank + new patch`):
1. **PyPI:** `pip yank pushkit $VERSION --reason "<short reason>"` (PyPI never deletes, only yanks; yanked versions don't install without explicit pin).
2. **GitHub Release:** `gh release edit v$VERSION --prerelease` (demotes from Latest) or `gh release delete v$VERSION --cleanup-tag` for severe issues.
3. **Code:** if the root cause is on `main`, revert the offending PR(s) with `git revert -m 1 <merge-sha>` and merge the revert.
4. **Re-release:** cut `vX.Y.(Z+1)` with the fix. Same tag-and-push flow.

Time estimate: **15 minutes** from detection to yank-published (PyPI yank is near-instant; GH release edit is instant; patch release takes a normal CI cycle).

DB migrations: not applicable — backend uses raw SQLite with no migration tooling. When that changes, amend Block E.

## Recovery playbooks

Four are seeded — see frontmatter for triggers + steps:
- **pypi-trusted-publish-failure** — most likely on first release; OIDC misconfig.
- **nsis-build-failure** — `makensis` errors on the Windows runner.
- **gh-release-asset-upload-failure** — 422 / `asset already exists` / 401.
- **tag-on-wrong-branch** — recovery from a tag pushed off a feature branch.

Add new playbooks via `/wf-meta amend ship-plan` as failure modes are encountered.

## Stakeholder + announcement

- **Primary channel:** the GitHub Release page itself. Its body is the `git-cliff`-generated `CHANGELOG.md`, which captures every conventional-commit since the previous tag.
- **Secondary signal:** a "latest release" badge in `README.md` (e.g., shields.io GitHub release badge). The badge auto-updates from the GitHub Releases API — no manual step.
- **Template:** `.ai/release-announcement-template.md` (create on first use). For now, `git-cliff`'s default Keep-a-Changelog template is sufficient.

## Additional contracts

None at v0.x.

Likely future contracts to revisit:
- **mobile-app-store** — when Android moves off debug signing toward Play Store (phased rollout %, review-window estimates, key-rotation policy).
- **data-migration** — once the backend adopts a migration tool (sqlc, goose, or sqlite-specific).

## Known follow-ups (not part of this contract)

These were surfaced during discovery; they're project work, not plan content:

- `.github/workflows/release.yml` and `Makefile` use `--url https://github.com/pushkit/cli`. Correct to `https://github.com/jayteealao/PushKit`.
- Go module paths `github.com/pushkit/cli` and `github.com/pushkit/backend` don't match the actual owner (`jayteealao`). Cosmetic; consider renaming when the project takes its long-term name.
- CI work required to satisfy this plan: add `retest` job, expand release.yml to multi-job, write the NSIS script (`installer/pushkit.nsi`), add the Android `versionName`/`versionCode` rewrite step, wire `git-cliff`, and add `create-github-release` with asset attachments.
