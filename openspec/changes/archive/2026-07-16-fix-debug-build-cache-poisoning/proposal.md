## Why

PR #102 fixes the repository's original privileged checkout and release-authority findings, but its replacement debug-build workflow still executes pull-request-controlled code under the privileged `issue_comment` event. CodeQL reports this as a new high-severity cache-poisoning path, so the trust boundary must move the build itself into an unprivileged pull-request context before the branch can be considered clean.

## What Changes

- Separate the `/build-debug` command broker from the debug build: the privileged comment handler may authorize and request a build, but must never check out or execute pull-request code.
- Run the requested build from a `pull_request` event scoped to the immutable PR revision, with read-only permissions, no protected environment or secrets, and cache isolation appropriate for untrusted code.
- Preserve the separate, maintainer-approved debug publication workflow, updating its provenance checks for the new build event and retaining the rule that publication never executes downloaded artifacts or PR code.
- Extend the repository's workflow policy validation so privileged triggers cannot check out or execute untrusted revisions and the debug-build trust domains cannot silently collapse again.
- Update CI security documentation to describe the broker, untrusted build, and trusted publication flow.
- Re-enumerate CodeQL and Dependabot after the branch is pushed: verify the new PR alert closes, no replacement alert appears, and the already-pinned `golang.org/x/crypto v0.52.0` and `golang.org/x/net v0.55.0` clear the 14 default-branch advisories after merge.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secure-ci-workflows`: Change on-demand debug builds from direct execution under `issue_comment` to a brokered, unprivileged pull-request build while preserving isolated trusted publication.

## Impact

- Affected workflows: `.github/workflows/debug-build.yml`, `.github/workflows/publish-debug-release.yml`, and potentially a dedicated comment-broker workflow.
- Affected validation and documentation: `scripts/check-workflow-policy/main.go`, its tests or fixtures, `scripts/check-workflow-security.sh`, `go vet`, `staticcheck`, `gosec`, and `docs/ci-security.md`.
- Operational impact: maintainers may continue requesting debug builds with `/build-debug`; the request is translated into a pull-request event rather than building directly in the comment-triggered run.
- Security posture: untrusted code loses access to a default-branch workflow context and its cache scope; release write authority remains isolated behind the protected `debug-release` environment.
- Dependencies: no additional module upgrade is expected because this branch already selects the patched `x/crypto` and `x/net` versions.
- Validation baseline: `go vet`, `staticcheck`, `gosec`, and `govulncheck` findings outside the debug-build trust boundary will be documented and remediated separately; this change must not introduce a regression in those results.
