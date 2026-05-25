---
schema: sdlc/v1
type: slice-index
slug: ship-plan-buildout
status: complete
stage-number: 3
created-at: "2026-05-22T22:46:55Z"
updated-at: "2026-05-22T22:46:55Z"
total-slices: 5
best-first-slice: commit-hygiene
tags:
  - release-engineering
  - ci-cd
  - github-actions
  - nsis
  - git-cliff
  - lefthook
  - android-versioning
slices:
  - slug: commit-hygiene
    status: defined
    complexity: s
    depends-on: []
  - slug: nsis-installer
    status: defined
    complexity: l
    depends-on: []
  - slug: backend-version
    status: defined
    complexity: xs
    depends-on: [commit-hygiene]
  - slug: android-versioning
    status: defined
    complexity: s
    depends-on: [commit-hygiene]
  - slug: release-orchestration
    status: defined
    complexity: xl
    depends-on: [commit-hygiene, nsis-installer, backend-version, android-versioning]
refs:
  index: 00-index.md
  shape: 02-shape.md
next-command: wf-plan
next-invocation: "/wf plan ship-plan-buildout commit-hygiene"
---

# Slice Index: ship-plan-buildout

## Slice Strategy

Five thin vertical slices, all delivered through a single PR on the `feat/ship-plan-buildout` branch (consistent with the slug-wide review scope chosen at intake). Each slice is independently planned, implemented, and verified before the next; they share the same branch but get their own `04-plan-*`, `05-implement-*`, `06-verify-*` artifacts so review continuity and reasoning trails stay clean. The slug-wide review (`07-review.md`) runs once against the cumulative branch diff before handoff.

**Why five and not three:**

- Each slice has a distinct technical surface (lefthook config, NSIS authoring, Go flag handler, Gradle properties, YAML orchestration). Combining them into 2–3 larger slices would mix surfaces and make the plan stage's parallel reuse scan less useful.
- Per-slice artifacts give us a clean rollback point: if `release-orchestration` reveals that the `nsis-installer` slice needs reshaping, only that slice's plan is invalidated, not a giant cross-cutting plan.
- The user explicitly chose "one PR, 5 slices" in the granularity question (Round 1 of slicing), confirming this hybrid model.

**Why risk-first ordering (per Round 2 of slicing):**

The hardest, highest-uncertainty work — NSIS authoring with optional Windows service component — is placed second (right after the mandatory `commit-hygiene` foundation). This gives the maintainer iteration time on a Windows machine before the integrator slice tries to invoke `makensis` in CI. Mechanical work (`backend-version`, `android-versioning`) sandwiches between, leaving `release-orchestration` as the final integrator.

**Why all slices land before the validation tag (per Round 3 of slicing):**

The v0.1.0-rc.1 tag is the workflow's acceptance gate. Earlier per-slice verification proves each piece works locally; the tag proves the whole pipeline works together. No throwaway intermediate tags.

## Recommended Order

1. **`commit-hygiene`** (complexity: s) — Foundation. Must land first so subsequent slices' commits parse as Conventional Commits and `git-cliff` has something to generate notes from. Also unblocks PR-level confidence via `ci.yml`.
2. **`nsis-installer`** (complexity: l) — Highest uncertainty. Maintainer iterates locally on a Windows machine with `makensis`. The optional Windows service component (Round 4 escalation in shape) means full upgrade/uninstall lifecycle automation — non-trivial. Landing this second gives the most iteration time.
3. **`backend-version`** (complexity: xs) — Small, mechanical. Adds `var Version` + `--version` flag to `backend/cmd/server/main.go` plus the cosmetic URL fixes in `Makefile`/`release.yml`. Required before `release-orchestration` can wire the smoke test.
4. **`android-versioning`** (complexity: s) — Single-file change in `android/app/build.gradle.kts` to accept `-PversionNameOverride` and `-PversionCodeOverride`. Validates locally with a debug build. Required before `release-orchestration` can inject from CI.
5. **`release-orchestration`** (complexity: xl) — The integrator. Expands `release.yml` from 1 to 8 jobs, adds `cliff.toml`, README sections, post-publish checks. Validation = push `v0.1.0-rc.1` from `main`, watch all 8 jobs go green.

## Cross-Cutting Concerns

- **Single branch, single PR.** All slices land on `feat/ship-plan-buildout`. The handoff stage opens one PR with a sectioned description covering all five slice's narratives.
- **Sequential merge into the branch, not into main.** Each slice's implement→verify cycle runs locally on the branch. The branch only merges to `main` once all slices verify cleanly and the slug-wide review (`07-review.md`) is clean. Validation tag pushes from `main` after merge.
- **Conventional Commits enforcement starts at slice 1's first commit.** The first commit on the branch should be the lefthook/commitlint setup itself (so subsequent commits are enforced from commit 2 onward). The CI backstop won't catch commit 1's own non-conformance; the maintainer authors it conventionally.
- **No CHANGELOG.md committed.** Per ship-plan Block B, the changelog is generated at release time and embedded in the GitHub Release notes only. Not committed to `main`.
- **Cosmetic URL fix bundled into `backend-version`.** Same files touched, single PR — no need to split the one-line edits into their own slice.
- **`PYPI_API_TOKEN` sealed secret creation is manual.** The maintainer creates the GH secret out-of-band (one-time setup). The plan stage for `release-orchestration` documents the command but does not script the secret creation. Same for the `tag-protection` GitHub branch-protection rule.

## Dependencies Between Slices

Hard edges (per Round 4 of slicing):

- `commit-hygiene` → all other slices (foundational; commits must be conventional from slice 2 onward).
- `nsis-installer` → `release-orchestration` (release.yml invokes `makensis backend/installer/pushkit.nsi`).
- `backend-version` → `release-orchestration` (post-publish smoke test asserts `pushkit-server --version` matches the tag).
- `android-versioning` → `release-orchestration` (release.yml passes `-PversionCodeOverride` / `-PversionNameOverride` to Gradle).

Soft edges:

- `nsis-installer`, `backend-version`, `android-versioning` are independent of each other and could be planned in parallel after `commit-hygiene` lands. The serial ordering (Round 2 risk-first) is a delivery-order preference, not a hard technical constraint.

## Deferred / Optional Slices

None. Every slice is required to satisfy the v0.1.0-rc.1 acceptance gate.

Items explicitly out of scope (carried forward from intake/shape — would be future workflows if ever scoped):

- Android release signing.
- Windows installer code-signing.
- Backend DB migration tooling.
- Go module path renames.
- macOS/Linux backend installers.
- Container image publishing to GHCR.
- SLSA provenance / sigstore attestations.

## Freshness Research

No new external research at slice stage. Shape's freshness research (PyPI Trusted Publishing, NSIS on `windows-2022`, git-cliff bump rules, lefthook v2.1.8, PEP 440 normalization, shields.io badge format) covers the external dependencies. Plan-stage may need targeted re-research for specific job recipes (e.g., the exact YAML for `orhun/git-cliff-action@v3`, `gradle/actions/setup-gradle@v4` cache key patterns) but those are tactical, not strategic.

## Recommended Next Stage

- **Option A (default):** `/wf plan ship-plan-buildout commit-hygiene` — plan the foundational slice first. `commit-hygiene` is the hard prerequisite for everything else and the smallest planning surface, so it's the natural starting point.
- **Option B:** `/wf plan ship-plan-buildout all` — plan all 5 slices in parallel. Viable because the three middle slices (`nsis-installer`, `backend-version`, `android-versioning`) are technically independent. Trade-off: 5 plan files at once is a lot of context, and shape's open questions about local NSIS validation may surface mid-plan and ripple. Recommended only if the maintainer has appetite for a single deep planning session.
- **Option C:** `/wf shape ship-plan-buildout` — revisit shape. Not applicable; the shaped spec gave clear acceptance criteria and the slicing strategy questions surfaced no ambiguities requiring shape rework.
