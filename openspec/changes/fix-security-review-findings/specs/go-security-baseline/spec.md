## ADDED Requirements

### Requirement: Maintained CI execution dependencies are immutable
Maintained CI and release paths SHALL select task-runner container images and downloaded build or security tools using reviewed immutable identifiers. Container images SHALL use digest-qualified references, and tools downloaded during a workflow SHALL use exact versions with integrity verification when the upstream mechanism supports it.

#### Scenario: Release build starts a task-runner container
- **WHEN** a release job builds binaries through an eirctl container context
- **THEN** the selected image reference includes a reviewed `sha256` digest and cannot be changed by retagging the human-readable version

#### Scenario: Maintained workflow installs GitVersion
- **WHEN** a workflow installs GitVersion to compute build or release metadata
- **THEN** it requests an exact reviewed GitVersion release rather than a floating major or minor range

#### Scenario: Vulnerability validation installs govulncheck
- **WHEN** CI installs `govulncheck`
- **THEN** it installs a reviewed exact module version rather than `@latest`

### Requirement: Immutable execution pins are validated automatically
The project SHALL provide automated validation that rejects mutable task-runner image references and floating maintained CI-tool selectors before changes merge.

#### Scenario: Task-runner image uses only a tag
- **WHEN** validation finds a maintained CI or release context image without a digest
- **THEN** validation fails and identifies the mutable image reference

#### Scenario: Maintained tool selector uses a range or latest
- **WHEN** validation finds `@latest`, a floating GitVersion range, or another non-exact maintained CI-tool selector
- **THEN** validation fails and identifies the file and selector

#### Scenario: All execution dependencies are immutable
- **WHEN** every maintained task-runner image and downloaded CI tool uses the required immutable selection
- **THEN** immutable-dependency validation succeeds
