## 1. Establish the supported documentation toolchain

- [x] 1.1 Verify the authoritative immutable version or digest and documented interface of the shared Ensono Asciidoctor runtime; record the source of truth and stop for a decision if it cannot produce both HTML and PDF.
- [x] 1.2 Inventory each `docs/*.md` guide, its inbound links, its target `.adoc` file, and retained supporting assets; record the mapping in the migration implementation notes.
- [x] 1.3 Define the root `docs.json` contract with `docs/index.adoc`, HTML/PDF formats, deterministic output paths, and only renderer attributes required by eirctl documentation content.

## 2. Add reproducible documentation build and validation

- [x] 2.1 Add an `eirctl` documentation context/task that invokes the shared builder with `docs.json`; document the exact local command and supported container-runtime prerequisites.
- [x] 2.2 Add a repository-owned expected-output manifest and validation script that checks required HTML/PDF artifacts, non-empty output, signatures, and generated local HTML references.
- [x] 2.3 Add focused automated tests or fixtures for output validation failure cases: missing output, invalid PDF/HTML signature, and unresolved generated local reference.
- [x] 2.4 Build HTML and PDF locally through the `eirctl` task; fix renderer, include, and asset failures before converting all guides.

## 3. Migrate canonical documentation

- [x] 3.1 Create `docs/index.adoc` and the common AsciiDoc document attributes/navigation structure, preserving the existing guide hierarchy.
- [x] 3.2 Convert installation, import, artifacts, watchers, CI-generator, graph implementation, V2 migration, and CI-security guides to `.adoc`; retain applicable code samples, diagrams, and YAML references.
- [x] 3.3 Correct editorial, platform/product naming, stale version-example, malformed-text, and installation-guidance defects during conversion without altering documented product behavior.
- [x] 3.4 Update `README.md` and all documentation links to the canonical `.adoc` pages; keep README as the concise GitHub-rendered landing page.
- [x] 3.5 Remove superseded Markdown guides only after source and rendered-link validation demonstrates complete replacement.

## 4. Enforce pull-request regression checks

- [x] 4.1 Integrate the `eirctl` documentation build and output validation into PR validation for documentation/build-contract/task/validator/workflow changes, preserving existing read-only permissions and full-SHA action pinning.
- [x] 4.2 Upload bounded HTML/PDF artifacts on success and failure with the repository's approved retention policy; do not publish generated documentation or expose credentials.
- [x] 4.3 Run the narrow documentation validation, then the relevant workflow-security, immutable-dependency, lint, test, and build checks; record all successful commands and any deliberately skipped checks.

## 5. Review migration readiness

- [x] 5.1 Verify every requirement scenario in `asciidoc-documentation-build` against implementation evidence.
- [x] 5.2 Review the migration diff for link compatibility, README GitHub rendering, removed Markdown sources, generated-output exclusions, and unrelated changes before opening the `docs/cleanup` pull request.
