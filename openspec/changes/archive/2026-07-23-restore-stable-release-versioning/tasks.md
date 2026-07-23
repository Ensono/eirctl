## 1. Define the repository-owned version policy

- [x] 1.1 Add `GitVersion.yml` selecting the intended workflow and configure root and `main` labels as YAML `null` so validated main-line versions are stable, unprefixed `{major}.{minor}.{patch}` values.
- [x] 1.2 Verify the pinned GitVersion 6.2.0 calculation against the current repository tag graph, including the existing `v0.11.5-12` tag, and record the expected next main-line version without modifying published tags.

## 2. Align release consumers

- [x] 2.1 Review and update `.github/workflows/release.yml` so its binary build argument, annotated Git tag, and GitHub Release tag consume the configured stable `SemVer` and add exactly one workflow-owned `v` prefix where required.
- [x] 2.2 Review and update `.github/workflows/release_container.yml` so production container tags consume the same configured stable, unprefixed `SemVer`.
- [x] 2.3 Review and update `.github/workflows/debug-build.yml` to use the shared GitVersion configuration while preserving appropriate debug artifact provenance and identity.

## 3. Validate the release-version contract

- [x] 3.1 Add focused tests or policy validation that rejects production-release version paths producing a numeric pre-release suffix and confirms a stable main-line `SemVer` result.
- [x] 3.2 Add validation for artifact-specific prefix placement: one `v` for binary/Git/GitHub Release values and no `v` for container tags.
- [x] 3.3 Run the relevant formatting, workflow-policy, and test commands; verify the configured GitVersion output with the pinned version before approving publication.
