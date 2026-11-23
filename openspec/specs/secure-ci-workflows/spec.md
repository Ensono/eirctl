# Purpose

TBD: Define least-privilege and trust-boundary controls for CI workflows.

# Requirements

## Requirement: Untrusted code executes without privileged credentials
Any workflow job that checks out or executes pull-request-controlled code SHALL use a trusted workflow definition and an isolated GitHub-hosted builder with no more than read-only repository permissions, and SHALL NOT receive release credentials, protected environment secrets, package-write permission, contents-write permission, or default-branch cache-write authority. A trusted default-branch event handler MAY authorize an untrusted build and invoke a reusable builder in the same run only when GitHub documents the root event's default-branch cache access as read-only, the reusable job inherits that event, and no caller path can elevate the called job's permissions, secrets, environment, runner persistence, or cache authority. The authorization job SHALL NOT check out or execute the pull-request revision itself.

### Scenario: Pull request build runs
- **WHEN** a workflow builds or tests code from a pull request
- **THEN** the execution job uses a GitHub-hosted runner, has no more than `contents: read` repository access, receives no protected secret or environment, and cannot write the default-branch cache scope

### Scenario: Issue comment requests a debug build
- **WHEN** a maintainer with `write`, `maintain`, or `admin` permission comments exactly `/build-debug` on a pull request
- **THEN** trusted default-branch code validates the command, actor permission, base repository, pull-request number, and current full head SHA before calling the reusable builder, without checking out or executing pull-request content in the authorization job

### Scenario: Reusable debug build runs
- **WHEN** the authorized `issue_comment` run calls the debug builder
- **THEN** the builder inherits the caller event's documented read-only cache authority, revalidates the pull request and immutable head SHA immediately before checkout, executes with read-only repository authority and no secrets, and records the repository, event, pull request, commit SHA, run identity, and run attempt through a separate trusted finalization job

### Scenario: Pull request changes before reusable checkout
- **WHEN** the current pull-request head no longer equals the full SHA captured by authorization
- **THEN** the reusable builder fails before checkout and does not silently build the newer revision

## Requirement: Debug publication is isolated from untrusted execution
The system SHALL publish a debug prerelease only from a separate trusted default-branch job or workflow that does not check out or execute pull-request-controlled code and that uses the existing protected `debug-release` environment before obtaining repository-write authority. Publication SHALL authenticate the successful `issue_comment` request run, its reusable builder result, its clean-runner provenance finalization, and the exact final artifact before write authority is used.

### Scenario: Maintainer publishes a successful debug build
- **WHEN** an authorized maintainer selects a successful request run whose workflow path, `issue_comment` event, repository, pull request, commit SHA, run attempt, final artifact identity, and trusted provenance pass validation
- **THEN** the trusted default-branch publication flow downloads the final artifact as opaque data and publishes only the expected binaries with the permissions required to create the prerelease

### Scenario: Publication metadata does not match
- **WHEN** the selected request run failed, came from another workflow or repository, did not use `issue_comment`, or does not match the intended pull request, current commit SHA, run attempt, final artifact, or trusted provenance
- **THEN** publication stops before obtaining or using repository-write authority

### Scenario: Publisher is dispatched from another ref
- **WHEN** a debug publication request runs from a branch or tag other than the protected default branch
- **THEN** both validation and publication jobs fail closed without obtaining repository-write authority

### Scenario: Artifact is finalized
- **WHEN** the reusable builder finishes successfully
- **THEN** a fresh trusted runner revalidates immutable identity, handles the intermediate binaries only as bounded opaque files, creates provenance from trusted authorization and run metadata, and uploads the uniquely named final artifact without executing pull-request content

### Scenario: Artifact is published
- **WHEN** the trusted publication flow handles an artifact produced from untrusted code
- **THEN** it does not execute, source, install, add to the command environment, or otherwise interpret that artifact or any code from the pull-request checkout

## Requirement: Workflows declare least-privilege permissions
Every active workflow SHALL declare an explicit read-only permissions baseline, and each job SHALL receive only additional scopes that are necessary for that job's documented operation.

### Scenario: Build and test workflow is reviewed
- **WHEN** workflow permissions are statically inspected
- **THEN** build and test execution jobs have no contents-write, packages-write, or unrelated write scopes

### Scenario: Release job requires write access
- **WHEN** a job publishes a repository release or package
- **THEN** write permission is scoped to that job and limited to the corresponding contents or packages operation

### Scenario: Check reporting requires write access
- **WHEN** a test report must create a GitHub check
- **THEN** `checks: write` is isolated to the reporting operation rather than granted to unrelated workflow jobs

## Requirement: Deployment credentials use protected environments
Infrastructure and production deployment jobs SHALL obtain sensitive credentials through appropriately protected GitHub Environments and SHALL NOT expose those credentials to untrusted pull-request execution.

### Scenario: Pull request evaluates infrastructure workflow
- **WHEN** an untrusted pull request causes infrastructure validation to run
- **THEN** no production or protected environment credentials are made available

### Scenario: Production deployment runs
- **WHEN** a production deployment job requests protected credentials
- **THEN** GitHub Environment protection and any required approval are enforced before the credentials become available

## Requirement: Generated workflows preserve security controls
When an executable workflow is generated from a template or fixture, the generator and checked-in output SHALL produce the same trust-boundary and least-privilege controls.

### Scenario: Generated workflow is regenerated
- **WHEN** the workflow generation process is run
- **THEN** the resulting executable workflow retains explicit permissions and protected handling of secrets

### Scenario: File is documentation-only
- **WHEN** a generated YAML file is intended only as a sample and not as an executable workflow
- **THEN** it is stored outside `.github/workflows/`

### Scenario: Code scanning reports a privileged checkout vulnerability
- **WHEN** CodeQL or the repository's configured code-scanning tool reports a new cache-poisoning, untrusted-checkout, or equivalent high-severity workflow alert for the debug-build or report-upload topology
- **THEN** the implementation is not accepted, and the alert SHALL be resolved by design rather than dismissed, suppressed, or bypassed

## Requirement: GitHub quality reporting
The CI system SHALL generate SARIF for Go vet and cyclomatic complexity, upload it to GitHub code scanning, and upload Cobertura coverage for trusted main pushes and same-repository pull requests to GitHub Code Quality. Report generation SHALL run without write authority and upload jobs SHALL execute only pinned report-download and upload actions.

### Scenario: Same-repository pull request
- **WHEN** Go analysis and tests succeed for a pull request from the repository
- **THEN** the workflow uploads Go quality SARIF for the pull-request analysis and coverage for its head revision

### Scenario: Fork pull request
- **WHEN** a pull request originates from a fork
- **THEN** coverage upload is skipped because GitHub does not grant fork workflows the required permission; the skipped upload is not treated as coverage data

### Scenario: Report generation or upload fails
- **WHEN** an expected report cannot be generated or uploaded
- **THEN** the corresponding workflow job fails visibly

### Scenario: Coverage protection is enabled
- **WHEN** maintainers enable GitHub's Restrict code coverage rule
- **THEN** they require the coverage upload job as a status check and account for the fork limitation

## Requirement: Security-sensitive CI configuration requires code-owner review
The repository SHALL assign executable workflows and the ownership policy itself to `@Ensono/digital-tools-maintainers`, and the active `main` ruleset SHALL require approval from a matching code owner before such changes can merge.

### Scenario: Workflow changes
- **WHEN** a pull request changes a file under `/.github/workflows/**`
- **THEN** GitHub requests review from `@Ensono/digital-tools-maintainers` and the ruleset blocks merge until an eligible code owner approves

### Scenario: Ownership policy changes
- **WHEN** a pull request changes `/.github/CODEOWNERS`
- **THEN** the ownership file's own rule requires `@Ensono/digital-tools-maintainers` approval before merge

### Scenario: Code-owner review is not runtime authorization
- **WHEN** an unapproved pull request triggers CI before review
- **THEN** workflow isolation, least privilege, provenance validation, and secret scoping remain fully enforced independently of CODEOWNERS

## Requirement: Security validation includes static analysis
Before this change is accepted, the repository SHALL run Go tests, `go vet`, `staticcheck`, and `gosec` in addition to workflow lint, Go-version, vulnerability, and OpenSpec validation. Pre-existing findings outside the debug-build trust boundary SHALL be recorded as a separately tracked baseline; this change SHALL not introduce a regression in that baseline.

### Scenario: Change validation runs
- **WHEN** the debug-build trust-boundary change is validated
- **THEN** workflow lint, Go tests, Go-version, and OpenSpec validation complete successfully; all required analyzers run; and any unrelated pre-existing analyzer findings are documented for separate remediation
