## 1. Confirm Security Baseline and Repository Controls

- [x] 1.1 Use `gh` to snapshot the 21 open code-scanning and Dependabot alerts, Actions default permissions, existing GitHub Environments, and branch/release protection relevant to this change
- [x] 1.2 Trace `.github/workflows/gha__e__infra__e__sample.yml` to its generator or fixture and decide whether it is executable workflow behavior or documentation-only output
- [x] 1.3 Confirm or document creation of a protected `debug-release` GitHub Environment with required maintainer approval before enabling the trusted publication workflow

## 2. Contain GitHub Actions Trust Boundaries

- [x] 2.1 Replace the privileged `/build-debug` path with a read-only build job that resolves an immutable pull-request head SHA and receives no release credentials or protected secrets
- [x] 2.2 Upload debug binaries plus repository, pull-request, commit-SHA, and workflow-run provenance as build artifacts without publishing a release
- [x] 2.3 Add a maintainer-triggered trusted publication workflow that validates the selected successful build metadata before acquiring `contents: write`
- [x] 2.4 Ensure the trusted publication workflow treats artifacts as opaque files, never checks out or executes pull-request code, and publishes an identifiable prerelease through the protected environment
- [x] 2.5 Add tests or static assertions for rejected failed runs, repository/PR/SHA mismatches, write permission in untrusted jobs, and execution of untrusted artifacts

## 3. Apply Least-Privilege Workflow Permissions

- [x] 3.1 Add an explicit `contents: read` baseline to every active workflow and remove workflow-wide write scopes that are not universally required
- [x] 3.2 Isolate release `contents: write`, container `packages: write`, and test-report `checks: write` permissions to only the jobs that require them
- [x] 3.3 Ensure pull-request and infrastructure validation jobs cannot access protected deployment credentials, while production operations use protected GitHub Environments
- [x] 3.4 Update the generator/template and checked-in output for `gha__e__infra__e__sample.yml`, or move documentation-only sample YAML outside `.github/workflows/`
- [x] 3.5 Run workflow syntax, lint, and security-policy checks and confirm the seven current Actions code-scanning patterns are no longer present

## 4. Align the Go Toolchain and Dependency Baseline

- [x] 4.1 Query the official Go release source at implementation time and record the newest stable non-prerelease patch, using Go 1.26.5 unless a newer stable release exists
- [x] 4.2 Align `go.mod` language/toolchain metadata, active `actions/setup-go` steps, maintained builder images, shared build configuration, and contributor prerequisites to the selected exact Go patch
- [x] 4.3 Preserve intentionally historical Go versions used only as test fixtures and verify they cannot control production builds
- [x] 4.4 Add an automated version-consistency check that reports divergent maintained Go pins and run it in pull-request CI
- [x] 4.5 Upgrade `golang.org/x/crypto` to v0.52.0 or later and `golang.org/x/net` to v0.55.0 or later as one coordinated module transaction
- [x] 4.6 Run module tidy with the selected toolchain, review direct and transitive dependency changes, and verify a second tidy produces no diff
- [x] 4.7 Add or integrate `govulncheck ./...` into security validation and confirm the updated module versions are outside all 14 currently reported vulnerable ranges

## 5. Verify Git SSH Server Identity

- [x] 5.1 Extend the SSH configuration model and parsing to support `UserKnownHostsFile` and `StrictHostKeyChecking` from SSH config and supported `GIT_SSH_COMMAND -o` options
- [x] 5.2 Resolve configured, default user, and applicable system known-host files with documented precedence and normalized cross-platform paths
- [x] 5.3 Replace unconditional `ssh.InsecureIgnoreHostKey()` with a `knownhosts` callback that verifies the effective hostname and port and fails closed
- [x] 5.4 Return actionable, credential-safe errors for missing trust sources, unknown hosts, and changed host keys
- [x] 5.5 Implement the explicit `StrictHostKeyChecking=no` compatibility path and emit a clear warning whenever insecure host handling is selected
- [x] 5.6 Add tests for trusted keys, unknown hosts, changed keys, hashed entries where supported, aliases, non-default ports, custom known-host files, default files, and insecure opt-out behavior
- [x] 5.7 Re-run private-key, passphrase, SSH configuration, proxy, and Git-over-SSH regression tests against the upgraded Go modules

## 6. Documentation and End-to-End Validation

- [x] 6.1 Document the debug build and trusted publication process, required maintainer/environment controls, immutable SHA provenance, and prerelease trust expectations
- [x] 6.2 Document known-host provisioning, effective host/port behavior, `UserKnownHostsFile`, host-key mismatch recovery, and the risks of the temporary insecure opt-out
- [ ] 6.3 Run the complete lint, unit, race, schema, build, and relevant integration suites under the selected Go release
- [ ] 6.4 Validate the workflow changes with internal and fork-style pull-request scenarios without exposing write tokens or protected secrets
- [ ] 6.5 Re-enumerate code-scanning, Dependabot, and secret-scanning alerts with `gh`; verify all 21 baseline findings are resolved after GitHub rescans or document any reviewed residual disposition
- [ ] 6.6 Verify the working tree contains only intentional source, generated, module, documentation, and OpenSpec changes and record any repository-setting steps that maintainers must complete