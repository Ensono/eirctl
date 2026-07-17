# CI security controls

## Toolchain baseline

The Go [official downloads page](https://go.dev/dl/) was checked on 2026-07-15. Go `1.26.5` was the newest stable, non-prerelease release and is the required maintained build baseline for this change.

## Required repository setup

Before enabling trusted debug prerelease publication, repository administrators must create a GitHub Environment named `debug-release` and configure it with required reviewer approval from the repository's release maintainers. The environment must not permit self-approval or administrator bypass unless that exception is explicitly approved by repository policy.

The baseline checked on 2026-07-15 found only `copilot`, `nonprod`, and `prod` environments; `debug-release` did not exist and must be created before the publication workflow can run.

Production deployment jobs must continue to use protected environments. Pull-request validation and untrusted debug-build jobs must not reference a protected environment or receive its secrets.

## Pull-request reporting

The pull-request execution job is intentionally limited to `contents: read` and does not receive `SONAR_TOKEN` or any other protected secret. It uploads the inert JUnit report as an artifact; a separate read-only-code reporting job downloads that artifact and receives only `checks: write` to publish the check. SonarCloud PR analysis remains disabled because it would expose a protected token to pull-request-controlled code. A separate `sonarcloud` job runs only after tests succeed for a trusted push to `main`; it checks out that trusted source, generates the reports, and receives `SONAR_TOKEN` only for the scan.

## Debug prerelease process

The debug prerelease process has three separate trust domains:

1. **Request broker:** an exact `/build-debug` comment on a pull request starts the **Request debug build** `issue_comment` workflow. It accepts only commenters with repository `write`, `maintain`, or `admin` permission. The broker never checks out or executes pull-request code; it has only `issues: write` so it can signal the request.
2. **Untrusted builder:** the broker removes and re-adds the `build-debug` label. This emits a `pull_request` `labeled` event for **Debug build**, which checks out the event's immutable `pull_request.head.sha` with `contents: read`. It has no protected environment, secrets, release authority, or automatic Go dependency cache. The artifact includes `debug-build-provenance.json` with the repository, PR, head SHA, workflow run ID, run attempt, and version.
3. **Trusted publisher:** a maintainer starts **Publish debug prerelease** (`workflow_dispatch`) with the build run ID, pull-request number, and full commit SHA. Validation accepts only a successful `pull_request` run of the exact debug-build workflow associated with that PR, and rejects repository, current-PR-SHA, run-attempt, artifact, or provenance mismatches before the separate `debug-release` environment-gated job receives `contents: write`.

Repeated authorized requests are serialized per PR. Each new request intentionally supersedes the prior label state by removing and re-adding `build-debug`, so the resulting build records the latest immutable SHA from its pull-request event. Operators may remove a stale `build-debug` label to avoid a pending request; the label is a request signal, not evidence that a binary is trusted.

Before enabling publication, administrators must configure `debug-release` with required release-maintainer reviewers and prohibit self-approval and administrator bypass unless repository policy explicitly approves an exception. The publisher downloads artifacts as opaque data: it never checks out or executes pull-request code or artifact contents. Published releases are prereleases that identify the PR and immutable source SHA; consumers must treat them as untrusted debug output.

## SSH server trust

Git-over-SSH verifies server identity by default using `UserKnownHostsFile` from SSH configuration or supported `GIT_SSH_COMMAND -o UserKnownHostsFile=...` options. If no custom file is selected, readable `~/.ssh/known_hosts`, `~/.ssh/known_hosts2`, and system known-host files are used. SSH aliases and ports are resolved before connection; non-default ports require the OpenSSH `[host]:port` known-host form.

Provision a host key before using a Git import, for example by obtaining and reviewing it through the repository owner's documented channel before adding it to `known_hosts`. An unknown host or changed host key stops the connection; verify the host identity out-of-band, remove only the obsolete entry, and add the reviewed replacement key. Errors intentionally identify only the host and remediation, never private keys, passphrases, or tokens.

`StrictHostKeyChecking=no` in the selected SSH configuration or supported `GIT_SSH_COMMAND -o` option is a temporary compatibility escape hatch. It disables server verification and emits a warning on every connection; use it only while provisioning a verified known-host entry, then remove it.
