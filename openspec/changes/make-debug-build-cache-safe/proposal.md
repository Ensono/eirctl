## Why

The current on-demand debug builder executes pull-request-controlled code from a `workflow_dispatch` run, which GitHub grants write access to the default-branch cache scope; CodeQL therefore reports alert 23 (`actions/cache-poisoning/poisonable-step`). The earlier `GITHUB_TOKEN` label-broker design avoided that cache authority but did not actually start the labeled pull-request workflow, while GitHub's June 26, 2026 read-only-cache change now gives this repository a simpler supported boundary: keep the authorized request under `issue_comment` and invoke the builder as a reusable workflow that inherits the caller's read-only cache access.

## What Changes

- Replace the broker's `workflow_dispatch` call with an in-run reusable-workflow call after the exact `/build-debug` command, requester permission, pull-request identity, and current immutable head SHA have been validated.
- Convert the debug builder from a manually dispatchable workflow to a `workflow_call`-only reusable workflow with explicit read-only permissions, no secrets or protected environment, no automatic dependency caching, and checkout of only the validated full pull-request SHA.
- Update debug publication to authenticate the successful request workflow run and its immutable build identity before a separate protected job publishes the downloaded binaries as opaque artifacts.
- Refactor workflow policy so cache-write authority is modeled separately from default-branch workflow selection, using GitHub's documented trigger classification and treating reusable jobs as inheriting their caller's event.
- Add regression fixtures for the working `issue_comment` → `workflow_call` topology, the suppressed `GITHUB_TOKEN` label topology, and the vulnerable `workflow_dispatch` checkout-and-execute topology.
- Update CI security documentation with the June 26, 2026 GitHub cache-control change, why `issue_comment` is suitable for this narrowly constrained builder, and why `workflow_dispatch` and token-generated label events are unsuitable.
- Clarify the two archived debug-build changes with prominent historical-status notes that distinguish the statically clean but non-functional label attempt from the functional but cache-write-capable dispatch replacement, and point readers to this corrective change. Preserve the archived record rather than rewriting its original decisions or completion history.
- Deliver the protected-policy migration and workflow cutover as a native GitHub stacked pull request: a bottom transition-policy layer accepts only the exact reviewed legacy and target topologies, while the top layer switches workflows and removes the legacy allowance. GitHub's `pull_request_target` policy workflow continues to execute checker code from the default branch; the stack makes the required bottom-first merge order explicit so that checker reaches `main` before the cutover is evaluated there.
- Require live post-cutover verification that `/build-debug` starts exactly one reusable builder, the artifact can be selected by the publisher, CodeQL alert 23 closes without dismissal, and no replacement workflow alert appears.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secure-ci-workflows`: Replace the vulnerable dispatched debug builder with a reusable builder that inherits the authorized `issue_comment` run's read-only cache boundary, and require policy, publication, documentation, and validation to enforce that topology.

## Impact

- Affected workflows: `.github/workflows/debug-build-request.yml`, `.github/workflows/debug-build.yml`, and `.github/workflows/publish-debug-release.yml`.
- Affected policy and tests: `scripts/check-workflow-policy/main.go`, its unit tests and fixtures, and workflow-security validation scripts.
- Affected documentation and planning history: `docs/ci-security.adoc` plus the archived `2026-07-16-fix-debug-build-cache-poisoning` and `2026-07-17-fix-security-review-findings` change directories.
- Delivery constraint: GitHub protected-base policy enforcement requires a two-layer stacked rollout; the runtime requirements are unchanged.
- GitHub behavior dependency: the documented read-only default-branch cache token issued to `issue_comment` runs on GitHub.com, announced June 26, 2026; the implementation must fail review if platform documentation or CodeQL's trigger classification no longer supports that assumption.
- No application API, Go module, artifact format, release-consumer, or new long-lived credential is introduced.
