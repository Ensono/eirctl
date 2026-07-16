# Purpose

TBD: Define least-privilege and trust-boundary controls for CI workflows.

# Requirements

## Requirement: Untrusted code executes without privileged credentials
Any workflow job that checks out or executes pull-request-controlled code SHALL use read-only repository permissions and SHALL NOT receive release credentials, protected environment secrets, package-write permission, or contents-write permission.

### Scenario: Pull request build runs
- **WHEN** a workflow builds or tests code from a pull request
- **THEN** the job executes with no more than `contents: read` repository access and without protected secrets

### Scenario: Issue comment requests a debug build
- **WHEN** an issue comment requests a debug build for a pull request
- **THEN** the build job resolves the pull request to an immutable commit SHA and executes that revision without repository-write or release authority

## Requirement: Debug publication is isolated from untrusted execution
The system SHALL publish a debug prerelease only from a separate trusted job or workflow that does not check out or execute pull-request-controlled code and that requires an authenticated maintainer action or protected-environment approval.

### Scenario: Maintainer publishes a successful debug build
- **WHEN** an authorized maintainer selects a successful debug build run whose repository, pull request, and commit SHA metadata pass validation
- **THEN** the trusted publication flow downloads the build artifact as opaque data and publishes it with only the permissions required to create the prerelease

### Scenario: Publication metadata does not match
- **WHEN** the selected build failed, came from another repository, or does not match the intended pull request and commit SHA
- **THEN** publication stops before obtaining or using repository-write authority

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
