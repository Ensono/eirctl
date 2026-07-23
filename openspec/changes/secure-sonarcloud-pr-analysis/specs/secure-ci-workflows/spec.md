## MODIFIED Requirements

### Requirement: Workflow policy rejects privileged untrusted execution
The repository's workflow security policy SHALL parse workflow YAML structurally, SHALL reject every privileged or default-branch workflow that checks out pull-request-controlled source, SHALL permit passive pull-request source analysis only when the workflow matches the explicitly constrained trusted SonarCloud topology, SHALL require that topology to materialize a bounded allowlist of regular Go source blobs through GitHub's API from a provenance-verified head repository and full commit SHA, and SHALL validate the separated broker, builder, publisher, and analyzer trust domains independent of YAML formatting.

#### Scenario: Privileged workflow checks out pull-request source
- **WHEN** structural validation finds an `issue_comment`, `pull_request_target`, `workflow_run`, `repository_dispatch`, or `workflow_dispatch` path that passes a pull-request-controlled repository, ref, SHA, or derived value to `actions/checkout`, `git checkout`, `git fetch`, `gh pr checkout`, or an equivalent checkout mechanism
- **THEN** validation fails with a trust-boundary error even when the ref is immutable, credentials are disabled, and only a scanner follows

#### Scenario: External action consumes checked-out code
- **WHEN** a privileged workflow passes a pull-request-controlled checkout to an action that builds, interprets, packages, scans, or otherwise consumes repository content
- **THEN** validation classifies the action as potential code execution and rejects the workflow even when the action itself is pinned

#### Scenario: Equivalent YAML syntax is used
- **WHEN** a privileged trigger or security-sensitive step is expressed with flow syntax, quoting, aliases, expressions, or different valid indentation
- **THEN** structural validation applies the same trust-boundary rule

#### Scenario: Debug build topology is valid
- **WHEN** static workflow validation inspects the debug-build flow
- **THEN** it confirms that the broker performs no checkout, the builder uses a supported dispatch with immutable pull-request identity and read-only isolation, and the publisher runs only from the protected default branch without pull-request checkout or execution

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
- **WHEN** CodeQL or the repository's configured code-scanning tool reports a new untrusted-checkout or equivalent high-severity workflow alert for the analyzer
- **THEN** the implementation is not accepted, and the alert SHALL be resolved by design rather than dismissed, suppressed, or bypassed

## ADDED Requirements

### Requirement: SonarCloud analysis covers trusted main and every pull request
The CI system SHALL submit SonarCloud analysis for every trusted push to `main` and every pull-request revision targeting `main`, including revisions originating from forks, and SHALL wait for the configured SonarCloud quality gate without exposing protected credentials to the untrusted build workflow.

#### Scenario: Trusted main push is analyzed
- **WHEN** tests succeed for a trusted push to `main`
- **THEN** the workflow loads settings for organization `ensono` and the bound `Ensono_eirctl` project whose main branch is `main`, generates the configured Go reports, runs the approved SonarCloud scanner successfully, and waits for the quality-gate result

#### Scenario: Same-repository pull request is analyzed
- **WHEN** the untrusted pull-request workflow completes for a branch in `Ensono/eirctl`
- **THEN** the default-branch analyzer submits analysis for the exact pull-request head SHA and SonarCloud decorates that pull request

#### Scenario: Fork pull request is analyzed
- **WHEN** the untrusted pull-request workflow completes for a fork-originated revision targeting `main`
- **THEN** the default-branch analyzer submits analysis for the exact fork revision without passing `SONAR_TOKEN` to the fork workflow or executing fork-controlled content

#### Scenario: Pull-request tests do not produce coverage
- **WHEN** the upstream pull-request run completes without the expected coverage report
- **THEN** the analyzer produces the explicitly configured source-only or failed-preparation outcome and does not silently skip SonarCloud reporting

### Requirement: SonarCloud project identity and analysis credential are operationally valid
Before live analysis is accepted, the `ensono` SonarQube Cloud organization SHALL contain exactly one canonical project for `Ensono/eirctl`, that project SHALL be bound to the GitHub repository with the fixed key `Ensono_eirctl` and main branch `main`, and the repository SHALL store a current, plan-supported, least-privilege analysis credential as the `SONAR_TOKEN` GitHub Actions secret.

#### Scenario: Fixed project identity is preserved
- **WHEN** implementation or operations configure SonarQube Cloud analysis for this repository
- **THEN** they use organization `ensono` and project key `Ensono_eirctl` without renaming the key, substituting an alternate project, or creating a blind duplicate

#### Scenario: Authorized project state is validated
- **WHEN** an authorized `ensono` administrator performs the pre-live validation
- **THEN** the administrator verifies through authenticated SonarQube Cloud access that `Ensono_eirctl` is bound to `Ensono/eirctl` and uses `main` as its main branch, repairing or provisioning that binding only under the same fixed identity if required

#### Scenario: SonarCloud administration access is pending
- **WHEN** the maintainer cannot inspect the authenticated project state, generate the supported credential, or replace the repository secret
- **THEN** live acceptance and required-check rollout remain blocked without changing the fixed project identity, exposing a token value, weakening the workflow trust boundary, or creating an unauthorized workaround

#### Scenario: Team plan supplies the analysis credential
- **WHEN** the `ensono` organization uses the Team plan or higher
- **THEN** operations generate a project-scoped Scoped Organization Token granting only **Execute analysis**, store its value as the repository `SONAR_TOKEN` secret, and record its owner and expiry outside the repository

#### Scenario: Free plan supplies the analysis credential
- **WHEN** the `ensono` organization uses the Free plan
- **THEN** operations generate a personal access token from a maintained identity with only the authorization required to analyze `Ensono_eirctl`, store its value as the repository `SONAR_TOKEN` secret, and record its owner and expiry outside the repository

#### Scenario: Project settings or authorization is invalid
- **WHEN** a trusted-main scan reports `NOT_FOUND`, a `NONEXISTENT` binding, an authorization failure, or inability to load settings for the exact canonical project
- **THEN** the scan fails visibly and same-repository PR, fork PR, and required-check rollout do not proceed until project settings load and analysis succeeds

#### Scenario: Replacement credential succeeds
- **WHEN** the rotated credential successfully loads canonical project settings and submits trusted-main analysis
- **THEN** operations revoke the superseded credential, or revoke it immediately without waiting if compromise is suspected

### Requirement: Trusted SonarCloud analysis validates immutable provenance
The trusted analyzer SHALL verify the upstream workflow identity, event, base repository, head repository, pull request, base branch, immutable head SHA, run ID, run attempt, report-artifact identity, bounded report contents, Git tree response, and selected source-blob identities before any step receives `SONAR_TOKEN`.

#### Scenario: Provenance matches
- **WHEN** the expected `Lint and Test` pull-request run, report artifact, verified head repository, full head SHA, complete Git tree, and selected source blobs resolve to the same current pull-request revision
- **THEN** the analyzer materializes the bounded passive inputs in an isolated analysis directory and proceeds to the scanner step

#### Scenario: Pull request revision is superseded
- **WHEN** a newer revision of the same pull request starts analysis
- **THEN** per-pull-request concurrency prevents the stale revision from being reported as the current result and never mixes artifacts or source blobs between revisions

#### Scenario: Provenance does not match
- **WHEN** any base or head repository, event, workflow, pull request, base branch, SHA, run-attempt, artifact, tree, blob, path, file-type, or size validation fails
- **THEN** the analyzer fails closed before exposing `SONAR_TOKEN`

#### Scenario: Artifact contains unexpected content
- **WHEN** the report artifact contains an unexpected path, file, symlink, special file, invalid UTF-8, malformed coverage mode or record, unsafe coverage path, or content beyond the configured bound
- **THEN** the analyzer rejects the artifact and does not invoke the scanner

#### Scenario: Coverage paths match isolated source
- **WHEN** a valid bounded Go coverage report names files relative to the verified repository root and the same Go files are materialized beneath `analysis/source`
- **THEN** protected pre-materialization code prefixes each canonical coverage record with the fixed `source/` namespace so the scanner imports coverage against the corresponding isolated source file

#### Scenario: Git tree cannot be proven complete
- **WHEN** GitHub returns a truncated tree, an unresolved commit, a changed head revision, a missing blob, or a blob whose identity or size differs from the verified tree entry
- **THEN** the analyzer rejects the source input and does not invoke the scanner

### Requirement: SonarCloud credentials and configuration remain trusted
The trusted analyzer SHALL scope `SONAR_TOKEN` to the immutable-SHA-pinned official scanner step and SHALL force trusted endpoint, organization, project, report-path, pull-request, revision, source-control, and quality-gate settings from a trusted configuration root outside the pull-request source directory at a precedence that pull-request-controlled configuration cannot override.

#### Scenario: Scanner receives protected credentials
- **WHEN** all provenance and passive-input validation succeeds
- **THEN** only the approved Sonar scanner step receives `SONAR_TOKEN`, while preceding provenance, API retrieval, validation, and materialization steps do not

#### Scenario: Credential authorization is validated without broader exposure
- **WHEN** operations validate the rotated credential against the canonical project
- **THEN** only the approved scanner step receives `SONAR_TOKEN`, no token-bearing preflight or diagnostic step is added, and no log exposes the credential value

#### Scenario: Pull request changes Sonar configuration
- **WHEN** the analyzed revision adds or modifies `sonar-project.properties` or equivalent scanner settings
- **THEN** that configuration is not materialized, and the trusted analyzer still uses `https://sonarcloud.io`, organization `ensono`, project `Ensono_eirctl`, and the verified pull-request metadata

#### Scenario: Untrusted source attempts execution
- **WHEN** the pull-request tree includes workflow files, scripts, local actions, dependency hooks, container definitions, scanner configuration, or executable binaries
- **THEN** those entries are not materialized, and the trusted analyzer never invokes them

#### Scenario: Scanner processes allowlisted source
- **WHEN** the scanner step starts
- **THEN** its project base contains trusted configuration, validated reports, and only non-executable regular Go source files obtained from the verified immutable revision, with no pull-request Git metadata or repository-local executable surface

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
