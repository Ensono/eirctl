# CI security controls

## Toolchain baseline

The Go [official downloads page](https://go.dev/dl/) was checked on 2026-07-15. Go `1.26.5` was the newest stable, non-prerelease release and is the required maintained build baseline for this change.

## Immutable execution dependency evidence

The following registry manifest-list digests were resolved and reviewed on 2026-07-17. References retain their readable tag and pin the manifest digest in the maintained task-runner configuration:

| Context | Registry reference | Manifest digest |
| --- | --- | --- |
| `bash` | `mirror.gcr.io/bash:5.0.18-alpine3.22` | `sha256:943f4381e5c98f3be2f505464cd4b84ec84715251abf30f37476d3865ddcc2ce` |
| `go1x` | `mirror.gcr.io/golang:1.26.5-trixie` | `sha256:117e07f49461abb984fc8aef661432461ff43d06faa22c3b73af6a49ce325cb9` |
| `golint` | `mirror.gcr.io/golangci/golangci-lint:v2.11.3-alpine` | `sha256:b1c3de5862ad0a95b4e45a993b0f00415835d687e4f12c845c7493b86c13414e` |
| `goreleaser` | `ghcr.io/goreleaser/goreleaser:v2.14.3` | `sha256:848430a900a83ca0e18f2f149fb4ddcdaea74a667aa07224268b97d448833591` |

GitVersion is selected as exact release `6.0.5`; CI installs `golang.org/x/vuln/cmd/govulncheck@v1.6.0`. Go module checksums provide integrity for the latter module install.

## Required repository setup

The existing `debug-release` GitHub Environment has required-release-maintainer approval and a deployment branch policy permitting only `main`. Do not create, replace, or weaken that environment as part of normal workflow changes. The publisher must be dispatched from `main`; that branch restriction applies to the publisher workflow definition, while its validated artifact may originate from a pull request.

Production deployment jobs must continue to use protected environments. Pull-request validation and untrusted debug-build jobs must not reference a protected environment or receive its secrets.

## Authoritative workflow policy

**Trusted workflow policy** is the stable required check name for branch protection. It runs from the protected base revision through `pull_request_target`, checks out only that base SHA, and materializes pull-request workflow/configuration files as data before passing them to the base-branch checker with `--candidate-root`. It never executes candidate scripts or local actions.

The `Lint and Test` workflow's **Advisory workflow policy feedback (not required)** step runs against the pull-request checkout only for fast contributor feedback. It is not an authoritative security boundary and must not be configured as the required workflow-policy check.

## Ownership governance

`.github/CODEOWNERS` assigns `.github/CODEOWNERS`, executable workflow files, and `sonar-project.properties` to `@Ensono/digital-tools-maintainers`. The `main-is-main` ruleset must require code-owner review for these files. CODEOWNERS is merge governance only: it never authorizes runtime behavior and does not replace workflow isolation, immutable provenance validation, least-privilege permissions, or secret scoping.

## Pull-request reporting

The pull-request execution job is intentionally limited to `contents: read` and does not receive `SONAR_TOKEN`, a protected environment, or another privileged credential. It uploads the inert JUnit report for check publication and a deterministic seven-day `sonar-reports-<run-id>-<attempt>` artifact containing only `.coverage/out` and `.coverage/report-junit.xml`. Missing coverage is a visible failed Sonar preparation result; it is never a silently skipped analysis.

## Trusted SonarCloud pull-request analysis

`Trusted SonarCloud pull-request analysis` is a protected-default-branch `workflow_run` workflow for completed `Lint and Test` pull-request runs. It is the only PR path that can receive `SONAR_TOKEN`, and its token is scoped to the immutable `SonarSource/sonarqube-scan-action` scanner step. The workflow has only `contents: read` permission and does not restore or save a cache.

Before any secret-bearing step, it resolves the upstream run through the GitHub API and fails closed unless the workflow name/event, base repository and `main` branch, pull-request number, full 40-character head SHA, run ID, run attempt, current PR revision, and exactly one current report artifact all match. The validated artifact is treated as passive parser input: the protected validator rejects missing reports, files over the configured bounds, symlinks, special files, traversal-derived paths, and all unexpected content.

Only after that validation, the trusted workflow writes `analysis/sonar-project.properties`, then checks out the verified full PR SHA with `persist-credentials: false` into `analysis/source`. No pull-request command, local action, dependency, package manager, cache, container, binary, or alternate action may run after that checkout; the approved scanner is the sole consumer of the isolated source and reports. The scanner parser still processes attacker-controlled source while holding a narrowly scoped token, so scanner upgrades, full-SHA action pins, forced settings, and CODEOWNERS review remain essential controls rather than optional hardening.

The scanner runs from the trusted `analysis` root and forces the SonarCloud endpoint, organization (`ensono`), project (`Ensono_eirctl`), source/test/report paths, PR number/branch/base, immutable revision, and quality-gate wait. The pull-request copy of `sonar-project.properties` is not used and cannot redirect the endpoint or project identity. Trusted pushes to `main` use the same pinned scanner action, retain trusted report generation and revision metadata, and wait for the quality gate.

If the analyzer needs rollback, first remove the external SonarCloud required check from `main-is-main`, then disable the trusted PR analyzer. Do not restore a secret-bearing ordinary PR job or the former container scanner path; retain CODEOWNERS and the structural workflow-policy restrictions.

### Rejected PR analysis designs

- **Ordinary secret-bearing PR jobs** were rejected because pull-request code and forks could exfiltrate the token.
- **Privileged PR builds** (`pull_request_target` or similar) were rejected because checking out and executing PR content in a privileged context breaks the trust boundary.
- **Same-repository-only scanning** was rejected because fork pull requests also require reviewable SonarCloud analysis.
- **SonarCloud automatic analysis** was rejected because this project requires explicit Go coverage and JUnit report ingestion.

## Debug prerelease process

The debug prerelease process has three separate trust domains:

1. **Request broker:** an exact `/build-debug` comment on a pull request starts the **Request debug build** `issue_comment` workflow. It accepts only commenters with repository `write`, `maintain`, or `admin` permission, reads the current PR head SHA, and uses the supported workflow-dispatch API to start the builder from `main`. It never checks out or executes pull-request code and has only pull-request read plus workflow-dispatch authority.
2. **Untrusted builder:** **Debug build** receives typed PR-number and SHA inputs, verifies both against the repository API before checkout, then checks out only that immutable SHA with `contents: read`. It has no protected environment, secrets, release authority, or automatic Go dependency cache. Its provenance records the `workflow_dispatch` event, repository, PR, source SHA, workflow run ID, run attempt, and version.
3. **Trusted publisher:** a maintainer dispatches **Publish debug prerelease** from `main` with the build run ID, pull-request number, and full commit SHA. Both jobs fail closed on another ref. Read-only validation accepts only a successful dispatched Debug build from this repository and verifies the current PR SHA, run attempt, artifact ID, and provenance before the separate `debug-release` environment-gated job receives `contents: write`.

Repeated authorized requests remain serialized per PR. The publisher downloads artifacts as opaque data: it never checks out or executes pull-request code or artifact contents. Published releases are prereleases that identify the PR and immutable source SHA; consumers must treat them as untrusted debug output.

## SSH server trust

Git-over-SSH verifies server identity by default using ordered `UserKnownHostsFile` and `GlobalKnownHostsFile` selections from SSH configuration or supported `GIT_SSH_COMMAND -o ...` options. Quote or escape a path containing spaces (for example, `-o UserKnownHostsFile="~/.ssh/team known_hosts"`); repeated directives and multiple configured files retain their order. If no custom file is selected, readable `~/.ssh/known_hosts`, `~/.ssh/known_hosts2`, and platform defaults are used: `/etc/ssh/ssh_known_hosts{,2}` on Unix and `%ProgramData%\\ssh\\ssh_known_hosts` on Windows. SSH aliases and ports are resolved before connection; non-default ports require the OpenSSH `[host]:port` known-host form.

Provision a host key before using a Git import, for example by obtaining and reviewing it through the repository owner's documented channel before adding it to `known_hosts`. An unknown host or changed host key stops the connection; verify the host identity out-of-band, remove only the obsolete entry, and add the reviewed replacement key. Errors intentionally identify only the host and remediation, never private keys, passphrases, or tokens.

`StrictHostKeyChecking=no` in the selected SSH configuration or supported `GIT_SSH_COMMAND -o` option is a temporary compatibility escape hatch. It disables server verification and emits a warning on every connection; use it only while provisioning a verified known-host entry, then remove it.

When updating maintained CI contexts or tools, update the readable tag and reviewed digest together, record the manifest evidence above, and run `scripts/check-immutable-ci-dependencies.sh`. The check rejects tag-only task-runner images, floating GitVersion selectors, and `govulncheck@latest`.

## Validation baseline

On 2026-07-17, formatting, module reproducibility, Actionlint, the workflow-policy checker and tests, immutable-dependency checks and negative fixtures, SSH race tests, the full Go test suite, and targeted policy static analysis passed. `go vet ./...`, full-repository Staticcheck, and Gosec retain pre-existing findings outside this change (including unkeyed Docker test literals, unused test helpers, and existing file-path/permission checks). The new workflow-policy package is clean under Staticcheck; these baseline findings must not be hidden or waived by this security change.
