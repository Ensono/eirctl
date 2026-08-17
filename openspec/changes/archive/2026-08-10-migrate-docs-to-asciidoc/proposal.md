## Why

The repository's user documentation is Markdown-only and has no reproducible documentation build or rendered-output validation. Migrating the substantive guides to the AsciiDoc pattern used by Ensono Stacks will make the documentation reusable in the shared toolchain and prevent source, include, link, and rendering regressions before merge.

## What Changes

- Convert the canonical guides in `docs/` from Markdown to AsciiDoc while retaining `README.md` as the concise GitHub-rendered landing page.
- Correct existing documentation defects as part of conversion: stale version examples, platform/product capitalization, typographical errors, malformed text, and unclear installation guidance.
- Add a root `docs.json` build contract modelled on `Ensono/stacks-infrastructure-aks`, with `docs/index.adoc` as its entry point and HTML and PDF as declared outputs.
- Add `eirctl` documentation tasks and a pinned/shared Asciidoctor execution context so contributors and CI use the same build command.
- Add PR validation that builds both declared formats, fails on AsciiDoc/rendering errors, validates the generated output, and retains bounded build artifacts for review.
- Replace Markdown-only documentation links and preserve navigation, local assets, code examples, and public documentation entry points.

## Capabilities

### New Capabilities
- `asciidoc-documentation-build`: Canonical AsciiDoc documentation sources, a declarative `docs.json` HTML/PDF build contract, and deterministic local/CI rendering and validation through `eirctl`.

### Modified Capabilities

None.

## Impact

- Affected documentation: `README.md`, all canonical guides and assets under `docs/`, and their intra-repository links.
- Affected automation: `eirctl` configuration, a documentation build script/task, and `.github/workflows/pr.yml` (or an equivalent pinned PR workflow integration).
- New tool/runtime dependency: the shared, version-pinned Ensono Asciidoctor container/toolchain used by the Stacks documentation pattern; it must be verified for availability, pinning, and supported output formats before implementation.
- Generated HTML/PDF build output is validation evidence and CI artifacts, not a new hosted documentation site in this change.
