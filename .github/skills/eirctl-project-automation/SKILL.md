---
name: eirctl-project-automation
description: "Use when: installing eirctl, editing eirctl.yaml, or running project tooling through existing eirctl tasks, pipelines, and contexts. In the eirctl repo, prefer the built-in build, test, lint, coverage, and schema tasks before adding new automation."
argument-hint: "Describe the project command or workflow to run via eirctl"
---

# eirctl Project Automation

## Purpose

Use `eirctl` as the project task runner and tool execution boundary. Prefer existing `eirctl.yaml` tasks, pipelines, and contexts over direct host execution.

In the `eirctl` repository itself, start from the root `eirctl.yaml` and its imported shared definitions under `shared/` before creating anything new. This repo already has build, test, lint, coverage, schema generation, and security automation defined.

## Core Rule

Do **not** run project tooling directly when an `eirctl` task, pipeline, or context already exists.

Prefer:

- `eirctl <task-or-pipeline>` or `eirctl run <task-or-pipeline>` for the common case
- `eirctl run task <task-name>` or `eirctl run pipeline <pipeline-name>` when explicit disambiguation helps
- `eirctl show <task|pipeline|context-name>` to inspect an existing definition
- `eirctl graph <pipeline-name>` to inspect pipeline shape and dependencies
- `eirctl shell <context-name>` for interactive work inside a native container context

Avoid direct host execution such as:

- `go test`, `go build`, `golangci-lint run`, `govulncheck ./...`
- `docker build`
- `terraform init`, `terraform plan`, `terraform apply`
- `helm template`, `helm upgrade`, `helm lint`
- `kubectl apply`, `kubectl diff`

Unless the user explicitly asks for direct execution or no reasonable `eirctl` path exists.

## eirctl Repo Workflow

When working in the `eirctl` repo, prefer the existing automation in the root config first:

1. Run `eirctl validate`.
2. Run `eirctl list`.
3. Reuse the repo's current tasks and pipelines before adding new ones, especially:
    - `eirctl build:binary`
    - `eirctl test:unit`
    - `eirctl lints`
    - `eirctl tidy`
    - `eirctl show:coverage`
    - `eirctl generate_own_schema`
    - `eirctl build:container`
4. Inspect definitions with `eirctl show <name>`, for example:
    - `eirctl show build:binary`
    - `eirctl show test:unit`
    - `eirctl show go1x`
5. Reuse imported shared definitions from:
    - `shared/build/go/eirctl.yaml`
    - `shared/security/eirctl.yaml`

Only add new tasks or contexts when the existing ones do not cover the workflow.

Repo-validated inspection commands in this repository:

- `eirctl --version`
- `eirctl validate`
- `eirctl list`
- `eirctl show build:binary`
- `eirctl show test:unit`
- `eirctl show go1x`
- `eirctl graph test:unit`

Repo task examples below are valid command shapes, but some require extra inputs or host capabilities to succeed:

- `eirctl build:binary` and `eirctl build:container` depend on template values such as `.Version` and `.Revision` being available in the task context.
- `eirctl tidy`, `eirctl test:unit`, `eirctl lints`, and `eirctl show:coverage` depend on working container execution in this environment.
- `eirctl generate_own_schema` may require outbound network access and a trusted CA chain for Go module downloads.

## Installation Guidance

First check whether `eirctl` is available:

1. Run `eirctl --version`.
2. If unavailable, install the latest release for the host OS and architecture from `https://github.com/Ensono/eirctl/releases`.
3. On Linux or macOS, download the relevant binary, mark it executable, and place it on `PATH` such as `/usr/local/bin` or `$HOME/.local/bin`.
4. Verify with `eirctl --version`.

For temporary agent environments, prefer installing into `$HOME/.local/bin` to avoid requiring elevated privileges. Ensure that directory is on `PATH` before invoking `eirctl`.

## Project Discovery Procedure

When working in a project that uses `eirctl`:

1. Look for an existing `eirctl.yaml` in the repository root or current working tree.
2. If present, inspect available automation with:
    - `eirctl validate`
    - `eirctl list`
    - `eirctl show <name>`
    - `eirctl graph <pipeline-name>`
3. If no `eirctl.yaml` exists and the user wants reproducible project automation, run or recommend `eirctl init`, then tailor the generated config.
4. Use existing tasks and pipelines before creating new ones.
5. If a needed command is missing, add a task and, where tool versions matter, add a context.

## Context Requirements

Use contexts to pin tools, isolate environments, and avoid host-specific dependencies.

Recommended context choices:

- Reuse existing contexts before adding new ones.
- Use a container context for toolchains with important versions, such as Go, Terraform, Helm, Node.js, Java, Python, or Cloud CLIs.
- Use an executable context only when a host-installed binary is intentionally part of the workflow.
- Leave a task without a context only for simple shell-safe logic that is genuinely portable, or when Docker itself is the tool being exercised.

In the `eirctl` repo specifically, check whether one of the existing contexts already fits before adding another, for example `go1x`, `golint`, `goreleaser`, `bash`, `yamllint`, `trivy:container`, or `sonar`.

Container contexts should prefer pinned image tags over `latest`.

Example pattern:

```yaml
contexts:
    terraform:
        container:
            name: public.ecr.aws/hashicorp/terraform:1.10.5
            entrypoint: /usr/bin/env
            shell: sh
            shell_args:
                - -c
        envfile:
            exclude:
                - PATH
                - HOME

tasks:
    terraform:validate:
        context: terraform
        command:
            - |
                terraform -chdir=${TF_DIR} init -backend=false
                terraform -chdir=${TF_DIR} validate
        required:
            env:
                - TF_DIR
```

Then run:

- `TF_DIR=infra eirctl run task terraform:validate`

## Editing eirctl Configuration

When adding tasks to `eirctl.yaml`:

- Keep task names clear and namespaced, for example `go:lint`, `tf:plan`, `test:unit`, `build:image`.
- Use `description` for non-obvious tasks.
- Use `required` declarations for mandatory inputs.
- Prefer environment variables for values that vary by environment.
- Prefer variables for reusable template values.
- Use pipelines for ordered or parallel workflows.
- In the `eirctl` repo, prefer composing existing imported tasks and contexts rather than duplicating them in the root config.
- Run `eirctl validate` after editing.
- Run `eirctl list` to confirm the task or pipeline is visible.
- Use `eirctl show <name>` to confirm the final wiring.

If you change schema-related config or schema-producing code in the `eirctl` repo, use the existing repo task such as `eirctl generate_own_schema` instead of inventing a parallel command path.

## Running Commands

Before executing a command:

1. Check whether an equivalent `eirctl` task or pipeline exists.
2. In the `eirctl` repo, default to the existing build, test, lint, coverage, tidy, and schema tasks first.
3. If a task or pipeline exists, run it instead of the raw tool.
4. If no task exists, add one unless the user needs a one-off host diagnostic.
5. If the command mutates infrastructure, clusters, state, releases, or persistent resources, ask for confirmation unless the user has explicitly requested it.

When deciding whether an example is safe to run immediately, separate commands into these groups:

- Inspection-only: `eirctl --version`, `eirctl validate`, `eirctl list`, `eirctl show <name>`, `eirctl graph <pipeline-name>`.
- Repo-local but environment-dependent: tasks using container contexts, Docker socket access, network downloads, or required template variables.
- Credential-dependent: tasks such as `sonar:scanner:cli` that require secrets or service auth.
- Illustrative cross-project examples: examples such as Terraform workflows that show the intended pattern but are not guaranteed to exist in the current repo.

Use `--` to forward extra arguments when the task is designed to consume them:

- `eirctl run task lint:yaml -- docs/example.yaml`
- `eirctl run task sonar:scanner:cli -- -Dsonar.projectVersion=0.0.13`
- `eirctl run task terraform:plan -- -var-file=dev.tfvars`

Be explicit about what those examples mean:

- In this repo, `eirctl run task lint:yaml -- docs/example.yaml` is a valid example of argument forwarding, but the underlying task still lints `.` and appends `.Args`; it is not limited to only the forwarded file.
- In this repo, `eirctl run task sonar:scanner:cli -- ...` is a valid command shape, but it requires `SONAR_TOKEN` and usually project-specific scanner settings.
- `eirctl run task terraform:plan -- ...` is an illustrative example for projects that define Terraform tasks. Do not assume it exists in the current repo unless `eirctl list` or `eirctl show <name>` confirms it.

For repo task execution examples, state prerequisites up front:

- If a task uses a container context, mention that container runtime access and any required socket mounts must work first.
- If a task interpolates build metadata such as `.Version` or `.Revision`, mention that these values must be provided by the config, environment, or invoking workflow.
- If a task downloads modules or images, mention that network access and certificate trust must be available.

## CI and Reproducibility

For CI workflows:

- Prefer invoking `eirctl` tasks in CI instead of duplicating raw tool commands in CI YAML.
- Use `eirctl generate` when the project wants CI config generated from eirctl pipelines.
- Keep local and CI behavior aligned by using the same task names and contexts.
- Avoid installing language or toolchain dependencies differently in CI when the `eirctl` context already defines them.

## Safety

- Do not bypass security controls, approvals, or signing requirements.
- Do not run destructive or publishing tasks such as `terraform apply`, `terraform destroy`, `helm upgrade`, `kubectl delete`, `goreleaser`, or image pushes without explicit user approval.
- Do not expose secrets in task definitions, command output, or examples.
- Prefer synthetic placeholder values in documentation and tests.
- If a command needs a secret such as `SONAR_TOKEN`, state that prerequisite plainly and do not attempt to source or echo the secret.
- If a command failed because of host constraints such as Docker socket permissions or TLS trust, report that as an environment prerequisite rather than implying the command syntax is wrong.

## Agent Checklist

Before direct tool execution, confirm:

- Is there an `eirctl.yaml`?
- Has `eirctl validate` passed?
- Does `eirctl list` or `eirctl show <name>` already expose the needed task, pipeline, or context?
- In the `eirctl` repo, does an existing task such as `build:binary`, `test:unit`, `lints`, `tidy`, or `generate_own_schema` already cover the workflow?
- Is there an existing context that already pins the toolchain?
- Should a new task or context be added instead of running the command directly?
- Is this command inspection-only, environment-dependent, credential-dependent, or just illustrative for another project?
- If it is environment-dependent, have container runtime access, network access, certificate trust, and required template values been checked?
- If it is credential-dependent, has the user explicitly supplied the required secret out of band?
- If the change touches shared automation or schemas, have the relevant repo tasks been rerun?

If reproducibility matters, use `eirctl`.
