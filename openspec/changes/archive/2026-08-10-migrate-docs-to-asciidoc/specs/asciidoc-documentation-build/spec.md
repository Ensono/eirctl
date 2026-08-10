## ADDED Requirements

### Requirement: Canonical AsciiDoc documentation sources
The repository SHALL maintain every substantive user guide currently owned under `docs/` as an AsciiDoc source file. `docs/index.adoc` SHALL be the canonical documentation entry point. `README.md` SHALL remain Markdown and SHALL link to the canonical AsciiDoc documentation without duplicating guide content.

#### Scenario: Contributor opens repository documentation
- **WHEN** a contributor opens `README.md` or `docs/index.adoc`
- **THEN** they can navigate to every canonical user guide through valid repository-relative links

#### Scenario: Existing guide is migrated
- **WHEN** a Markdown guide is replaced by its AsciiDoc counterpart
- **THEN** the Markdown source is removed, its inbound links target the AsciiDoc source, and its prose, code examples, diagrams, and applicable YAML assets remain available through the canonical documentation entry point

### Requirement: Declarative documentation build contract
The repository SHALL provide a root `docs.json` documentation build contract compatible with the shared Ensono Asciidoctor builder. The contract SHALL declare `docs/index.adoc` as its source, HTML and PDF as required formats, and a deterministic repository-relative output location for each format.

#### Scenario: Contributor inspects documentation contract
- **WHEN** a contributor reads `docs.json`
- **THEN** they can determine the documentation entry point, requested HTML/PDF formats, and output directory without inspecting CI configuration

#### Scenario: Documentation builder consumes the contract
- **WHEN** the repository documentation task runs
- **THEN** it invokes the shared builder using `docs.json` and produces the declared HTML and PDF outputs

### Requirement: Reproducible eirctl documentation build
The repository SHALL expose an `eirctl` task that builds the documentation from `docs.json` using a verified, immutable shared Asciidoctor runtime. The task SHALL be runnable by contributors and CI without workflow-specific build logic.

#### Scenario: Contributor builds documentation locally
- **WHEN** a contributor runs the documented `eirctl` documentation task in an environment with the supported container runtime
- **THEN** the task renders HTML and PDF into the contract's output directory

#### Scenario: Unsupported builder runtime
- **WHEN** the configured shared Asciidoctor runtime cannot be pulled, started, or render the required formats
- **THEN** the documentation task fails with a non-zero status and actionable error output

### Requirement: Documentation source validation
The documentation build SHALL fail on malformed AsciiDoc, missing includes, unresolved local source references, or renderer errors. Source validation SHALL cover the canonical entry point and all included canonical documentation pages.

#### Scenario: Missing included page
- **WHEN** `docs/index.adoc` or a guide includes a nonexistent local file
- **THEN** the documentation validation fails before the pull request can pass

#### Scenario: Malformed AsciiDoc source
- **WHEN** a canonical AsciiDoc page contains syntax that the configured renderer cannot process
- **THEN** the documentation validation fails with the renderer error

### Requirement: Known-good rendered documentation output
After a successful documentation build, repository-owned validation SHALL verify the generated HTML and PDF against a committed expected-output manifest. Validation SHALL require the declared HTML entry document and PDF artifact to exist, be non-empty, have their respective format signatures, and ensure generated HTML local references resolve within the generated output tree.

#### Scenario: Complete valid build output
- **WHEN** the documentation task renders the configured entry point and all required assets
- **THEN** output validation accepts the HTML and PDF artifacts and their generated local references

#### Scenario: Missing or malformed PDF output
- **WHEN** the renderer does not produce the required PDF, produces an empty PDF, or produces a file without a PDF signature
- **THEN** output validation fails with the missing or invalid artifact identified

#### Scenario: Broken generated HTML reference
- **WHEN** generated HTML references a missing local stylesheet, image, page, or other generated asset
- **THEN** output validation fails with the referring document and unresolved target identified

### Requirement: Pull-request documentation regression checks
The pull-request validation workflow SHALL run the same `eirctl` documentation task and rendered-output validation on relevant documentation, build-contract, task, validation-script, or workflow changes. It SHALL preserve the workflow's least-privilege permissions and immutable dependency policy, and SHALL retain bounded rendered artifacts for review.

#### Scenario: Documentation change in pull request
- **WHEN** a pull request changes a canonical AsciiDoc page, `docs.json`, documentation task/configuration, or output-validation code
- **THEN** the pull request check builds and validates HTML and PDF before it can succeed

#### Scenario: Documentation build fails in pull request
- **WHEN** source or rendered-output validation fails in a pull request
- **THEN** the check fails and bounded available build artifacts are retained for diagnostics
