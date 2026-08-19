## Context

PR #143 introduced an AsciiDoc documentation build through a pinned `ensono/eir-asciidoctor` image, a root `docs.json` contract, and a `docs:build` task. Review identified that the image has a multi-gigabyte local footprint, can be expensive to pull in CI or bandwidth-constrained environments, obscures rendering behind an imported PowerShell module, and does not use eirctl pipeline composition.

A contributor added a sample composed pipeline using direct Asciidoctor and Node tasks and changed CI to invoke it. That path passed CI, produced a comparable rendered artifact, and reduced the observed documentation job from 64 seconds to 41 seconds. It was added alongside the original path, however, so the branch now contains two renderers, two commands, two output roots, and local guidance that differs from CI. This change formally supersedes the builder decisions in the archived `migrate-docs-to-asciidoc` change without rewriting that historical record.

## Goals / Non-Goals

**Goals:**

- Make one composed `eirctl` pipeline the canonical local and CI documentation build.
- Keep rendering commands, source, dependencies, task ordering, and output paths inspectable in repository configuration.
- Use smaller purpose-specific, immutable runtimes and retain evidence of their provenance, footprint, and CI performance.
- Write all generated documentation beneath the conventionally ignored `.eirctl/` directory.
- Preserve repository-owned rendered-output validation and its failure behavior.
- Adopt top-level `await` for the validator's ESM entry point.
- Remove the superseded shared-builder path and update every active consumer and instruction.

**Non-Goals:**

- Changing canonical AsciiDoc content, navigation, or supported HTML/PDF formats.
- Publishing or hosting generated documentation.
- Generalizing the validator for other repositories or importing it as shared static content.
- Changing `eirctl show` behavior for all pipelines.
- Establishing a hard CI duration service-level objective from a small sample of hosted-runner timings.

## Decisions

### 1. Replace, rather than supplement, the shared builder

`build:docs` SHALL be the sole supported documentation entry point. It will compose independent HTML and PDF renderer tasks and a validation task that depends on both renderers. The existing `docs` context, `docs:build` task, and `docs.json` file will be removed after equivalence is verified.

This resolves the duplicated-code and divergent-contract findings. Retaining both paths is rejected because future source, renderer, output, or validation changes could pass one path and fail the other. Keeping the original shared-builder path is also rejected because it preserves the image-footprint, composition, and opacity concerns that motivated this change.

### 2. Keep renderer and validator tasks explicit

The HTML task SHALL invoke `asciidoctor` directly with `docs/index.adoc`, an explicit failure threshold that returns non-zero for source warnings/errors, and an explicit `.eirctl/outputs/docs/html/index.html` destination, then copy required SVG assets into that generated tree. The PDF task SHALL invoke `asciidoctor-pdf` directly with the same failure posture and an explicit `.eirctl/outputs/docs/pdf/index.pdf` destination. The validation task SHALL invoke the repository script with explicit manifest and output-root arguments.

Missing includes and malformed source will be rejected by renderer failure-level handling. Missing local images, links, and generated assets will be rejected by renderer diagnostics and the generated HTML reference validator. Negative fixtures SHALL exercise every unchanged Documentation source validation scenario before the shared builder is removed.

Each constituent task will have a descriptive name and description so a contributor can inspect its command with `eirctl show <task>` or in `eirctl.yaml`; no build behavior will depend on an undisclosed function inside a container image. Expanding top-level pipeline output in `eirctl show` is not required for this change.

### 3. Use verified purpose-specific immutable runtimes

Rendering and validation will use purpose-specific Asciidoctor and Node images. Before implementation is finalized, each direct image SHALL be checked against its authoritative registry for the latest stable version compatible with the repository and pinned by immutable digest. A stable release tag plus digest is preferred over a mutable-only tag such as `main` or `latest`; if the upstream image does not provide a suitable stable tag, the selected digest and rationale will be recorded.

`evidence.md` in this change directory will record authoritative registry URLs, selected tags and digests, supported architectures, compressed size when available, pulled/on-disk size, CI run and job URLs, build-step and total-job timings, and artifact names/sizes/contents for both baseline and final pipelines. Digest pinning, read-only workflow permissions, and existing immutable-dependency checks remain mandatory. The Node validator stays isolated from renderer internals so local and CI behavior does not depend on Node being incidentally present in the Asciidoctor image.

### 4. Use `.eirctl/outputs/docs` as the sole generated-output root

HTML, PDF, copied assets, validator input, and uploaded CI artifacts SHALL use `.eirctl/outputs/docs`. The output root will be explicit at the task boundary and reflected consistently in the expected-output manifest or validator invocation. Active guidance will no longer refer to `outputs/docs`.

The directory is already ignored by `.gitignore`, follows existing eirctl generated-output convention, and prevents build artifacts from appearing as ordinary repository content.

### 5. Keep validation repository-owned and use top-level await

`scripts/validate-docs-output.mjs`, its manifest, and its tests are bespoke eirctl controls required to check eirctl's generated entry points, signatures, and local references. They will remain repository-owned rather than being imported through eirctl's static-file mechanism. Their ownership and purpose will be stated in active build documentation so future reviewers do not mistake them for copied runner assets.

The module entry point will become:

```js
await main().catch((error) => {
  console.error(`Documentation output validation failed: ${error.message}`);
  process.exitCode = 1;
});
```

This is a style change enabled by the `.mjs` module format. It SHALL preserve the current diagnostic text, rejection handling, and non-zero process status, and existing failure tests will guard those semantics.

### 6. Keep local instructions, CI, and review evidence aligned

`CONTRIBUTING.md`, `docs/building.adoc`, and PR CI SHALL all invoke `go run cmd/main.go run build:docs` (with `--verbose` where appropriate) and refer to `.eirctl/outputs/docs`. CI will continue uploading bounded artifacts on success and failure for seven days under existing least-privilege permissions and full-SHA-pinned actions. The PR description will be updated to describe the composed contract and current validation evidence.

## Risks / Trade-offs

- **Direct rendering differs from the former shared builder** → Build with both paths once during migration, compare required entries/assets, run rendered-link validation, and remove the old path only after equivalence is demonstrated.
- **Two purpose-specific images may still incur pull overhead** → Record compressed/on-disk footprint and repeated CI timings; keep only runtimes required by an explicit task.
- **A digest attached to a mutable tag can obscure the human-readable version** → Prefer an authoritative stable release tag plus digest and document any exception.
- **Removing `docs.json` breaks the previously documented contributor command** → Treat the command/output change as a deliberate developer-workflow break and update all active guidance and PR metadata in the same change.
- **Parallel renderers write under one root** → Keep HTML and PDF in disjoint subdirectories and make validation depend on both tasks.
- **Hosted-runner timing varies** → Use timing as supporting evidence rather than a brittle pass/fail threshold; correctness and immutable pinning remain mandatory gates.

## Migration Plan

1. Verify authoritative stable image references, immutable digests, compatibility, and footprint for the renderer and validator runtimes.
2. Establish a baseline from the existing shared-builder job and build artifacts.
3. Finalize the composed tasks and `.eirctl/outputs/docs` contract; run HTML, PDF, and validation locally.
4. Compare generated required entries and assets with the baseline, then remove `docs`, `docs:build`, and `docs.json`.
5. Update the manifest/defaults, validator entry-point style, tests, contributor documentation, CI command/artifact path, PR description, and active OpenSpec requirement deltas.
6. Run source-negative fixtures, focused rendered-output validator tests, the canonical documentation pipeline, workflow security and immutable-dependency checks, and relevant lint/test/build checks; complete `evidence.md` with runtime, CI timing, and artifact evidence.

Rollback is a single revert of this change, restoring the shared builder, `docs.json`, `docs:build`, and `outputs/docs` together. The two build contracts must not coexist after either migration or rollback.

## Open Questions

- Which authoritative stable Asciidoctor image tag and digest should replace the current `main`-labelled digest while preserving HTML/PDF compatibility?
    - Answer: The latest stable release tag plus digest from the official Asciidoctor Docker registry, verified for compatibility with the repository's AsciiDoc content and PDF generation requirements.
- Does the chosen Asciidoctor image expose a reliable compressed size through its registry, or should the implementation record only pulled on-disk size plus CI timing?
    - Answer: The implementation should record both the compressed size (if available) and the pulled on-disk size, along with CI timing, to provide comprehensive evidence of the image's footprint and performance.
