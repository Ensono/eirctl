# Documentation build migration evidence

All sizes and registry metadata below were collected on 2026-08-12. Image tags are paired with the immutable manifest-list digest resolved from the Docker Registry HTTP API; local pull measurements are from Podman 5.8.2 on linux/amd64. Hosted-runner timing is supporting evidence only, not a correctness gate.

## Runtime selection

| Purpose | Authoritative source | Selected stable tag and digest | Published Linux architectures | Compressed layers | Pulled/on-disk (amd64) |
| --- | --- | --- | --- | ---: | ---: |
| HTML/PDF rendering | <https://hub.docker.com/r/asciidoctor/docker-asciidoctor>, <https://registry-1.docker.io/v2/asciidoctor/docker-asciidoctor/manifests/1.106.0> | `1.106.0@sha256:6266e05784c2d8ece9d9fe5e593b12c3beebebbc467135fd6f4a56269c93cea3` | `linux/amd64`, `linux/arm64` | 667,650,535 bytes | 1,501,051,251 bytes |
| Output validation | <https://hub.docker.com/_/node>, <https://registry-1.docker.io/v2/library/node/manifests/24.19.0-alpine3.24> | `24.19.0-alpine3.24@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43` | `linux/amd64`, `linux/arm64/v8`, `linux/s390x` | 58,204,379 bytes | 169,372,595 bytes |

The registry tag lists were queried before selection. `1.106.0` is the newest numeric stable `asciidoctor/docker-asciidoctor` tag, and `24.19.0-alpine3.24` is the newest available Node 24 Alpine tag. Both use an immutable, architecture-selecting manifest-list digest rather than a mutable-only tag.

The selected Asciidoctor image successfully ran `asciidoctor --failure-level WARN` and `asciidoctor-pdf --failure-level WARN` against `docs/index.adoc`. Focused renderer probes confirmed that an unterminated comment block and a missing include return non-zero at the configured `WARN` threshold; an output-creation error also returns non-zero. A missing local source image is carried into generated HTML and is rejected non-zero by the repository-owned generated-reference validator. The selected Node image ran `node scripts/validate-docs-output.mjs --manifest scripts/docs-output-manifest.json --output-root .eirctl/outputs/docs` successfully against the valid rendered files.

## Baseline shared image and output

| Item | Evidence |
| --- | --- |
| Shared image source | <https://hub.docker.com/r/ensono/eir-asciidoctor>, <https://registry-1.docker.io/v2/ensono/eir-asciidoctor/manifests/1.2.81> |
| Image reference | `1.2.81@sha256:e6d4d42394bcd3de42bbed653b0162f811cff33f8f9670a2c60cca89107b490d` |
| Published architectures | `linux/amd64`, `linux/arm64/v8` |
| Compressed layers | 1,340,519,508 bytes |
| Pulled/on-disk (amd64) | 3,327,138,981 bytes |
| Baseline CI run and job | <https://github.com/Ensono/eirctl/actions/runs/31389369535>, <https://github.com/Ensono/eirctl/actions/runs/31389369535/job/93457152336> |
| Baseline duration | 64 seconds total job; 52 seconds for `Build and validate documentation` |
| Baseline artifact | `rendered-documentation-31389369535-1`; its archived inspection contained `html/index.html`, `html/svg/denormalized.svg`, `html/svg/legend.svg`, `html/svg/normalized.svg`, and `pdf/index.pdf` |

The composed implementation produced the same required entries and three SVG assets locally. Its HTML and PDF signatures passed the repository validator, and generated local references resolved within the HTML output tree.

## Existing composed-pipeline CI comparison

The previous composed-pipeline trial is available at <https://github.com/Ensono/eirctl/actions/runs/31471275343/job/93714923153>. Its Documentation job ran for 41 seconds, with a 34-second `Build and validate documentation` step. The `rendered-documentation-31471275343-1` artifact was 120,381 bytes and contained `html/index.html`, the three SVG assets listed above, and `pdf/index.pdf`.

## Final-change CI evidence

The final documentation implementation run completed successfully:

| Item | Evidence |
| --- | --- |
| PR | <https://github.com/Ensono/eirctl/pull/143> |
| Workflow run | <https://github.com/Ensono/eirctl/actions/runs/31597370303> |
| Documentation job | <https://github.com/Ensono/eirctl/actions/runs/31597370303/job/94116069199> |
| Total Documentation job duration | 48 seconds (2026-08-12T12:38:02Z to 2026-08-12T12:38:50Z) |
| Build-and-validate step duration | 37 seconds (2026-08-12T12:38:10Z to 2026-08-12T12:38:47Z) |
| Artifact | `rendered-documentation-31597370303-1`, 120,779 bytes, SHA-256 `1150a07465c9db368850b91d545d91afc708520978d1c2524cc742f946c9a41c` |
| Inspected artifact contents | `html/index.html`, `html/svg/denormalized.svg`, `html/svg/legend.svg`, `html/svg/normalized.svg`, `pdf/index.pdf` |

The final implementation job is 16 seconds shorter than the recorded 64-second shared-image baseline. This timing comparison is supporting evidence only; it is not a correctness gate.

## Pre-existing dependency finding

The full lint pipeline was run on 2026-08-12 with rootless Podman using `DOCKER_HOST=unix:///run/user/1000/podman/podman.sock` and `EIRCTL_DOCKER_HOST=/run/user/1000/podman/podman.sock`. It completed its lint stages, but `govulncheck` reported GO-2026-4883 (CVE-2026-33997) and GO-2026-4887 (CVE-2026-34040) for the pre-existing direct dependency `github.com/docker/docker v28.5.2+incompatible`.

This documentation change does not modify `go.mod` or `go.sum`; the dependency version was introduced in commit `9bfbc996` (2026-02-24), before this change's base. The risk is acknowledged for this migration and will be remediated through a separate dependency/API migration change linked to [issue #95](https://github.com/Ensono/eirctl/issues/95). The security finding is not suppressed or treated as a clean scan.
