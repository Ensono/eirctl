## Context

GitHub reports 21 open security findings: two privileged-workflow trust-boundary findings, five missing workflow-permission findings, thirteen advisories against `golang.org/x/crypto v0.51.0`, and one advisory against `golang.org/x/net v0.54.0`. The debug release workflow is triggered by an issue comment, checks out a pull request's head branch, executes its build code, and has `contents: write`; this allows unreviewed code and release authority to coexist in one workflow. Other workflows either omit permissions or grant broad write access to jobs that mainly build and test.

Git-backed configuration sources use `golang.org/x/crypto/ssh` directly and currently install `ssh.InsecureIgnoreHostKey()`, so SSH authenticates the client but not the server. The module declares Go 1.26 with a Go 1.26.3 toolchain, the local environment is Go 1.26.4, CI still selects Go 1.25.x, and the latest stable release at proposal time is Go 1.26.5. Build images use a mixture of floating major, minor, and old test-fixture tags.

The change crosses CI, release operations, dependency management, Git transport behavior, tests, documentation, and generated workflow fixtures. It must remove exploitable trust relationships without silently breaking normal SSH configuration or release maintenance.

## Goals / Non-Goals

**Goals:**

- Ensure untrusted pull-request code runs only with read-only repository access and no release credentials or protected secrets.
- Preserve an intentional, auditable way for maintainers to produce debug prereleases from a pull request.
- Set explicit least-privilege permissions for every workflow and elevate permissions only in jobs that require them.
- Authenticate Git SSH servers against known host keys by default while providing a conspicuous compatibility escape hatch.
- Clear the current `x/crypto` and `x/net` advisories through a coordinated dependency update and regression testing.
- Align maintained build and CI surfaces to the latest stable Go patch release, currently Go 1.26.5, and prevent future version drift.
- Add repeatable validation for workflow security, dependency vulnerabilities, and the SSH trust policy.

**Non-Goals:**

- Redesign the application configuration model beyond SSH trust settings needed by this change.
- Introduce a general-purpose secrets manager or replace GitHub Environments.
- Guarantee that every future dependency is vulnerability-free without ongoing dependency updates.
- Rewrite generated sample workflows by hand if a repository generator or fixture is their source of truth.
- Change historical/test-fixture Go image values when those values intentionally test rendering behavior rather than build the project.

## Decisions

### 1. Split debug build execution from release publication

The existing issue-comment workflow will be divided into two trust domains:

```text
Untrusted build domain                     Trusted publication domain
──────────────────────                     ──────────────────────────
issue comment / PR context                 maintainer workflow_dispatch
checkout immutable PR head SHA             identify completed build run
contents: read                             verify repo, PR, SHA, conclusion
no repository/environment secrets          download previously built artifact
build and upload artifact                  do not execute artifact or PR code
record PR/SHA metadata                     contents: write only here
                                            publish prerelease
```

The build side may continue accepting `/build-debug`, but it will resolve and record the immutable PR head SHA rather than trusting only a mutable branch name. It will have `contents: read`, no protected environment, and only the artifact permission provided by Actions. The publication side will require an authenticated maintainer invocation and a protected `debug-release` GitHub Environment approval where repository settings allow it. It will validate that the selected build completed successfully for this repository and that its recorded SHA still corresponds to the intended PR. It will treat binaries as opaque release assets and will not execute them or check out PR code.

This retains debug prereleases while enforcing a real credential boundary. Merely checking `author_association`, adding an `if` expression, or placing approval around the original privileged job was rejected because the same runner would still execute attacker-controlled code with eventual write authority. `pull_request_target` was rejected because it is easy to reintroduce privileged untrusted checkout.

### 2. Default all workflows to read-only and elevate per job

Each workflow will declare a top-level baseline:

```yaml
permissions:
  contents: read
```

Jobs that publish releases or containers will receive only the required write scopes (`contents: write` or `packages: write`). Reporting jobs will receive `checks: write` only if their reporting action requires it, and test execution will remain in a separate read-only job. Pull-request jobs will not receive `contents: write` or protected environment secrets. Infrastructure jobs will rely on protected environment credentials and explicit permissions rather than workflow-wide implicit defaults.

Generated/sample workflow output and its source template or golden fixture will be updated together. If `.github/workflows/gha__e__infra__e__sample.yml` is intended only as sample output and is not expected to execute, it will be moved out of the executable workflow directory; otherwise it will remain there with explicit permissions and environment protections.

This approach was chosen over repository-level default-permission settings alone because file-local declarations are reviewable, portable, and satisfy static analysis even when repository settings change.

### 3. Verify SSH hosts using OpenSSH-compatible known-host files

`getGitSSHAuth` will construct a `knownhosts` callback from `golang.org/x/crypto/ssh/knownhosts` instead of using `ssh.InsecureIgnoreHostKey()`. Resolution will follow familiar OpenSSH inputs:

1. Respect `UserKnownHostsFile` supplied through the selected SSH config and supported `GIT_SSH_COMMAND -o` options.
2. Otherwise use existing readable user defaults such as `~/.ssh/known_hosts` (and compatible secondary files where supported).
3. Include readable system known-host files where appropriate.
4. Fail closed with an actionable error if no usable host-key source exists, the host is unknown, or the key mismatches.

The callback will verify the effective hostname and port after SSH config aliases are resolved. Errors will distinguish unknown-host setup from changed-key/mismatch conditions without printing private key material.

For exceptional automation that cannot provision known hosts immediately, `StrictHostKeyChecking=no` in the selected SSH configuration or `GIT_SSH_COMMAND` will explicitly select the insecure callback. The application will emit a clear warning when this bypass is active. Secure verification remains the default; there will be no silent fallback after a verification error.

A bespoke trust database and trust-on-first-use prompt were rejected because eirctl can run non-interactively and should interoperate with users' established OpenSSH trust configuration.

### 4. Upgrade vulnerable modules as one tested dependency transaction

Implementation will update at least:

- `golang.org/x/crypto` from v0.51.0 to v0.52.0 or the latest compatible patched release.
- `golang.org/x/net` from v0.54.0 to v0.55.0 or the latest compatible patched release.

The update will use the selected latest Go toolchain, run module tidy, inspect the resulting transitive changes, and exercise private-key parsing, SSH config resolution, proxy paths, and Git-over-SSH tests. `govulncheck ./...` and GitHub alert rechecks will validate the resulting baseline. Direct requirements will not be downgraded merely to reduce the visible diff.

Updating the modules separately was rejected because the Go extended modules are released and constrained as a coordinated family, and a partial update can produce unnecessary module graph churn or leave the alert set open.

### 5. Use one exact latest-stable Go patch across maintained build surfaces

At implementation start, the latest stable Go version will be queried from the official Go release endpoint. The observed baseline is Go 1.26.5. The implementation will align:

- the `go` language version and exact `toolchain` directive in `go.mod`;
- `actions/setup-go` selections in active workflows;
- maintained builder images and shared build configuration;
- developer/contributor documentation and any version validation script.

Patch-specific pins will be used where the surface supports them, while the `go` directive will retain Go's canonical language-version format. Intentionally historical test data may keep older values, but it must not control real builds. A validation check will compare maintained pins so CI fails on accidental drift.

Floating `1`, `1.26`, and `1.25.x` selectors were rejected because they make builds inconsistent and can move without review. Automated future patch upgrades may be handled by Dependabot or Renovate, but are not required to complete this change.

### 6. Validate security properties, not only syntax

Tests and CI will cover:

- workflow linting/static analysis and assertions that untrusted jobs cannot receive write scopes;
- debug-build metadata validation and rejection of mismatched, failed, or untrusted build runs;
- known host success, unknown host failure, changed key failure, host alias/port handling, and explicit insecure opt-out warning;
- full unit/lint/build tests under the selected Go release;
- `govulncheck ./...` and confirmation that the 21 enumerated GitHub alerts are fixed or have documented, reviewed dispositions.

## Risks / Trade-offs

- **[Debug releases require an additional maintainer step]** → Keep the build command convenient, document publication by run ID/PR, and protect only the privileged transition.
- **[Artifacts from untrusted code remain intrinsically untrusted]** → Publication never executes them; prereleases are clearly marked and include source SHA provenance.
- **[Existing users may rely on disabled host verification]** → Provide actionable errors, document `ssh-keyscan`/known-host provisioning, and retain an explicit warning-producing opt-out rather than silently accepting hosts.
- **[Known-host lookup differs across platforms]** → Normalize home paths, test Unix and Windows path handling where feasible, and use OpenSSH-compatible library behavior.
- **[Exact Go patch pins require maintenance]** → Add automated update configuration or a documented periodic update path and a consistency check, making drift visible rather than implicit.
- **[Dependency updates may alter SSH behavior]** → Upgrade as a focused transaction, inspect release notes/module graph, and run targeted transport tests plus the complete suite.
- **[Generated workflow edits may be overwritten]** → Identify and update the generation source and regenerate checked-in outputs; add a reproducibility check if one already exists.
- **[GitHub repository settings are not fully representable in code]** → Document required environment protection/default permission settings and verify them with `gh api` during rollout.

## Migration Plan

1. Record the current GitHub alerts and repository Actions/environment settings as the pre-change baseline.
2. Confirm the latest stable Go patch, update the module/toolchain/build pins, upgrade vulnerable modules, and run targeted plus full validation.
3. Add SSH known-host verification and tests, then document preparation and the temporary explicit opt-out.
4. Convert workflow permissions to read-only defaults and isolate reporting/publishing permissions.
5. Introduce the untrusted debug-build artifact flow and trusted publication flow; configure the protected publication environment before enabling release writes.
6. Validate workflows on a test pull request from both an internal branch and a fork, then verify trusted prerelease publication from a known build run.
7. Re-run GitHub code scanning, Dependabot, secret scanning, and local vulnerability checks; review every previously enumerated finding.
8. Remove superseded workflow paths only after the replacement path succeeds.

Rollback is capability-specific: revert module/Go pins together if builds regress; temporarily require the documented explicit SSH opt-out while correcting known-host compatibility; and disable debug publication rather than restoring a workflow that executes untrusted code with write credentials. Security boundaries must not be rolled back to the vulnerable design.

## Open Questions

- Does the repository currently have a protected environment suitable for debug-release approval, or must maintainers create `debug-release` during implementation?
- Is `.github/workflows/gha__e__infra__e__sample.yml` intentionally executable, or should generated sample output live outside `.github/workflows/`?
- Should future exact Go patch updates be managed by existing Dependabot configuration or by a separate automated updater?