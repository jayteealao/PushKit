---
schema: sdlc/v1
type: index
slug: ship-plan-buildout
title: "Build out CI infra + code to satisfy the ship-plan contract"
status: active
current-stage: verify
stage-number: 6
workflow-type: standard
created-at: "2026-05-22T21:28:25Z"
updated-at: "2026-05-26T11:13:29Z"
selected-slice: "nsis-installer"
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
next-command: wf-review
next-invocation: "/wf review ship-plan-buildout"
runtime-evidence-deferrals:
  - slice: commit-hygiene
    reason: "AC2 (commitlint-backstop fault-detection) and AC3 (backend-test fault-detection) require deliberate test PRs with bad commits; skipped by maintainer in verify triage. Configuration inspection confirms correct wiring."
    deferred-at: "2026-05-25T14:28:26Z"
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
  - 05-implement-backend-version.md
  - po-answers.md
  - 00-sync.md
progress:
  intake: complete
  shape: complete
  slice: complete
  plan: complete
  implement: complete
  verify: in-progress
  review: not-started
  handoff: not-started
  ship: not-started
  retro: not-started
---
