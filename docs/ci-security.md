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

## Trusted workflow checkout provenance

Privileged `pull_request_target` and `workflow_run` jobs must not pass a `github.event.pull_request.*` or `github.event.workflow_run.*` expression to `actions/checkout`. OpenSSF Scorecard reports every such form as `Dangerous-Workflow`, even when a separate condition proves that the revision came from protected `main`.

The authoritative pull-request policy workflow therefore relies on `pull_request_target`'s implicit protected base revision and omits the checkout `ref` entirely. Trusted release workflows use the literal `main` ref, then immediately compare the checked-out commit to `github.event.workflow_run.head_sha` supplied through an environment variable. The release job fails closed before GitVersion, builds, registry login, tagging, or publication if protected `main` has advanced or the completed upstream run does not match. Release checkouts never persist credentials; the binary release job receives its `contents: write` token only in the Git Data API tag-creation step after the build. The structural checker rejects the Scorecard dynamic checkout forms and requires this static-checkout-plus-verification and non-persistent-credential topology for every release job.

## Authoritative workflow policy

**Trusted workflow policy** is the stable required check name for branch protection. It runs from the protected base revision through `pull_request_target`, checks out only that base SHA, and materializes pull-request workflow/configuration files as data before passing them to the base-branch checker with `--candidate-root`. It never executes candidate scripts or local actions.

The `Lint and Test` workflow's **Advisory workflow policy feedback (not required)** step runs against the pull-request checkout only for fast contributor feedback. It is not an authoritative security boundary and must not be configured as the required workflow-policy check.

## Ownership governance

`.github/CODEOWNERS` assigns `.github/CODEOWNERS`, executable workflow files, and `sonar-project.properties` to `@Ensono/digital-tools-maintainers`. The `main-is-main` ruleset must require code-owner review for these files. CODEOWNERS is merge governance only: it never authorizes runtime behavior and does not replace workflow isolation, immutable provenance validation, least-privilege permissions, or secret scoping.

## Pull-request reporting

The pull-request execution job is intentionally limited to `contents: read` and does not receive `SONAR_TOKEN`, a protected environment, or another privileged credential. It uploads the inert JUnit report for check publication and selects only `.coverage/out` and `.coverage/report-junit.xml` for a deterministic seven-day `sonar-reports-<run-id>-<attempt>` artifact. GitHub's artifact action strips their common `.coverage` parent, so the downloaded artifact contract is exactly two root-level regular files named `out` and `report-junit.xml`. After validating names, types, and size bounds, protected code requires canonical repository-relative Go coverage records, prefixes each record path with the fixed `source/` scanner namespace used by API materialization, and normalizes both files under `reports/.coverage/`. This preserves the established report locations and aligns coverage keys with `analysis/source` without broadening the accepted artifact surface. Missing or malformed coverage is a visible failed Sonar preparation result; it is never a silently skipped analysis.

## Trusted SonarCloud pull-request analysis

`Trusted SonarCloud pull-request analysis` is a protected-default-branch `workflow_run` workflow for completed `Lint and Test` pull-request runs. It is the only PR path that can receive `SONAR_TOKEN`. The workflow has only `contents: read` permission, uses no job container or services, restores or saves no cache, and supports both same-repository and fork pull requests without executing either source tree.

Before any secret-bearing step, protected code resolves and binds the expected workflow and event, base repository and `main` branch, verified head repository (including fork identity), pull-request number, full current head SHA, run ID, run attempt, and exactly one unexpired `sonar-reports-<run-id>-<attempt>` artifact. The report download remains tied to that run and revision. Its protected validator accepts only bounded regular UTF-8 coverage and JUnit files and rejects missing reports, malformed coverage modes or records, unsafe coverage paths, invalid encoding, oversized content, symlinks, special files, traversal-derived paths, and unexpected entries.

Pull-request source is never passed to `actions/checkout`, `git checkout`, `git fetch`, an archive extractor, or another source action. After writing trusted `analysis/sonar-project.properties` outside the source root, the protected standard-library helper resolves the exact head commit and root tree through the verified head repository's Git Data API, requires a complete non-truncated recursive tree, validates every canonical path and mode, and retrieves each selected regular non-executable `.go` blob by its tree-recorded SHA. It verifies the API identity, declared and decoded size, and Git blob identity before making an exclusive `0644` write beneath the newly created `analysis/source` root. Symlinks, submodules, special or unknown entries, unsafe or duplicate paths, executable `.go` files, non-Go files, and changed PR heads fail closed or remain unmaterialized. The helper rechecks the current PR head after all writes.

The protected source limits are 384 recursive tree entries, 160 selected Go files, 160 path bytes, 131,072 bytes per Go blob, and 1,048,576 aggregate Go bytes. The measured baseline and headroom are recorded in the change verification notes. These values are reviewed constants represented by the workflow policy; they are not attacker-controlled inputs. Increasing one requires a new baseline, hostile boundary tests, and CODEOWNERS review.

The scanner is the only command or action after materialization. It runs from the trusted `analysis` root and forces the SonarCloud endpoint, organization (`ensono`), project (`Ensono_eirctl`), source/test/report paths, PR number/branch/base, immutable revision, and quality-gate wait. The action is pinned to full SHA `22918119ff8e1ca75a623e15c8296b6ea4fbe28f`; its CLI is fixed at `8.1.0.6389`, binaries URL at `https://binaries.sonarsource.com/Distribution/sonar-scanner-cli`, and signature verification remains enabled. `SONAR_TOKEN` exists only in this scanner step. The pull-request copy of `sonar-project.properties`, `.git`, workflows, scripts, local actions, dependency metadata and hooks, containers, archives, and binaries are never materialized.

This boundary narrows, but cannot eliminate, the risk that the trusted scanner parser processes hostile Go source and passive reports while holding its scoped token. Full-SHA action pins, forced settings and runtime, the no-post-materialization-execution policy, CodeQL, and CODEOWNERS review therefore remain required controls. A new `actions/untrusted-checkout/high` or equivalent high-severity CodeQL alert is a design failure and must be fixed in code; dismissal, suppression, lowering the ruleset threshold, or bypassing `main-is-main` is not acceptance.

Trusted pushes to `main` use the same pinned scanner action, retain trusted report generation and revision metadata, and wait for the quality gate. If the PR analyzer needs rollback, first remove the observed external SonarCloud required check from `main-is-main`, then disable the trusted PR analyzer. Do not restore a secret-bearing ordinary PR job, privileged checkout, source archive, or former container scanner path. Retain CODEOWNERS and the stricter structural no-checkout policy.

### Rejected PR analysis designs

- **Ordinary secret-bearing PR jobs** were rejected because pull-request code and forks could exfiltrate the token.
- **Privileged PR builds** (`pull_request_target` or similar) were rejected because executing PR content in a privileged context breaks the trust boundary.
- **Privileged immutable checkout** was rejected even with a full SHA and disabled credential persistence: CodeQL correctly treats pull-request-controlled checkout in `workflow_run` as `actions/untrusted-checkout/high`, and checkout also materializes executable repository content plus Git metadata.
- **Untrusted or generic source archives** were rejected because path/type handling and broad extraction recreate a checkout-like attack surface without per-blob provenance.
- **Same-repository-only scanning** was rejected because fork pull requests also require reviewable SonarCloud analysis.
- **SonarCloud automatic analysis** was rejected because this project requires explicit Go coverage and JUnit report ingestion and an auditable repository-controlled trust boundary.

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
