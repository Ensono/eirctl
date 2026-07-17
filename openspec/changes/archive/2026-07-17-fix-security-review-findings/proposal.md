## Why

The security-hardening branch still contains a broken debug-build signal and several trust-boundary, supply-chain, policy-enforcement, and SSH portability gaps. These issues should be resolved before relying on the new debug publication flow or treating the repository checks as authoritative.

## What Changes

- Replace the `GITHUB_TOKEN`-generated label signal with a supported dispatch path that reliably starts an isolated, read-only debug builder without executing pull-request code in the broker.
- Restrict debug publication to the protected default-branch workflow definition and extend static policy checks to enforce that boundary.
- Run authoritative workflow-policy validation from trusted default-branch code while treating pull-request workflows as data, so a pull request cannot weaken both the policy and its checker.
- Replace formatting-sensitive workflow security heuristics with structural validation and add negative fixtures for privileged-trigger, dynamic-ref, and code-consuming-action bypasses.
- Pin task-runner images and downloaded CI tools to reviewed immutable versions or digests, including release build contexts, GitVersion, and `govulncheck`.
- Preserve quoted known-host paths and discover platform-appropriate system known-host files, with regression tests for paths containing spaces and Windows defaults.
- Complete the delivery workflow by committing and pushing the verified implementation on the current branch, updating the associated pull request without overwriting unrelated description content, and monitoring required checks to a terminal green state.
- Provide a comprehensive post-merge manual testing plan covering positive, negative, security-boundary, portability, release, and rollback scenarios.
- Keep using the existing `debug-release` GitHub Environment; creating or provisioning that environment is explicitly outside this change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secure-ci-workflows`: Make broker-to-builder signaling reliable, require the trusted publisher to use the default-branch definition, and make workflow-policy enforcement structural and tamper-resistant.
- `go-security-baseline`: Require immutable task-runner image and downloaded CI-tool selections for maintained and privileged build paths.
- `verified-git-ssh`: Preserve valid known-host paths containing spaces and use platform-appropriate default trust locations.

## Impact

Affected areas include the debug request, build, publication, and pull-request workflows; workflow-policy tooling and fixtures; task-runner container definitions; GitVersion and vulnerability-scanner setup; SSH configuration parsing and host-key tests; CI documentation; and the corresponding OpenSpec requirements. Delivery also includes current-branch Git operations, pull-request communication, required-check monitoring, and a post-merge manual validation plan. The implementation may add a trusted policy workflow and checker input-root support, but it will not create or modify the existence of the `debug-release` environment, create or merge a pull request automatically, or push to a branch other than the current implementation branch.
