---
schema: sdlc/v1
type: index
slug: ship-plan-buildout
title: "Build out CI infra + code to satisfy the ship-plan contract"
status: active
current-stage: handoff
stage-number: 8
workflow-type: standard
created-at: "2026-05-22T21:28:25Z"
updated-at: "2026-05-30T20:33:00Z"
selected-slice: "release-orchestration"
branch-strategy: dedicated
branch: "feat/ship-plan-buildout"
base-branch: "main"
review-scope: slug-wide
pr-url: "https://github.com/jayteealao/PushKit/pull/1"
pr-number: 1
open-questions: []
tags:
  - release-engineering
  - ci-cd
  - github-actions
  - nsis
  - git-cliff
  - android
  - go
stack:
  detected-at: "2026-05-22T21:28:25Z"
  platforms: [service, cli, android]
  languages: [go, kotlin]
  ui: [compose]
  build: [gradle, go, make, nsis]
  package-managers: [go-modules, gradle, pip]
  testing: [go-testing, gradle-junit, android-lint]
  observability: []
  integrations: []
  available-skills:
    - {name: tech-research-enforcer, hint: "Force verify-against-docs before locking action/tool versions"}
    - {name: android-cli, hint: "Android project + SDK orchestration (user-requested)"}
    - {name: lazylogcat, hint: "Non-interactive logcat capture/filter for Android app (user-requested)"}
    - {name: framework-conventions-guide, hint: "Idiomatic Go / Gradle patterns"}
    - {name: testing-setup, hint: "Test infra scaffolding if Android tests need it"}
  available-mcp:
    - {name: web-reader, hint: "Fetch official docs (GitHub Actions, NSIS, git-cliff)"}
    - {name: web-search-prime, hint: "Find current versions / recipes"}
  user-confirmed: true
next-command: wf-handoff
next-invocation: "/wf handoff ship-plan-buildout"
runtime-evidence-deferrals:
  - slice: commit-hygiene
    reason: "AC2 (commitlint-backstop fault-detection) and AC3 (backend-test fault-detection) require deliberate test PRs with bad commits; skipped by maintainer in verify triage. Configuration inspection confirms correct wiring."
    deferred-at: "2026-05-25T14:28:26Z"
    cleared-by: null
  - slice: backend-version
    reason: "AC4 (wheel metadata URL): make and pip unavailable in verify environment; Makefile:13 and release.yml:44 confirmed correct via static grep. Clear by running make build-wheels VERSION=0.1.0-test-1 on a machine with make+pip and inspecting unzip -p dist/pushkit-*.whl '*/METADATA' | grep Home-page."
    deferred-at: "2026-05-26T11:37:46Z"
    cleared-by: null
  - slice: release-orchestration
    reason: "All 9 ACs (AC4, AC5, AC6, AC7, AC8, AC9, AC10, AC12, AC13) require a live GitHub Actions run and/or post-publish observation. Local environment has no access to GitHub Actions runners, live PyPI, or Windows 2022 runners. Static checks (YAML, cliff.toml, README) all pass. Clear by pushing v0.1.0-rc.1 from main after PR merge and observing all 8 pipeline jobs green + post-publish-checks pass."
    deferred-at: "2026-05-27T18:42:49Z"
    cleared-by: null
workflow-files:
  - 00-index.md
  - 01-intake.md
  - 02-shape.md
  - 03-slice.md
  - 03-slice-commit-hygiene.md
  - 03-slice-nsis-installer.md
  - 03-slice-backend-version.md
  - 03-slice-android-versioning.md
  - 03-slice-release-orchestration.md
  - 04-plan.md
  - 04-plan-commit-hygiene.md
  - 04-plan-nsis-installer.md
  - 05-implement.md
  - 05-implement-commit-hygiene.md
  - 06-verify.md
  - 06-verify-commit-hygiene.md
  - 05-implement-nsis-installer.md
  - 06-verify-nsis-installer.md
  - 04-plan-backend-version.md
  - 04-plan-android-versioning.md
  - 04-plan-release-orchestration.md
  - 05-implement-backend-version.md
  - 06-verify-backend-version.md
  - 05-implement-android-versioning.md
  - 06-verify-android-versioning.md
  - 05-implement-release-orchestration.md
  - 06-verify-release-orchestration.md
  - 07-review.md
  - 07-review-correctness.md
  - 07-review-security.md
  - 07-review-code-simplification.md
  - 07-review-ci.md
  - 07-review-release.md
  - 07-review-supply-chain.md
  - 07-review-infra-security.md
  - 07-review-reliability.md
  - 07-review-testing.md
  - 07-review-maintainability.md
  - 07-review-docs.md
  - 08-handoff.md
  - po-answers.md
  - 00-sync.md
progress:
  intake: complete
  shape: complete
  slice: complete
  plan: complete
  implement: complete
  verify: complete
  review: complete
  handoff: complete
  ship: not-started
  retro: not-started
---
