## Why

The current documentation build depends on a large shared image and hides the renderer behind a container-internal PowerShell module, while the proposed alternative currently leaves a second, conflicting build contract in the branch. A single composed pipeline with explicit commands and ignored output paths has already produced equivalent review artifacts while reducing the observed PR documentation job from 64 seconds to 41 seconds.

## What Changes

- **BREAKING**: Replace the `docs:build` shared-builder task and root `docs.json` contract with one canonical `build:docs` pipeline composed from explicit HTML, PDF, and rendered-output validation tasks.
- Use purpose-specific Asciidoctor and Node runtimes selected from authoritative registries and pinned to verified immutable digests; do not retain the large `ensono/eir-asciidoctor` context solely to invoke its PowerShell module.
- Make the renderer commands, source entry point, failure threshold, and `.eirctl/outputs/docs` destination explicit in repository-owned `eirctl` configuration, preserving failure on malformed source, missing includes, unresolved local references, and renderer errors.
- Keep the expected-output manifest, validator, and validator tests repository-owned because they enforce eirctl's local documentation contract rather than providing a reusable imported asset.
- Change the ESM validator entry point from `main().catch(...)` to top-level `await main().catch(...)`, preserving the existing diagnostic and non-zero exit behavior.
- Make local contributor guidance and PR CI invoke the same canonical pipeline and upload the same bounded `.eirctl/outputs/docs` artifacts.
- Remove the superseded context, task, contract file, output-path references, and other duplicate documentation-build wiring so there is only one supported path.
- Record runtime footprint and before/after CI timing evidence as part of implementation validation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `asciidoc-documentation-build`: Replace the shared `docs.json` builder contract with an inspectable, composed, resource-conscious `eirctl` pipeline and move generated output to `.eirctl/outputs/docs`.

## Impact

- Affected configuration and scripts: `eirctl.yaml`, `docs.json`, `scripts/docs-output-manifest.json`, `scripts/validate-docs-output.mjs`, and its tests.
- Evidence record: `openspec/changes/compose-asciidoc-documentation-build/evidence.md` will hold authoritative registry links, runtime architecture/digest/footprint data, CI run and job links, timings, and artifact observations.
- Affected automation: `.github/workflows/pr.yml` documentation invocation and artifact path.
- Affected guidance: `CONTRIBUTING.md`, `docs/building.adoc`, the PR description, and any active references to `docs:build` or `outputs/docs`.
- Dependency posture: the shared Ensono documentation image is removed; the direct Asciidoctor and Node runtime references become the documentation pipeline dependencies and remain digest-pinned.
- No product runtime, public API, documentation source, or published-site behavior changes are intended.
