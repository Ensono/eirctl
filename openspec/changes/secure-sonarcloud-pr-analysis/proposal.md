## Why

SonarCloud analysis currently fails on trusted `main` pushes because the containerized scanner cannot create `/eirctl/.scannerwork`, while pull-request runs deliberately skip analysis to avoid exposing `SONAR_TOKEN` to untrusted code. The repository also has no `CODEOWNERS` file, so changes to executable workflows and `sonar-project.properties` do not receive mandatory review from the maintainers responsible for the CI trust boundary.

## What Changes

- Replace the failing container-based GitHub Actions scan path with the latest approved, immutable-SHA-pinned official SonarQube scan action and keep main-branch quality-gate enforcement.
- Add a default-branch-controlled trusted SonarCloud workflow that analyzes every pull-request revision, including fork-originated revisions, without executing pull-request-controlled scripts, actions, dependencies, containers, or binaries.
- Treat pull-request source and test reports as untrusted passive analysis inputs; validate their repository, pull request, workflow-run, run-attempt, artifact, and immutable commit provenance before scanning, then materialize the verified full SHA only through an isolated `persist-credentials: false` checkout that no pull-request-controlled command can execute.
- Keep `SONAR_TOKEN` confined to the pinned scanner step, force trusted SonarCloud endpoint/project settings and explicit pull-request metadata, and prevent pull-request configuration from redirecting or overriding security-sensitive scanner settings.
- Extend structural workflow policy and tests to recognize only this narrowly constrained passive static-analysis topology while continuing to reject privileged execution of pull-request-controlled content.
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
- Affected policy and tests: `scripts/check-workflow-policy`, workflow-security fixtures/tests, immutable-dependency checks, and `docs/ci-security.md`.
- Affected ownership and settings: a new `.github/CODEOWNERS` file and the active `main-is-main` repository ruleset.
- Affected Sonar configuration: `sonar-project.properties`, trusted scanner arguments/configuration, coverage and JUnit artifact handling, and the existing `SONAR_TOKEN` repository secret.
- Dependencies: latest stable allow-listed GitHub Actions pinned by full commit SHA, including `SonarSource/sonarqube-scan-action` and any GitHub-owned artifact, checkout, or API helper actions used by the trusted topology.
- Operations: one live same-repository PR, one live fork PR, and one trusted `main` push are required to verify analysis association, token isolation, quality-gate reporting, and ruleset enforcement.
