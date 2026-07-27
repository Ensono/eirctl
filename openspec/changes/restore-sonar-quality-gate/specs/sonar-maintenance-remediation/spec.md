## ADDED Requirements

### Requirement: Sonar findings are remediated from authenticated evidence
The maintenance process SHALL identify each in-scope SonarCloud finding by immutable issue key, expected rule, current component, severity, assignment, and gate relevance before source changes begin, SHALL record the token-free snapshot and subsequent disposition in `evidence.md`, and SHALL treat the current authenticated API response as authoritative when it differs from an earlier report.

#### Scenario: Scoped issue matches the recorded preconditions
- **WHEN** an implementation task starts for a named Sonar issue
- **THEN** the implementer confirms that the issue is open under the expected rule and component and inspects the containing code, callers, and focused tests before editing

#### Scenario: Scoped issue has drifted
- **WHEN** an issue is closed, moved into materially different behavior, no longer matches its recorded rule, overlaps unrelated changes, or requires an excluded semantic or API change
- **THEN** the implementation stops for scope review and does not substitute a different issue or improvise a broader fix

### Requirement: Each remediation is bounded and behavior-preserving
Each issue remediation SHALL make the smallest coherent change that removes the reported structural problem while preserving public APIs, observable behavior, error semantics, ordering, precedence, security predicates, test assertions, and generated or serialized shapes unless this change explicitly specifies otherwise.

#### Scenario: Mechanical remediation is sufficient
- **WHEN** a finding can be resolved through a local constant, named private type, explanatory no-op comment, cancellation placement, private helper extraction, or assertion-preserving test decomposition
- **THEN** the implementation changes only that concern and the minimum directly related tests

#### Scenario: Clean remediation requires excluded behavior changes
- **WHEN** resolving a finding would require a public API break, altered import or configuration semantics, weakened security validation, changed concurrency behavior, reduced test coverage, or a new dependency
- **THEN** the issue remains deferred and the implementation records why it cannot be resolved safely within this change

### Requirement: Smaller-model tasks contain executable safety gates
Every issue-level task SHALL state named preconditions, permitted files or concern, required invariants, focused validation commands, postconditions, and a stop-and-escalate condition so that a smaller coding model can execute it without inferring broader authority.

#### Scenario: Smaller model completes an issue task
- **WHEN** all recorded preconditions and invariants match the repository
- **THEN** the model applies the bounded fix, formats changed files, runs the named checks, inspects the diff for scope and assertion loss, and records command outcomes and residual risk

#### Scenario: Smaller model cannot prove a postcondition
- **WHEN** a required check fails, cannot run, or does not demonstrate the task's stated postcondition
- **THEN** the task remains incomplete and the model reports the blocker without weakening tests, validation, or security controls

### Requirement: Security-sensitive refactors preserve trust behavior
Refactors in Git SSH configuration and trusted workflow-policy validation SHALL preserve all existing trust boundaries, precedence rules, fail-closed behavior, matching semantics, and error propagation, and SHALL receive focused security regression testing and independent review.

#### Scenario: Git SSH configuration is decomposed
- **WHEN** `processSSHConfig` is split into focused helpers
- **THEN** explicit command values still take precedence over file values, documented defaults and repeated known-host entries are preserved, and strict host-key verification remains fail-closed except for the existing explicit opt-out

#### Scenario: Command selects a global known-host file
- **WHEN** `GIT_SSH_COMMAND` supplies OpenSSH `GlobalKnownHostsFile` through a supported `-o` form
- **THEN** the selected global trust files retain command precedence, ordered repetition, quoting, and path boundaries instead of being discarded or replaced by file/default trust sources

#### Scenario: SSH configuration supplies complex known-host paths
- **WHEN** `UserKnownHostsFile` or `GlobalKnownHostsFile` contains quoted or escaped spaces, multiple paths on one directive, or repeated directives
- **THEN** each intended path is preserved exactly and in order before validation and host-key loading, including legacy first-entry compatibility fields

#### Scenario: Workflow policy is decomposed
- **WHEN** repository-topology validation is split into focused phases
- **THEN** every existing predicate executes in the same fail-fast order with unchanged action-prefix, immutable-pin, provenance, permission, checkout, and trusted/untrusted boundary semantics

### Requirement: Sonar closure is verified without policy bypass
Local checks SHALL NOT be treated as proof that a Sonar issue is closed; final acceptance SHALL use authenticated SonarCloud issue and measure data from a fresh analysis of the exact revision while preserving the existing quality gate, new-code definition, scanner configuration, exclusions, and coverage mapping.

#### Scenario: Local remediation checks pass
- **WHEN** focused and repository-wide tests pass after an issue change
- **THEN** the implementation records local structural postconditions but does not claim Sonar closure before authenticated analysis

#### Scenario: Fresh analysis closes the scoped findings
- **WHEN** SonarCloud analyzes the exact protected revision containing the remediations
- **THEN** `evidence.md` records the exact analyzed revision and gate measures, every mandatory Tier 0/Tier 1 key is closed or absent, every Tier 2 key is closed or absent or has an authenticated drift/defer disposition, no replacement in-scope Critical smell is introduced, and the configured quality gate reports passing without suppression, acceptance, reclassification, dismissal, exclusion, threshold reduction, or ruleset bypass

#### Scenario: Fresh analysis remains red
- **WHEN** the exact-revision analysis still fails the quality gate
- **THEN** implementation stops and uses authenticated issue and measure evidence to identify the qualifying condition instead of changing Sonar or branch-protection policy
