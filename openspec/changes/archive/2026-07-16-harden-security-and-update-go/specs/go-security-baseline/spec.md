## ADDED Requirements

### Requirement: Maintained builds use the latest stable Go patch release
The project SHALL use the latest stable Go patch release available at implementation time across module toolchain metadata, active CI workflows, maintained builder images, and contributor instructions. The baseline identified for this change is Go 1.26.5.

#### Scenario: Implementation baseline is selected
- **WHEN** implementation begins
- **THEN** the official Go release source is queried and the newest stable non-prerelease version is recorded as the required toolchain

#### Scenario: Maintained Go pins are checked
- **WHEN** version-consistency validation inspects active build and CI surfaces
- **THEN** every maintained exact toolchain pin matches the selected stable Go patch release

#### Scenario: Historical fixture contains an older version
- **WHEN** an older Go version is intentionally present only as test input and cannot control an actual project build
- **THEN** the fixture may retain that value with its purpose covered by its test

### Requirement: Go version drift fails validation
The project SHALL provide an automated validation that detects mismatched maintained Go versions before changes merge.

#### Scenario: A CI workflow uses an older Go release
- **WHEN** a maintained workflow or builder definition selects a Go version different from the project baseline
- **THEN** validation fails and identifies the divergent file and version

#### Scenario: All maintained pins agree
- **WHEN** module metadata, workflows, builder images, and documented build prerequisites use the selected baseline
- **THEN** version-consistency validation succeeds

### Requirement: Runtime dependencies avoid known vulnerable ranges
The selected Go module graph SHALL use versions outside the vulnerable ranges associated with the project's open GitHub security advisories, including patched versions of `golang.org/x/crypto` and `golang.org/x/net`.

#### Scenario: Vulnerable extended module versions are upgraded
- **WHEN** the module graph is resolved after the security update
- **THEN** `golang.org/x/crypto` is at least v0.52.0 and `golang.org/x/net` is at least v0.55.0, or each is on a later compatible patched release

#### Scenario: Dependency security validation runs
- **WHEN** `govulncheck` and repository security scanning evaluate the updated production module graph
- **THEN** no unresolved current alert remains for the replaced vulnerable versions

### Requirement: Dependency upgrades preserve Git and SSH behavior
Security dependency updates SHALL preserve supported private-key parsing, SSH configuration resolution, proxy behavior, and Git-over-SSH repository access except for the intentional introduction of host-key verification.

#### Scenario: Protected private key is used
- **WHEN** a user accesses a Git source with a supported passphrase-protected SSH private key
- **THEN** key parsing and authentication continue to work under the updated cryptography module

#### Scenario: Git SSH integration is tested
- **WHEN** the coordinated module update is validated
- **THEN** targeted transport tests and the full project lint, test, and build suites pass using the selected Go baseline

### Requirement: Module metadata remains reproducible
Go dependency updates SHALL produce consistent `go.mod` and `go.sum` metadata under the selected toolchain and SHALL include review of resulting direct and transitive module changes.

#### Scenario: Module files are regenerated
- **WHEN** the dependency upgrade and module tidy operations complete
- **THEN** a clean rerun produces no additional module-file changes and the dependency diff is available for review