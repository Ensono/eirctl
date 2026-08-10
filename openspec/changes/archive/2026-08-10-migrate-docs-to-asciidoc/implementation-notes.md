# Migration Implementation Notes

## Shared Asciidoctor runtime

- **Authoritative integration reference:** `Ensono/stacks-infrastructure-aks` at commit `fafceaff11bb5304876f3c703e14e21a47dcb33a`, specifically `build/eirctl/contexts.yaml`, `build/eirctl/tasks.yaml`, and `docs.json`.
- **Reference runtime tag:** `ensono/eir-asciidoctor:1.2.81`.
- **Resolved immutable multi-platform image index:** `docker.io/ensono/eir-asciidoctor:1.2.81@sha256:e6d4d42394bcd3de42bbed653b0162f811cff33f8f9670a2c60cca89107b490d` (queried from Docker Hub on 2026-08-10). This is the runtime pin used by this change.
- **Verified local runtime:** Podman 5.8.2 pulled and ran the pinned image. The image contains the `EnsonoBuild` module at `/modules/EnsonoBuild/EnsonoBuild.psd1`; after importing it, `Build-Documentation` is available.
- **Verified builder interface:** `Build-Documentation -Config <path> -Basepath <path> [-Verbose]`. Its help lists `pdf` and `html` as supported `-Type` values. The reference invokes `Build-Documentation -Config /eirctl/docs.json -BasePath /eirctl -Verbose`.
- **Container prerequisite:** a supported OCI container runtime. `podman` is available in this environment; the repository documentation will list Docker, Podman, and nerdctl as supported runtime commands where applicable.

## Guide migration inventory

| Markdown source | Canonical replacement | Active inbound links to update | Retained local assets/references |
| --- | --- | --- | --- |
| `docs/installation.md` | `docs/installation.adoc` | `README.md` | None |
| `docs/import.md` | `docs/import.adoc` | `README.md`, `.github/copilot-instructions.md`, `.github/skills/eirctl-project-automation/SKILL.md`, `shared/README.md`, `docs/ci-security.md` | None |
| `docs/artifacts.md` | `docs/artifacts.adoc` | `README.md`, `docs/ci-security.md` | None |
| `docs/watchers.md` | `docs/watchers.adoc` | `README.md` | None |
| `docs/ci-generator.md` | `docs/ci-generator.adoc` | None | `cmd/eirctl/testdata/gha.sample.yml`, `internal/genci/genci.go`, `internal/genci/githubimpl.go` |
| `docs/graph-implementation.md` | `docs/graph-implementation.adoc` | `README.md`, `.github/copilot-instructions.md` | `docs/svg/denormalized.svg`, `docs/svg/legend.svg`, `docs/svg/normalized.svg` |
| `docs/v2.md` | `docs/v2.adoc` | `docs/ci-security.md` | None |
| `docs/ci-security.md` | `docs/ci-security.adoc` | None | None |

## Local build command and runtime prerequisites

Run the local build from the repository root:

```sh
go run cmd/main.go run docs:build --verbose
```

The task requires an installed, running Docker-API-compatible OCI runtime that `eirctl` can reach using its configured Docker client; Docker Engine is the CI runtime and Podman is suitable when exposed through its Docker-compatible socket. The pinned runtime requires network access to pull `docker.io/ensono/eir-asciidoctor` on its first run. `nerdctl` is not documented as supported because its Docker-API compatibility has not been verified with this `eirctl` task.

## Validation record

Successful commands on 2026-08-10:

* `node --test scripts/validate-docs-output.test.mjs`
* `DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock go run cmd/main.go run docs:build`
* `scripts/check-immutable-ci-dependencies.sh`
* `scripts/check-workflow-security.sh`
* `DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock EIRCTL_DOCKER_HOST=/run/user/$(id -u)/podman/podman.sock go run cmd/main.go run lints`
* `go test ./...`
* `go build -o /tmp/eirctl-doc-migration ./cmd/main.go`

No planned checks were skipped. The `go:vuln:check` lint stage reports currently affected upstream Docker-module advisories but exits successfully under the repository's existing lint policy; this change does not modify Go dependencies.

## Requirement-scenario evidence

* Canonical navigation: `docs/index.adoc` includes all eight migrated guides and `README.md` links to that entry point and canonical `.adoc` pages.
* Declarative/reproducible build: `docs.json` declares the index source, deterministic `outputs/docs/{{ format }}` paths, and HTML/PDF formats; `eirctl.yaml` invokes the pinned builder and output validator.
* Source/rendered validation: the successful `docs:build` run rendered the complete include tree and `scripts/validate-docs-output.mjs` verified required, non-empty, signature-valid HTML/PDF plus generated local HTML references.
* Failure coverage: `scripts/validate-docs-output.test.mjs` verifies missing output, invalid HTML/PDF signatures, and unresolved generated local references.
* Pull requests: `.github/workflows/pr.yml` runs `docs:build` under read-only workflow permissions and uploads `outputs/docs` for seven days whether the build succeeds or fails.
* Change artifacts: `openspec validate migrate-docs-to-asciidoc --strict --json` passed.

All `docs/*.md` sources were inventoried. The migration retains the documented YAML assets (`docs/.generated-github-workflow.yml`, `docs/docker-compose.yaml`, `docs/example.yaml`, `docs/pipeline.yaml`, and `docs/samples/docker.build.yaml`), the logo, and the graph SVGs. Historical OpenSpec archive/spec links are also inbound references but will remain unchanged so their historical records are not rewritten; active documentation and repository guidance links above are the links in scope for update.
