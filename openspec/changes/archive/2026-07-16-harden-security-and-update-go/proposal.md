## Why

The project currently has 21 open GitHub security findings, including a privileged workflow that executes pull-request-controlled code, vulnerable Go networking and cryptography modules, and workflows that grant more access than their jobs require. The project also uses inconsistent Go versions across development, module metadata, and CI, so this change reduces immediate risk while establishing a consistent latest-stable Go baseline.

## What Changes

- Separate untrusted pull-request builds from trusted release publication so PR-controlled code never executes with repository-write or release credentials.
- Require explicit least-privilege `GITHUB_TOKEN` permissions for all workflows and jobs, including generated/sample infrastructure workflows.
- Upgrade `golang.org/x/crypto`, `golang.org/x/net`, and related module selections to patched versions, then verify Git-over-SSH and other affected behavior.
- Replace unconditional SSH host-key bypass with known-host verification and an explicit, documented opt-out for exceptional use cases.
- Align `go.mod`, the toolchain directive, CI setup, build images, and contributor guidance on the latest stable Go release; at proposal time this is Go 1.26.5.
- Add security-focused validation that checks workflow trust boundaries, workflow permissions, Go version consistency, dependency vulnerability status, and SSH host verification behavior.

## Capabilities

### New Capabilities
- `secure-ci-workflows`: Defines trust-boundary and least-privilege requirements for pull-request, debug-build, infrastructure, and release workflows.
- `verified-git-ssh`: Defines secure SSH host verification and explicit opt-out behavior for Git-backed configuration sources.
- `go-security-baseline`: Defines a consistent latest-stable Go toolchain and a vulnerability-free baseline for Go runtime dependencies.

### Modified Capabilities

None. There are no existing project specifications whose requirements are changed.

## Impact

- Affected workflows: `.github/workflows/debug-build.yml`, `.github/workflows/gha__e__infra__e__sample.yml`, `.github/workflows/pr.yml`, release workflows, and any workflow-generation source or fixtures that produce them.
- Affected Git transport code: `internal/config/loader_git.go` and its SSH-related tests and documentation.
- Affected build metadata: `go.mod`, `go.sum`, CI Go setup, Docker/build definitions, and contributor documentation.
- Dependencies: `golang.org/x/crypto`, `golang.org/x/net`, and transitive modules selected by the coordinated upgrade.
- Operations: maintainers invoking debug builds may use a revised trusted approval/publication flow; users connecting to previously unknown SSH hosts may need to populate `known_hosts` or explicitly opt out.