# CI security controls

## Toolchain baseline

The Go [official downloads page](https://go.dev/dl/) was checked on 2026-07-15. Go `1.26.5` was the newest stable, non-prerelease release and is the required maintained build baseline for this change.

## Required repository setup

Before enabling trusted debug prerelease publication, repository administrators must create a GitHub Environment named `debug-release` and configure it with required reviewer approval from the repository's release maintainers. The environment must not permit self-approval or administrator bypass unless that exception is explicitly approved by repository policy.

The baseline checked on 2026-07-15 found only `copilot`, `nonprod`, and `prod` environments; `debug-release` did not exist and must be created before the publication workflow can run.

Production deployment jobs must continue to use protected environments. Pull-request validation and untrusted debug-build jobs must not reference a protected environment or receive its secrets.

## Pull-request reporting

The pull-request execution job is intentionally limited to `contents: read` and does not receive `SONAR_TOKEN` or any other protected secret. It uploads the inert JUnit report as an artifact; a separate read-only-code reporting job downloads that artifact and receives only `checks: write` to publish the check. SonarCloud PR analysis is therefore disabled by this workflow until it can be run without exposing a protected token to pull-request-controlled code.

## Debug prerelease process

A `/build-debug` issue comment starts the **Debug build** workflow. It resolves the pull request to its immutable head SHA, checks out only that SHA with read-only repository access, and uploads binaries together with `debug-build-provenance.json`. The build workflow has no release permission, protected environment, or protected secret.

A maintainer publishes a successful build through **Publish debug prerelease** (`workflow_dispatch`), supplying the build run ID, pull-request number, and full commit SHA. Its validation job rejects failed runs and repository, PR, or SHA mismatches before the separate `debug-release` environment-gated publish job receives `contents: write`. The publisher downloads artifacts as opaque data: it never checks out or executes pull-request code or artifact contents. Published releases are prereleases and identify the PR and immutable source SHA; consumers must treat them as untrusted debug output.

## SSH server trust

Git-over-SSH verifies server identity by default using `UserKnownHostsFile` from SSH configuration or supported `GIT_SSH_COMMAND -o UserKnownHostsFile=...` options. If no custom file is selected, readable `~/.ssh/known_hosts`, `~/.ssh/known_hosts2`, and system known-host files are used. SSH aliases and ports are resolved before connection; non-default ports require the OpenSSH `[host]:port` known-host form.

Provision a host key before using a Git import, for example by obtaining and reviewing it through the repository owner's documented channel before adding it to `known_hosts`. An unknown host or changed host key stops the connection; verify the host identity out-of-band, remove only the obsolete entry, and add the reviewed replacement key. Errors intentionally identify only the host and remediation, never private keys, passphrases, or tokens.

`StrictHostKeyChecking=no` in the selected SSH configuration or supported `GIT_SSH_COMMAND -o` option is a temporary compatibility escape hatch. It disables server verification and emits a warning on every connection; use it only while provisioning a verified known-host entry, then remove it.
