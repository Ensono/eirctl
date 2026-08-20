## MODIFIED Requirements

### Requirement: Untrusted code executes without privileged credentials
Any workflow job that checks out or executes pull-request-controlled code SHALL use a trusted workflow definition and an isolated GitHub-hosted builder with no more than read-only repository permissions, and SHALL NOT receive release credentials, protected environment secrets, package-write permission, contents-write permission, or default-branch cache-write authority. A trusted default-branch event handler MAY authorize an untrusted build and invoke a reusable builder in the same run only when GitHub documents the root event's default-branch cache access as read-only, the reusable job inherits that event, and no caller path can elevate the called job's permissions, secrets, environment, runner persistence, or cache authority. The authorization job SHALL NOT check out or execute the pull-request revision itself.

#### Scenario: Pull request build runs
- **WHEN** a workflow builds or tests code from a pull request
- **THEN** the execution job uses a GitHub-hosted runner, has no more than `contents: read` repository access, receives no protected secret or environment, and cannot write the default-branch cache scope

#### Scenario: Issue comment requests a debug build
- **WHEN** a maintainer with `write`, `maintain`, or `admin` permission comments exactly `/build-debug` on a pull request
- **THEN** trusted default-branch code validates the command, actor permission, base repository, pull-request number, and current full head SHA before calling the reusable builder, without checking out or executing pull-request content in the authorization job

#### Scenario: Reusable debug build runs
- **WHEN** the authorized `issue_comment` run calls the debug builder
- **THEN** the builder inherits the caller event's documented read-only cache authority, revalidates the pull request and immutable head SHA immediately before checkout, executes with read-only repository authority and no secrets, and records the repository, event, pull request, commit SHA, run identity, and run attempt through a separate trusted finalization job

#### Scenario: Pull request changes before reusable checkout
- **WHEN** the current pull-request head no longer equals the full SHA captured by authorization
- **THEN** the reusable builder fails before checkout and does not silently build the newer revision

#### Scenario: Broker emits a suppressed event
- **WHEN** static validation finds that the broker relies on a `GITHUB_TOKEN`-generated event that GitHub suppresses from starting workflows
- **THEN** validation fails before the broken signaling topology can merge

#### Scenario: Writable event calls the reusable builder
- **WHEN** static validation finds a `workflow_dispatch`, `repository_dispatch`, default-branch `push`, or other documented default-branch-cache-write event that can call the reusable builder
- **THEN** validation fails even when repository permissions are read-only and convenience caching is disabled

### Requirement: Debug publication is isolated from untrusted execution
The system SHALL publish a debug prerelease only from a separate trusted default-branch job or workflow that does not check out or execute pull-request-controlled code and that uses the existing protected `debug-release` environment before obtaining repository-write authority. Publication SHALL authenticate the successful `issue_comment` request run, its reusable builder result, its clean-runner provenance finalization, and the exact final artifact before write authority is used.

#### Scenario: Maintainer publishes a successful debug build
- **WHEN** an authorized maintainer selects a successful request run whose workflow path, `issue_comment` event, repository, pull request, commit SHA, run attempt, final artifact identity, and trusted provenance pass validation
- **THEN** the trusted default-branch publication flow downloads the final artifact as opaque data and publishes only the expected binaries with the permissions required to create the prerelease

#### Scenario: Publication metadata does not match
- **WHEN** the selected request run failed, came from another workflow or repository, did not use `issue_comment`, or does not match the intended pull request, current commit SHA, run attempt, final artifact, or trusted provenance
- **THEN** publication stops before obtaining or using repository-write authority

#### Scenario: Publisher is dispatched from another ref
- **WHEN** a debug publication request runs from a branch or tag other than the protected default branch
- **THEN** both validation and publication jobs fail closed without obtaining repository-write authority

#### Scenario: Artifact is finalized
- **WHEN** the reusable builder finishes successfully
- **THEN** a fresh trusted runner revalidates immutable identity, handles the intermediate binaries only as bounded opaque files, creates provenance from trusted authorization and run metadata, and uploads the uniquely named final artifact without executing pull-request content

#### Scenario: Artifact is published
- **WHEN** the trusted publication flow handles an artifact produced from untrusted code
- **THEN** it does not execute, source, install, add to the command environment, or otherwise interpret that artifact or any code from the pull-request checkout

### Requirement: Workflow policy rejects privileged untrusted execution
The repository's workflow security policy SHALL parse workflow YAML structurally; SHALL independently model protected-default-branch workflow selection, repository and secret privilege, and default-branch cache-write authority; SHALL resolve local reusable-workflow callers so called jobs inherit the effective root event; SHALL reject every path that executes pull-request-controlled source with privileged credentials, protected secrets or environments, persistent runner state, or default-branch cache-write authority; SHALL permit passive pull-request source analysis only when the workflow matches the explicitly constrained trusted SonarCloud topology; SHALL require that topology to materialize a bounded allowlist of regular Go source blobs through GitHub's API from a provenance-verified head repository and full commit SHA; and SHALL validate the separated debug authorization, reusable builder, provenance finalizer, publisher, and analyzer trust domains independent of YAML formatting.

#### Scenario: Cache-write-capable workflow executes pull-request source
- **WHEN** structural validation finds a `workflow_dispatch`, `repository_dispatch`, default-branch `push`, `delete`, `registry_package`, `page_build`, `schedule`, or another event documented by GitHub as default-branch-cache-write-capable that passes a pull-request-controlled repository, ref, SHA, or derived value to checkout or download and later executes the resulting content
- **THEN** validation fails with a cache-poisoning trust-boundary error even when the ref is immutable, credentials are disabled, repository permissions are read-only, and automatic dependency caching is disabled

#### Scenario: Privileged workflow checks out pull-request source
- **WHEN** structural validation finds a job with protected credentials, a protected environment, write authority, OIDC authority, or persistent runner state that passes a pull-request-controlled repository, ref, SHA, or derived value to `actions/checkout`, `git checkout`, `git fetch`, `gh pr checkout`, or an equivalent checkout mechanism
- **THEN** validation fails with a trust-boundary error even when only a scanner or pinned action follows

#### Scenario: Read-only issue-comment caller invokes the constrained builder
- **WHEN** the exact authorized `issue_comment` topology calls the local reusable debug builder and GitHub's documented cache policy classifies the effective event as read-only for the default-branch cache
- **THEN** structural validation accepts the call only if authorization performs no checkout, the builder has no write permission, secret, protected environment, secret inheritance, self-hosted runner, or alternate writable caller, and immutable PR identity is revalidated before execution

#### Scenario: Reusable builder has a writable caller
- **WHEN** any effective caller of the reusable debug builder can write the default-branch cache or elevate repository, secret, environment, OIDC, or runner authority
- **THEN** structural validation rejects the topology regardless of the reusable workflow's own declared trigger and permissions

#### Scenario: External action consumes checked-out code
- **WHEN** a cache-write-capable or otherwise privileged workflow passes a pull-request-controlled checkout to an action that builds, interprets, packages, scans, or otherwise consumes repository content
- **THEN** validation classifies the action as potential code execution and rejects the workflow even when the action itself is pinned

#### Scenario: Equivalent YAML syntax is used
- **WHEN** a trigger, reusable call, security-sensitive permission, secret, runner, checkout, or execution step is expressed with flow syntax, quoting, aliases, expressions, or different valid indentation
- **THEN** structural validation applies the same caller-resolution, cache-authority, and trust-boundary rules

#### Scenario: Debug build topology is valid
- **WHEN** static workflow validation inspects the debug-build flow
- **THEN** it confirms that trusted authorization performs no checkout, the only builder entry point is a local `workflow_call` from the read-only `issue_comment` run, the builder revalidates and checks out the immutable pull-request SHA on a GitHub-hosted runner with read-only isolation, provenance is finalized on a fresh non-executing runner, and the publisher runs only from the protected default branch without pull-request checkout or execution

#### Scenario: Suppressed label topology is introduced
- **WHEN** the request broker uses `GITHUB_TOKEN` to add or re-add a label and depends on the resulting `pull_request:labeled` event to start the builder
- **THEN** structural validation fails with an unsupported signaling error

#### Scenario: Trusted passive SonarCloud topology is valid
- **WHEN** a default-branch `workflow_run` analyzer validates the exact pull-request revision and upstream artifact provenance, grants no write permission, uses no cache or pull-request command, obtains only bounded regular Go blobs through GitHub's API, supplies `SONAR_TOKEN` only to the approved pinned scanner step, and allows only that scanner to parse isolated source and report data under forced trusted settings
- **THEN** structural validation accepts the analyzer as the narrowly constrained passive-analysis topology

#### Scenario: Trusted source materialization remains passive
- **WHEN** the trusted analyzer materializes pull-request source
- **THEN** protected base-branch code resolves the verified head repository and full SHA through GitHub's API, requires a complete non-truncated Git tree, writes only allowlisted regular `.go` blobs as non-executable files under `analysis/source`, creates trusted scanner configuration outside that directory before materialization, and invokes no command, cache, local action, container, package manager, binary, checkout action, or external action other than the approved scanner after materialization

#### Scenario: Source tree contains a forbidden entry
- **WHEN** the verified tree or a requested blob contains a symlink, submodule, special or non-blob entry, absolute or traversal path, backslash path, duplicate normalized path, excessive path length, excessive file count, excessive per-file size, excessive aggregate size, or content whose blob identity does not match the requested tree entry
- **THEN** source materialization fails closed before `SONAR_TOKEN` is exposed

#### Scenario: SonarCloud topology broadens the exception
- **WHEN** the trusted analyzer checks out pull-request source; materializes non-Go content, Git metadata, workflows, scripts, local actions, scanner configuration, dependency hooks, containers, or binaries; executes a pull-request command; uses an unapproved scanner; restores or saves a cache; exposes the secret outside the scanner step; omits required provenance or source bounds; or permits pull-request scanner settings to control the endpoint or project identity
- **THEN** structural validation fails closed with a trust-boundary error

#### Scenario: Code scanning reports a privileged checkout vulnerability
- **WHEN** CodeQL or the repository's configured code-scanning tool reports a new cache-poisoning, untrusted-checkout, or equivalent high-severity workflow alert for the debug-build or analyzer topology
- **THEN** the implementation is not accepted, and the alert SHALL be resolved by design rather than dismissed, suppressed, or bypassed
