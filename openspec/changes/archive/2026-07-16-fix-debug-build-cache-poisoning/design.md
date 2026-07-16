## Context

PR #102 replaces a single issue-comment-triggered debug release workflow with separate build and publication workflows. Publication is now isolated behind `workflow_dispatch`, a protected environment, provenance validation, and job-scoped `contents: write`; that part of the trust split is sound.

The build half remains unsafe. `.github/workflows/debug-build.yml` runs on `issue_comment`, resolves a PR head SHA, checks it out, and executes the PR's Go pipeline. Although its `GITHUB_TOKEN` is read-only, an `issue_comment` run has default-branch cache scope and an Actions runtime token. Pull-request-controlled code can therefore poison caches consumed by later trusted runs. CodeQL records this as alert 10, `actions/cache-poisoning/poisonable-step`, at the build step.

The current policy checker verifies explicit permissions and immutable SHA checkout but does not model whether a trigger is privileged. The active `secure-ci-workflows` spec also incorrectly treats read-only permissions as sufficient isolation for an issue-comment build.

## Goals / Non-Goals

**Goals:**

- Preserve the maintainer `/build-debug` request experience.
- Ensure all checkout and execution of PR-controlled code happens under an unprivileged `pull_request` event and PR-scoped cache boundary.
- Keep release publication separate, explicitly approved, provenance-checked, and unable to execute PR code or artifacts.
- Make the trigger/checkout invariant enforceable by the repository's workflow policy checker.
- Close PR CodeQL alert 10 without dismissing it and without introducing a replacement finding.

**Non-Goals:**

- Automatically publish a debug release from a comment.
- Treat maintainer approval as proof that PR code is trusted.
- Redesign the general PR test workflow or release process.
- Change Go module versions already fixed by PR #102.
- Close default-branch Dependabot alerts before the patched dependency pins are merged.

## Decisions

### 1. Use a privileged broker and an unprivileged builder

The `/build-debug` path will contain three distinct trust domains:

```text
┌──────────────────────────────┐
│ Command broker               │
│ event: issue_comment         │
│ code: protected base branch  │
│ authority: PR label write    │
│ PR checkout/run: forbidden   │
└──────────────┬───────────────┘
               │ authorize maintainer
               │ apply build-request label
               ▼
┌──────────────────────────────┐
│ Untrusted builder            │
│ event: pull_request/labeled  │
│ code: immutable PR head SHA  │
│ authority: contents: read    │
│ cache scope: PR merge ref    │
└──────────────┬───────────────┘
               │ artifact + provenance
               ▼
┌──────────────────────────────┐
│ Trusted publisher            │
│ event: workflow_dispatch     │
│ code: protected base branch  │
│ authority: contents: write   │
│ PR checkout/run: forbidden   │
│ gate: debug-release env      │
└──────────────────────────────┘
```

The broker will parse only an exact `/build-debug` command on a PR, resolve the commenter’s repository permission through the GitHub API, and require `write`, `maintain`, or `admin`. It will then retrigger a dedicated label (remove it first when already present, then add it) so the `pull_request` workflow receives a `labeled` event. Broker runs for the same PR will be serialized to avoid racing label transitions.

The builder will run only when that dedicated label is added. It will use the PR event payload, verify the base repository is this repository, and check out `github.event.pull_request.head.sha`, never a branch name or comment-controlled ref. It will declare `contents: read`, reference no repository or environment secrets, attach no protected environment, and upload only build output plus provenance.

**Alternatives considered:**

- **Keep `issue_comment` with `contents: read`: rejected.** Repository permissions do not remove the Actions cache runtime token or default-branch cache scope.
- **Use `workflow_dispatch` with a PR number and disable `setup-go` caching: rejected.** Dispatch still runs in a trusted default-branch context; PR code can directly use the runtime cache API even if convenience caching is disabled.
- **Require maintainers to apply a label manually: secure but rejected as the primary path.** It loses the established comment command for little security benefit; the broker can translate the command without executing PR code.
- **Build every PR revision:** secure but rejected. Debug binaries are an on-demand operational artifact and need not consume resources for every PR update.

### 2. Treat the pull-request event as the cache security boundary

The builder will use the unprivileged `pull_request` context because GitHub scopes caches created by PR runs to the PR merge ref; they are not available to default-branch or sibling-branch runs. Automatic dependency caching should be disabled in the debug builder as defense in depth and to reduce unnecessary cache exposure, but this is not the primary control: event isolation is. The policy must therefore reject privileged-trigger execution of PR-controlled code rather than merely searching for `actions/cache`.

### 3. Update publication provenance for the new build event

The existing publication workflow remains structurally separate. Its validation will require that the selected run:

- belongs to this repository and the exact debug-build workflow;
- completed successfully under the `pull_request` event;
- is associated with the requested PR;
- records the same immutable head SHA as both the current PR and artifact provenance;
- matches the selected run attempt and artifact identity.

Publication continues to treat artifacts as opaque bytes and must not source, execute, install, or otherwise interpret PR-controlled code.

### 4. Extend policy checks to model privileged triggers

The workflow policy checker will classify `issue_comment`, `pull_request_target`, `workflow_run`, `repository_dispatch`, and `workflow_dispatch` as privileged/default-branch events. For such workflows it will reject any path that checks out a PR-controlled revision and later invokes a shell command, local action, build tool, or other executable content from that checkout.

The checker will also enforce the concrete debug-build topology:

- the broker has only the write scope needed to signal the PR and performs no checkout;
- the builder uses `pull_request` with the dedicated label gate, immutable head SHA, read-only permissions, no environment, and no secret references;
- the publisher remains separate and never checks out the PR.

This repository-specific topology check complements CodeQL and provides a fast local regression signal.

## Risks / Trade-offs

- **[Risk] Label remove/add retriggering races or drops concurrent requests.** → Serialize broker runs by PR, await each API operation, and make repeated requests intentionally supersede or queue behind the active request.
- **[Risk] A stale label event builds an outdated revision.** → Resolve all build identity from that event's immutable PR head SHA and record it in provenance; publication additionally requires the current PR head to match.
- **[Risk] Untrusted code attempts cache poisoning through the runtime API.** → Execute it only in the `pull_request` cache scope; disable automatic debug-build caching as defense in depth; never execute PR code in privileged event contexts.
- **[Risk] The broker becomes a repository-write escalation point.** → Give it only the narrow PR/issue label scope, authorize the commenter via repository permission, accept an exact command, and prohibit checkout and shell execution of PR data.
- **[Risk] Static policy checks overfit current YAML and miss data flow.** → Encode conservative trigger-level invariants, retain CodeQL as an independent semantic check, and include negative fixtures for privileged checkout/execution patterns.
- **[Risk] Publication metadata validation drifts from the builder event.** → Update validation and tests atomically; fail closed on missing, ambiguous, stale, or mismatched run/PR/SHA/provenance data.
- **[Risk] The protected `debug-release` environment is absent or weakly configured.** → Keep publication disabled until required reviewers and no-self-approval controls are configured and documented.

## Migration Plan

1. Add or refactor the issue-comment workflow into a broker that authorizes the requester and signals the PR without checkout or execution.
2. Convert the debug builder to `pull_request` `labeled` execution and retain immutable SHA provenance.
3. Update publisher validation for the new event and run association.
4. Extend policy tests/checks and update CI security documentation.
5. Run local policy, workflow lint, `go test`, `go vet`, `staticcheck`, `gosec`, Go-version, vulnerability, and OpenSpec validation. Require the trust-boundary checks to pass; record any pre-existing, unrelated analyzer findings as a separately tracked baseline and verify this change does not worsen it.
6. Push the branch and require CodeQL to close alert 10 with no new PR alerts.
7. Merge only after the `debug-release` protected environment is configured; confirm Dependabot rescans close the 14 advisories fixed by the branch's existing module pins.

Rollback is to disable the broker/builder and remove the label integration while retaining the trusted publication workflow. Do not roll back to executing PR code under `issue_comment`.

## Open Questions

- What dedicated label name should be used (`build-debug` or a repository-prefixed variant), and should the broker remove it after the build for UI clarity?
- Should a repeated request cancel an in-progress debug build for the same PR, or queue a second immutable-revision build?
- Are `debug-release` environment reviewers and anti-bypass settings already configured in repository settings, or must that remain an explicit rollout prerequisite?
