## Why

SonarCloud analysis initially failed on trusted `main` pushes because the containerized scanner could not create `/eirctl/.scannerwork`. After the official scanner action removed that local blocker, live run `29914871551` reached SonarQube Cloud but failed to load project settings for `Ensono_eirctl`, reported `NOT_FOUND` and a `NONEXISTENT` binding, and ended with “Not authorized or project not found.” The public organization still exposes the legacy `Ensono_taskctl` project, while the configured `Ensono_eirctl` key does not resolve and the existing `SONAR_TOKEN` value and authorization cannot be verified. Pull-request runs must still avoid exposing the credential to untrusted code. The first trusted pull-request design also proved unacceptable in live validation because checking out an untrusted revision in a privileged `workflow_run` context produced a high-severity CodeQL finding that the active ruleset correctly blocks. The repository also began without `CODEOWNERS`, so changes to executable workflows and `sonar-project.properties` required stronger mandatory review from the maintainers responsible for the CI trust boundary.

## What Changes

- Replace the failing container-based GitHub Actions scan path with the latest approved, immutable-SHA-pinned official SonarQube scan action and keep main-branch quality-gate enforcement.
- Reconcile the SonarQube Cloud project identity before live validation by importing and binding `Ensono/eirctl` or migrating the legacy `Ensono_taskctl` project to the exact `Ensono_eirctl` key, aligning its main branch to `main`, and replacing the repository `SONAR_TOKEN` value with a current credential supported by the organization plan. Prefer a project-scoped Scoped Organization Token where available because it grants only **Execute analysis**; otherwise use a personal access token with the minimum analysis authorization required by the Free plan.
- Add a default-branch-controlled trusted SonarCloud workflow that analyzes every pull-request revision, including fork-originated revisions, without executing pull-request-controlled scripts, actions, dependencies, containers, or binaries.
- Treat pull-request source and test reports as untrusted passive analysis inputs; validate their repository, pull request, workflow-run, run-attempt, artifact, and immutable commit provenance before scanning.
- Eliminate pull-request source checkout from the privileged analyzer. Materialize only allowlisted regular Go source blobs obtained through GitHub's API from the verified head repository and full commit SHA, with strict path, type, count, and size bounds and no Git metadata, scripts, actions, configuration, dependencies, containers, or binaries.
- Keep `SONAR_TOKEN` confined to the pinned scanner step, force trusted SonarCloud endpoint/project settings and explicit pull-request metadata, and prevent pull-request configuration from redirecting or overriding security-sensitive scanner settings.
- Extend structural workflow policy and tests to recognize only this narrowly constrained passive static-analysis topology, reject every privileged pull-request source checkout, and require the implementation to pass the active CodeQL security threshold without suppressing or dismissing an alert.
- Add `CODEOWNERS` entries assigning `.github/workflows/**`, `.github/CODEOWNERS`, and `/sonar-project.properties` to `@Ensono/digital-tools-maintainers`, and enable required code-owner review in the active `main` ruleset.
- Add the stable SonarCloud quality-gate check to the `main` ruleset after a successful live analysis establishes its exact check context.
- Resolve every action introduced or touched by this change to its latest stable release at implementation time, pin it by full commit SHA, retain a readable version comment, and keep all selected actions within the repository allow list.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secure-ci-workflows`: Require reliable trusted-branch and pull-request SonarCloud analysis, passive-data handling for untrusted revisions, protected scanner configuration, CODEOWNERS review, current immutable CI dependencies, and merge enforcement of the SonarCloud quality gate.

## Impact

- Affected workflows: `.github/workflows/pr.yml`, a new trusted SonarCloud workflow, and `.github/workflows/trusted-workflow-policy.yml` if its candidate allow list or validation path must include the new workflow.
- Affected policy and tests: `scripts/check-workflow-policy`, a protected GitHub-API source-materialization helper and its hostile-input fixtures, workflow-security fixtures/tests, immutable-dependency checks, CodeQL validation, and `docs/ci-security.md`.
- Affected ownership and settings: a new `.github/CODEOWNERS` file and the active `main-is-main` repository ruleset.
- Affected Sonar configuration and administration: `sonar-project.properties`, trusted scanner arguments/configuration, coverage and JUnit artifact handling, SonarQube Cloud project import/binding or key migration, main-branch alignment, and rotation plus authorization validation of the `SONAR_TOKEN` repository secret.
- Dependencies: latest stable allow-listed GitHub Actions pinned by full commit SHA, including `SonarSource/sonarqube-scan-action` and any GitHub-owned artifact or API helper actions used by the trusted topology. `actions/checkout` remains permitted only for protected base-branch code and is prohibited for pull-request source in the privileged analyzer.
- Operations: first verify that authenticated project settings load for the exact bound `Ensono_eirctl` project with the rotated credential; then run one trusted `main` push, one live same-repository PR, and one live fork PR to verify analysis association, token isolation, quality-gate reporting, and ruleset enforcement.
