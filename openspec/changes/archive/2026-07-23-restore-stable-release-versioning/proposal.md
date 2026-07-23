## Why

The GitVersion upgrade from 5.x to 6.x changed the default `main`-branch `SemVer` output from a stable three-component release version to a numeric pre-release version (for example, `0.11.5-12`). Release workflows use that value for binary metadata, Git tags, GitHub Releases, and container tags, so validated main-line releases now have an unintended format and SemVer precedence.

## What Changes

- Add an explicit GitVersion configuration that defines the repository's intended main-branch release-version policy instead of relying on GitVersion defaults.
- Restore stable `{major}.{minor}.{patch}` versions for production releases created from validated `main` commits.
- Keep the existing workflow-owned `v` prefix for binary display values and Git release tags while ensuring GitVersion's `SemVer` output itself remains unprefixed and stable.
- Apply the same GitVersion policy to release, container publication, and debug-build workflows.
- Add regression coverage or validation for the version outputs used by release artifacts and tags.

## Capabilities

### New Capabilities
- `stable-release-versioning`: Produces stable, predictable semantic versions for production releases from validated main-line commits.

### Modified Capabilities

- None.

## Impact

- GitVersion configuration added at the repository root.
- `.github/workflows/release.yml`, `.github/workflows/release_container.yml`, and `.github/workflows/debug-build.yml` may be updated to consistently consume the configured output.
- Release Git tags, GitHub Release names, embedded CLI version strings, and GHCR image tags return to stable three-component semantic versions.
- The existing published pre-release-formatted tag remains immutable unless separately approved for remediation.
