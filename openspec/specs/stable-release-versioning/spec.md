# Purpose

Define stable semantic versioning and consistent version formatting for production releases and their published artifacts.

# Requirements

## Requirement: Stable semantic version for validated main releases
The release-version calculation for a validated commit on the `main` branch SHALL produce an unprefixed stable semantic version in `{major}.{minor}.{patch}` form. The calculated value SHALL NOT include a pre-release suffix solely to represent commits after the version source.

### Scenario: Main release version is calculated after an existing release tag
- **WHEN** GitVersion calculates `SemVer` for a validated `main` commit after an existing version tag
- **THEN** the calculated `SemVer` matches `{major}.{minor}.{patch}` without a `-<number>` pre-release suffix

## Requirement: Release consumers apply formatting consistently
Production release consumers SHALL use the calculated stable semantic version consistently. Binary display metadata and Git/GitHub release tags SHALL add exactly one leading `v`, while container image tags SHALL use the unprefixed semantic version.

### Scenario: Release workflow creates production artifacts
- **WHEN** the release and container-publication workflows process a validated `main` version
- **THEN** the binary display version and Git release tag are `v{major}.{minor}.{patch}` and the container image tag is `{major}.{minor}.{patch}`

## Requirement: Versioning policy is repository-controlled
The production version format SHALL be defined by repository-owned GitVersion configuration rather than depending solely on GitVersion's built-in branch defaults.

### Scenario: GitVersion tool defaults change
- **WHEN** a future GitVersion action or tool update changes its built-in main-branch defaults
- **THEN** the repository-owned configuration continues to require stable semantic versions for validated `main` releases

## Requirement: Release-version regression validation
The repository SHALL validate the production release-version contract before release publication, including stable main-line `SemVer` output and correct prefix placement for each artifact type.

### Scenario: A workflow or configuration change reintroduces a numeric pre-release suffix
- **WHEN** version-contract validation evaluates a production release path whose calculated main-line version contains a numeric pre-release suffix
- **THEN** validation fails before the change can be approved for release publication
