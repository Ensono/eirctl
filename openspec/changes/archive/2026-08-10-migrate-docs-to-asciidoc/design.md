## Context

`eirctl` currently keeps its user guides as static Markdown under `docs/`, with no documentation renderer, output contract, or PR validation. The repository already uses `eirctl` as its orchestration layer and has a security-sensitive GitHub Actions lint job that runs only pinned workflow actions.

The reference implementation in `Ensono/stacks-infrastructure-aks` defines the source entry point and `html`/`pdf` outputs in root `docs.json`, then invokes `Build-Documentation -Config /eirctl/docs.json -BasePath /eirctl` through an `eirctl` `_docs` task in a shared `ensono/eir-asciidoctor` context. Its `docs.json` declares `docs/index.adoc` and `outputs/docs/{{ format }}`. This change adopts that integration pattern, not its infrastructure-specific content or styling.

## Goals / Non-Goals

**Goals:**

- Make AsciiDoc the canonical source format for every substantive guide under `docs/`.
- Use a root `docs.json` to declare one source entry point and deterministic HTML and PDF outputs.
- Build and validate the same outputs locally and in PR CI through `eirctl` and a pinned shared Asciidoctor context.
- Treat rendered artifacts, link/include resolution, and expected output shape as merge-blocking documentation quality checks.
- Keep a concise, GitHub-rendered `README.md` that links to canonical AsciiDoc guides.
- Repair documentation issues discovered while converting, without changing product behavior.

**Non-Goals:**

- Publishing or hosting a new documentation site.
- Migrating generated YAML examples, SVG assets, schemas, or source-code API documentation to AsciiDoc.
- Recreating the full Stacks Infrastructure AKS theme, glossary generation, Azure DevOps pipeline, or PDF extensions unless required by `eirctl` content.
- Committing generated HTML or PDF to the repository.

## Decisions

### 1. Use `docs.json` and the shared `eirctl` documentation task pattern

A root `docs.json` SHALL declare `docs/index.adoc` as the source, `html` and `pdf` as formats, and a deterministic repository-relative output directory. An `eirctl` documentation task SHALL run the shared documentation builder against that file.

This matches the requested Stacks pattern and lets contributors run the same command as CI. Hand-written renderer shell commands in the workflow are rejected because they drift from the local build contract.

### 2. Keep Markdown only for the repository landing page

`README.md` remains Markdown because GitHub presents it as the repository landing page. It will be shortened only where necessary and link to converted `docs/*.adoc` guides. Canonical documentation content will not be duplicated between README and the AsciiDoc pages.

Replacing the root README with AsciiDoc would reduce repository discoverability. Generating a Markdown README adds a second output and a drift problem without serving the requested docs build contract.

### 3. Use one AsciiDoc entry point and include-based navigation

`docs/index.adoc` SHALL be the documentation entry point. Converted guides SHALL be linked or included through that entry point using paths that remain valid when rendered from the repository root. Images, SVG diagrams, YAML samples, and code blocks SHALL retain repository-relative references compatible with both configured outputs.

This is a smaller, directly compatible shape than introducing Antora. Antora is rejected for this change because it adds a documentation-site architecture and navigation/runtime configuration beyond the requested shared `docs.json` builder pattern.

### 4. Validate source and known-good rendered output in CI

The documentation task SHALL fail on renderer, AsciiDoc include, image, or link-resolution errors. After both formats build, a repository-owned validation script SHALL check a committed expected-output manifest against the generated directory. The manifest SHALL identify the required HTML entry document and PDF artifact, and validation SHALL assert they exist, are non-empty, and have format-specific signatures (`<!doctype html`/`<html` for HTML and `%PDF-` for PDF). It SHALL also check that local references emitted in HTML resolve within the generated output.

This checks observable output without committing generated artifacts or relying on fragile full-document byte snapshots. A syntax-only linter is rejected because it cannot detect renderer- or asset-level regressions.

### 5. Make CI reproducible and least-privileged

The existing PR lint job SHALL invoke the documentation task using its read-only permissions. The Asciidoctor runtime SHALL use a verified, immutable version or digest consistent with the repository's CI dependency policy; the implementation MUST not fetch an unpinned latest image or disable validation to accommodate environment problems. Bounded HTML/PDF artifacts SHALL be uploaded for failed and successful runs to aid review.

## Risks / Trade-offs

- **Shared container availability or compatibility is not guaranteed by the current repository** → Verify the authoritative `ensono/eir-asciidoctor` image/version or digest and its `Build-Documentation` interface before pinning; stop for a decision if it is inaccessible or cannot render both formats.
- **Relative Markdown links can break during conversion** → Maintain a source-to-target migration inventory, validate all local source references, and validate rendered HTML links.
- **PDF rendering can vary across tool versions/fonts** → Pin the builder runtime, check structural output rather than binary equality, and upload the PDF as evidence.
- **README and guides can diverge** → Keep README limited to orientation, installation entry links, badges, and high-level project information; do not duplicate complete guides.
- **Existing PR workflow is security-sensitive** → Preserve permissions and full-SHA action pinning; isolate build commands in repository scripts/tasks and do not introduce secrets.

## Migration Plan

1. Inventory each `docs/*.md` source, its inbound README/docs links, and its `.adoc` target before moving files.
2. Add the `docs.json` contract, shared Asciidoctor context, `eirctl` documentation task, and output-validation script/manifest.
3. Convert the guides and update links, correcting editorial and stale-example defects as they are encountered; remove each replaced `.md` only after its `.adoc` replacement and links are verified.
4. Build HTML and PDF locally with `eirctl`; run source, output, and link validation; then integrate the same command in PR CI with bounded artifacts.
5. Roll back by reverting the migration commit(s): Markdown sources remain recoverable in Git history and no generated output is published or persisted as source of truth.

## Open Questions

- Which immutable tag or digest of the shared `ensono/eir-asciidoctor` image is the supported version for this repository and Go/CI environment?
- Does the shared builder require a standard `docs.json` schema field beyond the AKS reference's `title`, `output`, `path`, `formats`, `libs`, and format attributes for this documentation set?
- Which workflow/reusable workflow provides the organization-standard artifact retention and documentation-builder setup, if one exists outside the inspected repository examples?
