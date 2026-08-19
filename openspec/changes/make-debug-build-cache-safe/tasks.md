## 1. Model the Effective Cache Trust Boundary

- [x] 1.1 Refactor `scripts/check-workflow-policy` so protected-default-branch source selection, repository/secret/environment privilege, runner persistence, and default-branch cache-write authority are represented independently.
- [x] 1.2 Encode GitHub's documented default-branch-cache-write event set with a source comment and make local reusable-workflow jobs inherit every effective root caller event.
- [x] 1.3 Add table-driven parser and call-graph tests for direct and reusable execution under `issue_comment`, `workflow_dispatch`, `repository_dispatch`, default-branch `push`, and equivalent YAML spellings.
- [x] 1.4 Replace the current debug-build policy exemption and dispatch-specific topology check with fail-closed rules for exact comment authorization, a local `workflow_call`-only builder, immutable identity revalidation, GitHub-hosted read-only execution, clean provenance finalization, and protected publication.
- [x] 1.5 Add positive fixtures for the supported `issue_comment` → `workflow_call` topology and negative fixtures for a writable reusable caller, direct dispatched builder, `GITHUB_TOKEN` label signaling, secret inheritance, self-hosted execution, stale or missing revalidation, and same-job provenance generation.
- [x] 1.6 Run the focused workflow-policy unit tests, `scripts/check-workflow-security.sh`, and immutable-action validation; fix the implementation rather than weakening any unrelated security rule.

## 2. Replace Dispatch with a Reusable Debug Builder

- [x] 2.1 Convert `.github/workflows/debug-build.yml` to typed required `workflow_call` inputs only, preserving full PR/SHA validation, exact-SHA checkout, `persist-credentials: false`, disabled automatic Go caching, pinned actions, explicit `contents: read`, and absence of secrets, environments, and self-hosted runners.
- [x] 2.2 Split `.github/workflows/debug-build-request.yml` into a trusted authorization job and a dependent local reusable-workflow call; require the exact command, maintainer permission, this repository as the PR base, and the current full head SHA, and pass only validated job outputs to the caller.
- [x] 2.3 Preserve per-PR request serialization without cancelling an already-running immutable build, and make a head change between authorization and builder checkout fail closed.
- [x] 2.4 Upload build output under an intermediate run-specific artifact name, then add a fresh trusted finalization job that revalidates identity, checks a bounded expected binary layout without execution, creates provenance from trusted outputs and run metadata, and uploads the immutable final `debug-build-${{ github.run_id }}` artifact.
- [x] 2.5 Ensure untrusted build code cannot create an eligible final artifact or provenance record: a name collision, unexpected file, symlink, special file, unsafe path, missing binary, or bound violation must fail the request run.

## 3. Rebind Protected Debug Publication

- [x] 3.1 Update `.github/workflows/publish-debug-release.yml` to accept only successful `.github/workflows/debug-build-request.yml` runs triggered by `issue_comment` in this repository, rather than `debug-build.yml` runs triggered by `workflow_dispatch`.
- [x] 3.2 Validate the current PR head, run ID and attempt, exact final artifact ID and name, repository, root event, PR number, full commit SHA, and clean-runner provenance before the separate publish job receives `contents: write`.
- [x] 3.3 Keep both publisher jobs restricted to `main`, retain the protected `debug-release` environment, download only the validated artifact ID, recheck provenance on the publish runner, and pass only expected binaries to the pinned release action without checkout or execution.
- [x] 3.4 Add policy fixtures and tests for valid reusable-run publication plus wrong workflow path, wrong event, stale SHA, failed run, expired or ambiguous artifact, mismatched run attempt, forged or malformed provenance, alternate-ref dispatch, and downloaded-content execution.

## 4. Correct Documentation and Archived Outcomes

- [x] 4.1 Update `docs/ci-security.adoc` with the June 26, 2026 GitHub read-only-cache change, authoritative links, the current write-capable trigger set, reusable caller-event inheritance, the new trust-domain diagram, operator steps, cache caveats, and rollback criteria.
- [x] 4.2 Add `outcome.md` to `openspec/changes/archive/2026-07-16-fix-debug-build-cache-poisoning` documenting commit `663dfe10465e`, the statically fixed CodeQL result, the suppressed `GITHUB_TOKEN` label event, the absence of successful end-to-end acceptance, and the superseding changes.
- [x] 4.3 Add `outcome.md` to `openspec/changes/archive/2026-07-17-fix-security-review-findings` documenting commit `d22ee989cecd`, why dispatch restored functionality, why `workflow_dispatch` retained cache-write authority, and how alert 23 superseded that conclusion.
- [x] 4.4 Add a short prominent historical-outcome notice linking to `outcome.md` at the top of each affected archive's proposal, design, and tasks files without changing the original decisions, checkboxes, dates, or validation claims below the notice.
- [x] 4.5 Check all current and archived links, render or validate the AsciiDoc documentation through the repository-owned documentation entry point, and confirm the archive now directs readers to `make-debug-build-cache-safe` instead of presenting either predecessor as the current recommendation.

## 5. Validate the Implementation Locally

- [x] 5.1 Run YAML/workflow linting, action-pin and Go-version checks, `go test` for workflow policy, and all tests that exercise request, builder, artifact, provenance, and publisher validation.
- [x] 5.2 Run the repository's broader required Go tests, `go vet`, `staticcheck`, `gosec`, and vulnerability scan; resolve regressions and record unrelated pre-existing findings separately without weakening checks.
- [x] 5.3 Run `openspec validate make-debug-build-cache-safe --type change --strict` and verify proposal, design, delta spec, tasks, documentation, and archive outcome records remain coherent.
- [x] 5.4 Inspect the complete diff and staged content to confirm no secret, generated binary, downloaded artifact, temporary research file, unrelated refactor, or obsolete dispatch/label exception remains.

## 6. Deliver and Prove the Flow with a GitHub PR Stack

- [x] 6.1 Create a bottom policy-transition branch from `main` that changes no debug workflow file, passes the existing protected-base checker, and accepts only the exact reviewed legacy and target topologies while adding the structural parser, caller-resolution, and regression tests.
- [x] 6.2 Use the official `github/gh-stack` extension to initialize and submit a two-layer stack whose bottom policy PR targets `main` and whose top workflow-cutover PR targets the policy branch; record both PR URLs and stack order.
- [ ] 6.3 Require the bottom PR's protected policy, focused tests, workflow lint, broader validation, and review to pass, then merge it first. If the top cutover cannot proceed promptly, disable the debug request path rather than broaden the transition rule.
- [ ] 6.4 Keep the top layer atomic: install the reusable request/builder/finalizer/publisher topology, documentation and archive outcomes, and strict policy removal of every legacy dispatch/label allowance. After GitHub retargets it to `main`, require protected policy and CodeQL Actions success for the exact pushed SHA with no replacement high-severity workflow alert.
- [ ] 6.5 Merge the strict top layer, confirm alert 23 closes without dismissal, then use an authorized test pull request to show that one exact `/build-debug` comment creates one successful `issue_comment` request run and one reusable builder invocation for the captured full head SHA.
- [ ] 6.6 Record run URLs and API evidence for workflow path, root event, caller/called jobs, permissions, runner, run attempt, intermediate artifact, trusted final artifact, and provenance; confirm an unauthorized or non-exact comment does not execute the builder.
- [ ] 6.7 Exercise stale-head and malformed-selection failure paths, and dispatch the protected publisher far enough to prove its validation accepts only the correct final artifact; do not approve or publish a prerelease solely for testing.
- [ ] 6.8 Update implementation evidence with GitHub and CodeQL versions, the documented cache-policy source and access date, observed identities, alert state, artifact cleanup, residual risks, and repository prerequisites. Disable the request path rather than reverting to either known-broken predecessor if acceptance fails.
