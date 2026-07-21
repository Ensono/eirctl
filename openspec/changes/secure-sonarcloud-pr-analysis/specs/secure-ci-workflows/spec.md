## MODIFIED Requirements

### Requirement: Workflow policy rejects privileged untrusted execution
The repository's workflow security policy SHALL parse workflow YAML structurally, SHALL reject workflows that combine a privileged or default-branch trigger with checkout and execution of pull-request-controlled code, SHALL permit passive pull-request source analysis only when the workflow matches the explicitly constrained trusted SonarCloud topology, SHALL permit that topology to materialize only a provenance-verified full SHA through an isolated `persist-credentials: false` checkout, and SHALL validate the separated broker, builder, publisher, and analyzer trust domains independent of YAML formatting.

#### Scenario: Privileged workflow executes pull-request code
- **WHEN** structural validation finds an `issue_comment`, `pull_request_target`, `workflow_run`, `repository_dispatch`, or `workflow_dispatch` path that checks out a pull-request-controlled revision and executes content from that checkout
- **THEN** validation fails with a trust-boundary error

#### Scenario: Dynamic checkout ref is not proven safe
- **WHEN** a privileged workflow passes an input, event value, or step output to `actions/checkout` and the policy cannot prove that value resolves to trusted source
- **THEN** validation treats the ref as untrusted and rejects subsequent code execution

#### Scenario: External action consumes checked-out code
- **WHEN** a privileged workflow passes the checked-out workspace to an action that builds, interprets, packages, or otherwise executes repository content outside the explicitly constrained passive SonarCloud analysis topology
- **THEN** validation classifies the action as code execution even when the action itself is pinned

#### Scenario: Equivalent YAML syntax is used
- **WHEN** a privileged trigger or security-sensitive step is expressed with flow syntax, quoting, or different valid indentation
- **THEN** structural validation applies the same trust-boundary rule

#### Scenario: Debug build topology is valid
- **WHEN** static workflow validation inspects the debug-build flow
- **THEN** it confirms that the broker performs no checkout, the builder uses a supported dispatch with immutable pull-request identity and read-only isolation, and the publisher runs only from the protected default branch without pull-request checkout or execution

#### Scenario: Trusted passive SonarCloud topology is valid
- **WHEN** a default-branch `workflow_run` analyzer validates the exact pull-request revision and upstream artifact provenance, grants no write permission, uses no cache or pull-request command, supplies `SONAR_TOKEN` only to the approved pinned scanner step, and allows only that scanner to parse isolated source and report data under forced trusted settings
- **THEN** structural validation accepts the analyzer as the narrowly constrained passive-analysis exception

#### Scenario: Trusted checkout remains passive
- **WHEN** the trusted analyzer materializes a pull-request revision
- **THEN** it checks out only the provenance-verified full head SHA with `persist-credentials: false` into an isolated analysis directory, creates trusted scanner configuration outside that source directory before checkout, and invokes no command, cache, local action, container, package manager, binary, or external action other than the approved scanner after materialization

#### Scenario: SonarCloud topology broadens the exception
- **WHEN** the trusted analyzer executes a pull-request command, local action, dependency, container, or binary; uses an unapproved scanner; restores or saves a cache; exposes the secret outside the scanner step; omits required provenance checks; or permits pull-request scanner settings to control the endpoint or project identity
- **THEN** structural validation fails closed with a trust-boundary error

## ADDED Requirements

### Requirement: SonarCloud analysis covers trusted main and every pull request
The CI system SHALL submit SonarCloud analysis for every trusted push to `main` and every pull-request revision targeting `main`, including revisions originating from forks, and SHALL wait for the configured SonarCloud quality gate without exposing protected credentials to the untrusted build workflow.

#### Scenario: Trusted main push is analyzed
- **WHEN** tests succeed for a trusted push to `main`
- **THEN** the workflow generates the configured Go reports, runs the approved SonarCloud scanner successfully, and waits for the quality-gate result

#### Scenario: Same-repository pull request is analyzed
- **WHEN** the untrusted pull-request workflow completes for a branch in `Ensono/eirctl`
- **THEN** the default-branch analyzer submits analysis for the exact pull-request head SHA and SonarCloud decorates that pull request

#### Scenario: Fork pull request is analyzed
- **WHEN** the untrusted pull-request workflow completes for a fork-originated revision targeting `main`
- **THEN** the default-branch analyzer submits analysis for the exact fork revision without passing `SONAR_TOKEN` to the fork workflow or executing fork-controlled content

#### Scenario: Pull-request tests do not produce coverage
- **WHEN** the upstream pull-request run completes without the expected coverage report
- **THEN** the analyzer produces the explicitly configured source-only or failed-preparation outcome and does not silently skip SonarCloud reporting

### Requirement: Trusted SonarCloud analysis validates immutable provenance
The trusted analyzer SHALL verify the upstream workflow identity, event, repository, pull request, base branch, immutable head SHA, run ID, run attempt, artifact identity, and bounded artifact contents before any step receives `SONAR_TOKEN`.

#### Scenario: Provenance matches
- **WHEN** the expected `Lint and Test` pull-request run and its report artifact resolve to the same verified pull request, run attempt, and full head SHA
- **THEN** the analyzer materializes those inputs in an isolated analysis directory and proceeds to the scanner step

#### Scenario: Pull request revision is superseded
- **WHEN** a newer revision of the same pull request starts analysis
- **THEN** per-pull-request concurrency prevents the stale revision from being reported as the current result and never mixes artifacts between revisions

#### Scenario: Provenance does not match
- **WHEN** any repository, event, workflow, pull request, base branch, SHA, run-attempt, artifact, path, file-type, or size validation fails
- **THEN** the analyzer fails closed before exposing `SONAR_TOKEN`

#### Scenario: Artifact contains unexpected content
- **WHEN** the report artifact contains an unexpected path, file, symlink, special file, or content beyond the configured bound
- **THEN** the analyzer rejects the artifact and does not invoke the scanner

### Requirement: SonarCloud credentials and configuration remain trusted
The trusted analyzer SHALL scope `SONAR_TOKEN` to the immutable-SHA-pinned official scanner step and SHALL force trusted endpoint, organization, project, report-path, pull-request, revision, and quality-gate settings from a trusted configuration root outside the pull-request source directory at a precedence that pull-request-controlled configuration cannot override.

#### Scenario: Scanner receives protected credentials
- **WHEN** all provenance and passive-input validation succeeds
- **THEN** only the approved Sonar scanner step receives `SONAR_TOKEN`, while preceding materialization and validation steps do not

#### Scenario: Pull request changes Sonar configuration
- **WHEN** the analyzed revision adds or modifies `sonar-project.properties` or equivalent scanner settings
- **THEN** the trusted analyzer ignores or replaces those settings and still uses `https://sonarcloud.io`, organization `ensono`, project `Ensono_eirctl`, and the verified pull-request metadata

#### Scenario: Untrusted source attempts execution
- **WHEN** analyzed source includes workflow files, scripts, local actions, dependency hooks, container definitions, or executable binaries
- **THEN** the trusted analyzer treats them only as source data and never invokes them

#### Scenario: Scanner dependency is selected
- **WHEN** implementation introduces or updates the SonarCloud scan action or supporting GitHub Actions
- **THEN** each action is on the repository allow list, is the latest stable release resolved at implementation time, is pinned to its full commit SHA with a readable version comment, and passes immutable-dependency validation

### Requirement: Security-sensitive CI configuration requires code-owner review
The repository SHALL assign executable workflows, the ownership policy itself, and the root SonarCloud project configuration to `@Ensono/digital-tools-maintainers`, and the active `main` ruleset SHALL require approval from a matching code owner before such changes can merge.

#### Scenario: Workflow changes
- **WHEN** a pull request changes a file under `/.github/workflows/**`
- **THEN** GitHub requests review from `@Ensono/digital-tools-maintainers` and the ruleset blocks merge until an eligible code owner approves

#### Scenario: Sonar project configuration changes
- **WHEN** a pull request changes `/sonar-project.properties`
- **THEN** GitHub requests review from `@Ensono/digital-tools-maintainers` and the ruleset blocks merge until an eligible code owner approves

#### Scenario: Ownership policy changes
- **WHEN** a pull request changes `/.github/CODEOWNERS`
- **THEN** the ownership file's own rule requires `@Ensono/digital-tools-maintainers` approval before merge

#### Scenario: Code-owner review is not runtime authorization
- **WHEN** an unapproved pull request triggers CI before review
- **THEN** workflow isolation, least privilege, provenance validation, and secret scoping remain fully enforced independently of CODEOWNERS

### Requirement: SonarCloud quality gate protects the main branch
The active `main` ruleset SHALL require the stable external SonarCloud quality-gate check associated with the pull-request head revision in addition to the existing lint and Linux test checks.

#### Scenario: Quality gate passes
- **WHEN** SonarCloud reports a passing quality gate for the current pull-request head SHA
- **THEN** the SonarCloud required-check condition is satisfied

#### Scenario: Quality gate fails or is missing
- **WHEN** SonarCloud reports a failing quality gate or no completed result exists for the current pull-request head SHA
- **THEN** the `main` ruleset blocks merge

#### Scenario: Required check is configured
- **WHEN** the first live trusted PR analysis establishes the exact external check context and integration ID
- **THEN** repository maintainers add that observed identity to `main-is-main` without substituting the default-branch `workflow_run` job context
