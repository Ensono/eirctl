# Purpose

TBD: Define least-privilege and trust-boundary controls for CI workflows.

# Requirements

## Requirement: Authoritative workflow policy uses trusted checker code
The required workflow-policy check SHALL execute checker code from the protected default branch and SHALL inspect pull-request workflow files strictly as data without executing pull-request-controlled code, actions, or scripts.

### Scenario: Pull request changes the policy checker
- **WHEN** a pull request weakens or removes its copy of the workflow-policy checker
- **THEN** the authoritative check continues to use the protected default-branch checker against the pull request's candidate workflow files

### Scenario: Trusted policy check inspects candidate workflows
- **WHEN** the authoritative policy workflow evaluates a pull request
- **THEN** it materializes only the candidate files needed for validation in a data-only location and reports pass or fail without executing content from that location

## Requirement: Untrusted code executes without privileged credentials
Any workflow job that checks out or executes pull-request-controlled code SHALL use a trusted workflow definition and an isolated builder with no more than read-only repository permissions, and SHALL NOT receive release credentials, protected environment secrets, package-write permission, contents-write permission, or default-branch cache authority. A privileged event handler MAY authorize and dispatch an untrusted build but SHALL NOT check out or execute the pull-request revision itself. The signal mechanism SHALL use an event that GitHub permits to create a new workflow run when emitted with the broker credential.

### Scenario: Pull request build runs
- **WHEN** a workflow builds or tests code from a pull request
- **THEN** the job has no more than `contents: read` repository access, no protected secrets or environment, and cache scope isolated from the default branch

### Scenario: Issue comment requests a debug build
- **WHEN** an authorized maintainer comments `/build-debug` on a pull request
- **THEN** the privileged comment handler performs no pull-request checkout or execution and emits a supported dispatch for a separate isolated builder

### Scenario: Signaled debug build runs
- **WHEN** the supported dispatch starts the debug builder
- **THEN** the builder validates the requested pull request and immutable head commit SHA, checks out that SHA with read-only authority, and records the pull request, commit SHA, run identity, and run attempt in artifact provenance

### Scenario: Broker emits a suppressed event
- **WHEN** static validation finds that the broker relies on a `GITHUB_TOKEN`-generated event that GitHub suppresses from starting workflows
- **THEN** validation fails before the broken signaling topology can merge

## Requirement: Debug publication is isolated from untrusted execution
The system SHALL publish a debug prerelease only from a separate trusted default-branch job or workflow that does not check out or execute pull-request-controlled code and that uses the existing protected `debug-release` environment before obtaining repository-write authority.

### Scenario: Maintainer publishes a successful debug build
- **WHEN** an authorized maintainer selects a successful unprivileged debug build whose workflow identity, event, repository, pull request, commit SHA, run attempt, and artifact provenance pass validation
- **THEN** the trusted default-branch publication flow downloads the build artifact as opaque data and publishes it with only the permissions required to create the prerelease

### Scenario: Publication metadata does not match
- **WHEN** the selected build failed, came from another repository, or does not match the intended pull request, current commit SHA, run attempt, or provenance
- **THEN** publication stops before obtaining or using repository-write authority

### Scenario: Publisher is dispatched from another ref
- **WHEN** a debug publication request runs from a branch or tag other than the protected default branch
- **THEN** both validation and publication jobs fail closed without obtaining repository-write authority

### Scenario: Artifact is published
- **WHEN** the trusted publication flow handles an artifact produced from untrusted code
- **THEN** it does not execute that artifact or any code from the pull-request checkout

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

## Requirement: Workflow policy rejects privileged untrusted execution
The repository's workflow security policy SHALL parse workflow YAML structurally, SHALL reject workflows that combine a privileged or default-branch trigger with checkout and execution of pull-request-controlled code, and SHALL validate the separated broker, builder, and publisher trust domains independent of YAML formatting.

### Scenario: Privileged workflow executes pull-request code
- **WHEN** structural validation finds an `issue_comment`, `pull_request_target`, `workflow_run`, `repository_dispatch`, or `workflow_dispatch` path that checks out a pull-request-controlled revision and executes content from that checkout
- **THEN** validation fails with a trust-boundary error

### Scenario: Dynamic checkout ref is not proven safe
- **WHEN** a privileged workflow passes an input, event value, or step output to `actions/checkout` and the policy cannot prove that value resolves to trusted source
- **THEN** validation treats the ref as untrusted and rejects subsequent code execution

### Scenario: External action consumes checked-out code
- **WHEN** a privileged workflow passes the checked-out workspace to an action that builds, interprets, packages, or otherwise executes repository content
- **THEN** validation classifies the action as code execution even when the action itself is pinned

### Scenario: Equivalent YAML syntax is used
- **WHEN** a privileged trigger or security-sensitive step is expressed with flow syntax, quoting, or different valid indentation
- **THEN** structural validation applies the same trust-boundary rule

### Scenario: Debug build topology is valid
- **WHEN** static workflow validation inspects the debug-build flow
- **THEN** it confirms that the broker performs no checkout, the builder uses a supported dispatch with immutable pull-request identity and read-only isolation, and the publisher runs only from the protected default branch without pull-request checkout or execution

## Requirement: Security validation includes static analysis
Before this change is accepted, the repository SHALL run Go tests, `go vet`, `staticcheck`, and `gosec` in addition to workflow policy, workflow lint, Go-version, vulnerability, and OpenSpec validation. Pre-existing findings outside the debug-build trust boundary SHALL be recorded as a separately tracked baseline; this change SHALL not introduce a regression in that baseline.

### Scenario: Change validation runs
- **WHEN** the debug-build trust-boundary change is validated
- **THEN** workflow policy, workflow lint, Go tests, Go-version, and OpenSpec validation complete successfully; all required analyzers run; and any unrelated pre-existing analyzer findings are documented for separate remediation
