## Context

The release, container-publication, and debug-build workflows invoke GitVersion and consume its `semVer` output. Before the GitVersion 6 upgrade, production releases were emitted as stable three-component versions. GitVersion 6 changed the built-in `main`-branch behavior so its default empty label produces a numeric pre-release suffix (for example, `0.11.5-12`).

The repository has no `GitVersion.yml`, which leaves release identity dependent on a third-party default configuration. Production workflows use the calculated value directly for annotated Git tags, GitHub Releases, container image tags, and embedded CLI version metadata. The workflow—not GitVersion—owns the `v` prefix for the binary display string and Git tag.

## Goals / Non-Goals

**Goals:**
- Make the intended GitVersion behavior explicit and version-controlled.
- Produce an unprefixed stable `{major}.{minor}.{patch}` `SemVer` value for validated `main` releases.
- Preserve the existing release presentation convention: `v` is added only where the workflow requires it.
- Keep release, container, and debug-build calculations aligned with the same GitVersion configuration.
- Validate the version contract before publishing release artifacts.

**Non-Goals:**
- Retag, delete, or republish the existing `v0.11.5-12` release.
- Redesign the repository's release cadence, release approval gate, or `workflow_run` trust boundary.
- Change the GitVersion tool/action versions as part of this correction.
- Alter the public CLI version output beyond restoring its stable release component.

## Decisions

### Add an explicit GitVersion configuration

Add a root `GitVersion.yml` that selects the intended built-in workflow and overrides the root and `main` branch labels with YAML `null`, so GitVersion 6 emits stable releases on `main` rather than an empty-label numeric pre-release.

The configuration will retain recognition of the existing `v`-prefixed tag convention and the current patch-increment behavior unless verification demonstrates an existing explicit release policy that requires a different increment.

**Rationale:** The GitVersion 6 change is an upstream default change. An explicit repository configuration turns the production version shape into a source-controlled contract and isolates it from future default changes.

**Alternatives considered:**
- Pin GitVersion back to 5.x: rejected because it abandons the security-driven upgrade and leaves a floating dependency.
- Post-process `semVer` in each workflow: rejected because it can turn a pre-release identity into a different stable identity after GitVersion has calculated it, duplicates logic, and leaves debug/release paths prone to divergence.
- Accept `-{number}` tags as the new release convention: rejected because production main-line releases are meant to be stable and pre-release SemVer ordering is externally observable.

### Preserve ownership of the `v` prefix in workflows

Keep GitVersion's `SemVer` unprefixed. Continue adding `v` only when building the CLI display version and creating Git tags/GitHub Releases; container tags remain unprefixed.

**Rationale:** This matches existing consumers and avoids double-prefixing or storing presentation formatting in the semantic version value.

### Add version-contract validation

Add focused automated validation for the release-version configuration and the workflow consumers. The checks will cover stable `main` output, correct placement of the `v` prefix, and prevention of a numeric pre-release suffix in production-release paths.

**Rationale:** A GitVersion configuration change is easy to regress through a dependency or workflow edit; static and/or fixture-based validation makes the release contract reviewable without publishing an artifact.

## Risks / Trade-offs

- [An existing tag calculation behaves differently once `main` is stable] → Validate the configuration against the current tag graph before merge and retain published tags unchanged.
- [GitVersion configuration semantics distinguish `null` from an empty string] → Use an actual YAML null and test the effective output with the pinned GitVersion version.
- [Debug builds need unique artifact identities] → Preserve debug provenance and artifact naming independently of the production release `SemVer`; verify the configured output remains appropriate for the debug workflow.
- [A workflow consumes a value with an unexpected prefix] → Test release tags, binary build arguments, and GHCR tags separately because they intentionally have different formatting requirements.

## Migration Plan

1. Add the explicit GitVersion configuration and version-contract checks.
2. Run the pinned GitVersion calculation against the current repository history and validate CI/workflow tests without publishing.
3. Merge through the existing protected `main` process.
4. Observe the next approved release: GitVersion emits unprefixed stable `SemVer`, release artifacts use the expected `v` prefix only where required, and container tags remain unprefixed.
5. Roll back by reverting the configuration and associated validation changes if release calculation fails before publication. Do not mutate existing published tags as part of rollback.

## Open Questions

- Should debug builds retain a pre-release-formatted version for artifact distinction, or is provenance metadata alone sufficient once the common configuration is applied? The implementation must verify the expected debug-build behavior before enforcing its final assertion.
- Does the repository want an explicit release trigger in the future rather than publishing every validated `main` commit? This is out of scope for this change.
