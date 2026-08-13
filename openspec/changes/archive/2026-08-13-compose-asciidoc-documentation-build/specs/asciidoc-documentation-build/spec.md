## ADDED Requirements

### Requirement: Inspectable and resource-conscious documentation execution
The documentation build SHALL use purpose-specific runtime images selected from authoritative registries, verified for compatibility, and pinned to immutable digests. Renderer and validator commands, source entry point, output paths, and task dependencies SHALL be declared directly in repository-owned `eirctl` configuration rather than hidden behind a container-internal imported module. The repository SHALL record runtime footprint and before/after CI timing evidence when changing the documentation runtime.

#### Scenario: Contributor inspects a documentation task
- **WHEN** a contributor reads `eirctl.yaml` or runs `eirctl show` for a constituent documentation task
- **THEN** they can identify the executable, source entry point, output path, runtime context, and validation command without inspecting files inside the runtime image

#### Scenario: Documentation runtime dependency changes
- **WHEN** an Asciidoctor or Node runtime reference is added or changed
- **THEN** its authoritative stable version compatibility, immutable digest, and available footprint evidence are recorded before the change is accepted

### Requirement: Repository-owned documentation validator entry point
The rendered-output validator and expected-output manifest SHALL remain repository-owned controls for eirctl documentation. The ESM validator SHALL await its asynchronous main function at top level, report validation failures to standard error, and set a non-zero process status without exposing stack traces during expected validation failures.

#### Scenario: Validator rejects invalid generated output
- **WHEN** repository-owned validation detects a missing, empty, malformed, or internally broken generated artifact
- **THEN** the top-level awaited entry point reports the actionable validation message and the process exits unsuccessfully

#### Scenario: Contributor inspects validator provenance
- **WHEN** a contributor reads the active documentation build guidance
- **THEN** it identifies the validator and manifest as eirctl-owned files rather than copied or imported runner assets

## MODIFIED Requirements

### Requirement: Declarative documentation build contract
The repository SHALL declare one canonical `build:docs` pipeline in repository-owned `eirctl` configuration. The pipeline SHALL identify `docs/index.adoc` as its source, HTML and PDF as required formats, explicit deterministic output paths beneath `.eirctl/outputs/docs`, and a rendered-output validation task that depends on both renderer tasks. The repository SHALL NOT retain a second documentation task, `docs.json` contract, or alternate output root for the same build.

#### Scenario: Contributor inspects documentation contract
- **WHEN** a contributor reads the documentation pipeline and its constituent task definitions
- **THEN** they can determine the documentation entry point, HTML/PDF renderers, task dependencies, validation command, and output directories without inspecting CI configuration or container-internal modules

#### Scenario: Documentation pipeline executes
- **WHEN** the canonical documentation pipeline runs
- **THEN** it renders HTML and PDF to `.eirctl/outputs/docs`, copies required local assets, and validates both formats through the repository-owned validator

### Requirement: Reproducible eirctl documentation build
The repository SHALL expose `build:docs` as the sole supported `eirctl` documentation pipeline. The pipeline SHALL compose explicit HTML, PDF, and rendered-output validation tasks using verified, immutable, purpose-specific runtimes and SHALL be runnable by contributors and CI without workflow-specific rendering logic.

#### Scenario: Contributor builds documentation locally
- **WHEN** a contributor runs the documented `build:docs` pipeline in an environment with the supported container runtime
- **THEN** the pipeline renders and validates HTML and PDF beneath `.eirctl/outputs/docs`

#### Scenario: Unsupported renderer runtime
- **WHEN** a configured renderer or validator runtime cannot be pulled, started, or execute its required command
- **THEN** the documentation pipeline fails with a non-zero status and actionable error output identifying the failed constituent task

### Requirement: Known-good rendered documentation output
After a successful documentation build, repository-owned validation SHALL verify generated HTML and PDF beneath `.eirctl/outputs/docs` against the committed expected-output manifest. Validation SHALL require the declared HTML entry document and PDF artifact to exist, be non-empty, have their respective format signatures, and ensure generated HTML local references resolve within the generated output tree. The validation task SHALL receive its manifest and effective output root explicitly.

#### Scenario: Complete valid build output
- **WHEN** the documentation pipeline renders the configured entry point and all required assets beneath `.eirctl/outputs/docs`
- **THEN** output validation accepts the HTML and PDF artifacts and their generated local references

#### Scenario: Missing or malformed PDF output
- **WHEN** the renderer does not produce the required PDF, produces an empty PDF, or produces a file without a PDF signature
- **THEN** output validation fails with the missing or invalid artifact identified

#### Scenario: Broken generated HTML reference
- **WHEN** generated HTML references a missing local stylesheet, image, page, or other generated asset
- **THEN** output validation fails with the referring document and unresolved target identified

### Requirement: Pull-request documentation regression checks
The pull-request validation workflow SHALL run the same canonical `build:docs` pipeline used by contributors for relevant documentation, pipeline, validation-script, runtime, or workflow changes. It SHALL preserve the workflow's least-privilege permissions and immutable dependency policy, and SHALL retain bounded rendered artifacts from `.eirctl/outputs/docs` for review.

#### Scenario: Documentation change in pull request
- **WHEN** a pull request changes a canonical AsciiDoc page, documentation pipeline/configuration, runtime reference, output-validation code, or related workflow
- **THEN** the pull-request check runs `build:docs` and validates HTML and PDF before it can succeed

#### Scenario: Documentation build fails in pull request
- **WHEN** source or rendered-output validation fails in a pull request
- **THEN** the check fails and bounded available artifacts from `.eirctl/outputs/docs` are retained for diagnostics
