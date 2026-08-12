## 1. Verify runtimes and establish evidence

- [x] 1.1 Query the authoritative Asciidoctor and Node image registries for current stable compatible releases, select immutable digests, and record tag, digest, architecture support, and source URLs in `evidence.md`; do not retain a mutable-only tag without documenting why no stable tag is suitable.
- [x] 1.2 Record in `evidence.md` the shared `ensono/eir-asciidoctor` image's available compressed and pulled/on-disk footprint plus baseline CI run/job URLs, total job duration, build-step duration, artifact name, size, and inspected contents.
- [x] 1.3 Verify the selected Asciidoctor runtime renders both HTML and PDF for `docs/index.adoc` and the selected Node runtime executes the repository validator before removing the old path.

## 2. Make the composed pipeline canonical

- [x] 2.1 Finalize purpose-specific, digest-pinned Asciidoctor and Node contexts in `eirctl.yaml`, preserving only the workspace mount and capabilities required to read sources and write generated output.
- [x] 2.2 Define `build:docs` as HTML and PDF renderer tasks plus a validation task that depends on both, with direct commands, renderer failure-level arguments that return non-zero for source warnings/errors, and explicit `.eirctl/outputs/docs/html/index.html`, `.eirctl/outputs/docs/pdf/index.pdf`, manifest, and output-root arguments.
- [x] 2.3 Verify `eirctl show` for each constituent task exposes a descriptive task name, runtime context, direct executable, source, and output path without relying on a container-internal imported module.
- [x] 2.4 Compare required entries, SVG assets, signatures, and generated local references between the old and composed builds, then remove the old `docs` context, `docs:build` task, and root `docs.json` contract so only one build path remains.

## 3. Align output validation and style

- [x] 3.1 Align `scripts/docs-output-manifest.json`, validator defaults/arguments, tests, and all generated paths on `.eirctl/outputs/docs`, keeping the effective output root explicit at the task boundary.
- [x] 3.2 Change the `.mjs` entry point to `await main().catch(...)` while preserving the current concise error message and non-zero `process.exitCode` behavior.
- [x] 3.3 Add or update focused fixtures that prove the canonical pipeline fails non-zero for malformed AsciiDoc, missing includes, unresolved local source images/links, and renderer errors, plus rendered-output tests for successful output, missing artifacts, invalid HTML/PDF signatures, unresolved generated references, and the top-level entry point's diagnostic and unsuccessful status.
- [x] 3.4 Document that the validator and expected-output manifest are bespoke eirctl-owned controls, not copied runner files or candidates for static-file import.

## 4. Align contributor and CI contracts

- [x] 4.1 Make the PR Documentation job invoke the same documented `build:docs` pipeline and upload bounded artifacts only from `.eirctl/outputs/docs` on both success and failure, preserving read-only permissions, full-SHA action pins, and seven-day retention.
- [x] 4.2 Update `CONTRIBUTING.md`, `docs/building.adoc`, the PR description, and other active guidance to use `build:docs`, `.eirctl/outputs/docs`, the direct runtime prerequisites, and the repository-owned validation contract.
- [x] 4.3 Search active source, configuration, workflow, and guidance files for stale `docs:build`, `outputs/docs`, `docs.json`, shared-image, and PowerShell-module references; retain historical OpenSpec archive text unchanged.

## 5. Validate and record the migration

- [x] 5.1 Run `node --test scripts/validate-docs-output.test.mjs` and record the result.
- [x] 5.2 Run `build:docs` through an available Docker-API-compatible runtime, directly inspect the HTML/PDF entries and copied assets, and confirm generated files remain ignored beneath `.eirctl/`.
- [x] 5.3 Run `scripts/check-immutable-ci-dependencies.sh` and `scripts/check-workflow-security.sh` after the workflow and runtime changes.
- [x] 5.4 Run the relevant lint, Go test, and build checks, plus configured pre-commit checks if present, without weakening existing policies.
- [ ] 5.5 Complete `evidence.md` with final authoritative registry URLs, tags/digests, architectures, available compressed and pulled/on-disk footprints, PR run/job URLs, total job and build-step timings, artifact names/sizes/contents, and the baseline comparison; do not treat hosted-runner timing as a hard correctness gate.
- [x] 5.6 Verify every modified and added `asciidoc-documentation-build` scenario, confirm there is one canonical local/CI build contract, and run strict OpenSpec validation before requesting review.
