# Purpose

TBD: Define the project's Go toolchain and dependency security baseline.

# Requirements

## Requirement: Maintained builds use the latest stable Go patch release
The project SHALL use the latest stable Go patch release available at implementation time across module toolchain metadata, active CI workflows, maintained builder images, and contributor instructions. The baseline identified for this change is Go 1.26.5.

### Scenario: Implementation baseline is selected
- **WHEN** implementation begins
- **THEN** the official Go release source is queried and the newest stable non-prerelease version is recorded as the required toolchain

### Scenario: Maintained Go pins are checked
- **WHEN** version-consistency validation inspects active build and CI surfaces
- **THEN** every maintained exact toolchain pin matches the selected stable Go patch release

### Scenario: Historical fixture contains an older version
- **WHEN** an older Go version is intentionally present only as test input and cannot control an actual project build
- **THEN** the fixture may retain that value with its purpose covered by its test

## Requirement: Go version drift fails validation
The project SHALL provide an automated validation that detects mismatched maintained Go versions before changes merge.

### Scenario: A CI workflow uses an older Go release
- **WHEN** a maintained workflow or builder definition selects a Go version different from the project baseline
- **THEN** validation fails and identifies the divergent file and version

### Scenario: All maintained pins agree
- **WHEN** module metadata, workflows, builder images, and documented build prerequisites use the selected baseline
- **THEN** version-consistency validation succeeds

## Requirement: Runtime dependencies avoid known vulnerable ranges
The selected Go module graph SHALL use versions outside the vulnerable ranges associated with the project's open GitHub security advisories, including patched versions of `golang.org/x/crypto` and `golang.org/x/net`.

### Scenario: Vulnerable extended module versions are upgraded
- **WHEN** the module graph is resolved after the security update
- **THEN** `golang.org/x/crypto` is at least v0.52.0 and `golang.org/x/net` is at least v0.55.0, or each is on a later compatible patched release

### Scenario: Dependency security validation runs
- **WHEN** `govulncheck` and repository security scanning evaluate the updated production module graph
- **THEN** no unresolved current alert remains for the replaced vulnerable versions

## Requirement: Dependency upgrades preserve Git and SSH behavior
Security dependency updates SHALL preserve supported private-key parsing, SSH configuration resolution, proxy behavior, and Git-over-SSH repository access except for the intentional introduction of host-key verification.

### Scenario: Protected private key is used
- **WHEN** a user accesses a Git source with a supported passphrase-protected SSH private key
- **THEN** key parsing and authentication continue to work under the updated cryptography module

### Scenario: Git SSH integration is tested
- **WHEN** the coordinated module update is validated
- **THEN** targeted transport tests and the full project lint, test, and build suites pass using the selected Go baseline

## Requirement: Module metadata remains reproducible
Go dependency updates SHALL produce consistent `go.mod` and `go.sum` metadata under the selected toolchain and SHALL include review of resulting direct and transitive module changes.

### Scenario: Module files are regenerated
- **WHEN** the dependency upgrade and module tidy operations complete
- **THEN** a clean rerun produces no additional module-file changes and the dependency diff is available for review

## Requirement: Maintained CI execution dependencies are immutable
Maintained CI and release paths SHALL select task-runner container images and downloaded build or security tools using reviewed immutable identifiers. Container images SHALL use digest-qualified references, and tools downloaded during a workflow SHALL use exact versions with integrity verification when the upstream mechanism supports it.

### Scenario: Release build starts a task-runner container
- **WHEN** a release job builds binaries through an eirctl container context
- **THEN** the selected image reference includes a reviewed `sha256` digest and cannot be changed by retagging the human-readable version

### Scenario: Maintained workflow installs GitVersion
- **WHEN** a workflow installs GitVersion to compute build or release metadata
- **THEN** it requests an exact reviewed GitVersion release rather than a floating major or minor range

### Scenario: Vulnerability validation installs govulncheck
- **WHEN** CI installs `govulncheck`
- **THEN** it installs a reviewed exact module version rather than `@latest`

## Requirement: Immutable execution pins are validated automatically
The project SHALL provide automated validation that rejects mutable task-runner image references and floating maintained CI-tool selectors before changes merge.

### Scenario: Task-runner image uses only a tag
- **WHEN** validation finds a maintained CI or release context image without a digest
- **THEN** validation fails and identifies the mutable image reference

### Scenario: Maintained tool selector uses a range or latest
- **WHEN** validation finds `@latest`, a floating GitVersion range, or another non-exact maintained CI-tool selector
- **THEN** validation fails and identifies the file and selector

### Scenario: All execution dependencies are immutable
- **WHEN** every maintained task-runner image and downloaded CI tool uses the required immutable selection
- **THEN** immutable-dependency validation succeeds
