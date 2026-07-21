## Context

The `Lint and Test` workflow serves both `pull_request` and trusted `push` events. Its current SonarCloud job is restricted to a push on `main`, receives `SONAR_TOKEN`, and invokes the repository's container-backed `sonar:scanner:cli` task. Recent trusted runs reach the scanner with the secret present but fail before analysis because the image's non-root UID cannot create `.scannerwork` in the bind-mounted GitHub workspace. Pull-request runs skip the job by design.

This repository already separates untrusted execution from privileged publication, pins actions by full commit SHA, restricts actions to a selected allow list, and structurally validates privileged workflow topologies. GitHub treats `workflow_run` as privileged because it executes default-branch workflow code and can access repository secrets. GitHub also warns that checking out and executing pull-request-controlled content in such a workflow can expose secrets or poison shared state.

SonarCloud analysis differs from a build: the scanner must parse pull-request-controlled source while authenticating to SonarCloud, but it does not need to execute repository scripts, install dependencies, restore caches, run generated binaries, or load repository-local actions. This design makes that narrow parser boundary explicit rather than treating a privileged checkout as generally safe.

The repository currently has no `CODEOWNERS` file. The `main-is-main` ruleset requires one approving review but has `require_code_owner_review` disabled. `@Ensono/digital-tools-maintainers` has maintain access and is the appropriate owner for CI trust-boundary files. CODEOWNERS strengthens merge governance but does not prevent a workflow from running before review, so it does not replace runtime isolation.

## Goals / Non-Goals

**Goals:**

- Restore successful SonarCloud analysis and quality-gate waiting for trusted `main` pushes.
- Analyze every pull-request revision, including fork-originated revisions, without giving pull-request code direct access to `SONAR_TOKEN`.
- Preserve Go coverage and JUnit report ingestion, which rules out SonarCloud automatic analysis.
- Run trusted scanner orchestration only from the protected default branch.
- Establish and validate immutable provenance between the initiating PR workflow run, exact pull-request revision, reports, and SonarCloud analysis.
- Ensure pull-request-controlled scanner configuration cannot redirect credentials or override the project identity.
- Keep new and touched Actions allow-listed, at the latest stable release selected at implementation time, and pinned by full commit SHA.
- Require code-owner approval for executable workflow files, `CODEOWNERS` itself, and `sonar-project.properties`.
- Make the SonarCloud quality gate a required merge check once its stable check context is observed.

**Non-Goals:**

- Giving fork or pull-request workflows direct access to repository secrets.
- Running pull-request build commands, scripts, local actions, package installation, containers, or binaries in the trusted SonarCloud workflow.
- Replacing existing lint, test, report, debug-build, release, or deployment trust domains.
- Enabling SonarCloud automatic analysis, because it does not ingest Go coverage reports.
- Broadly updating unrelated dependencies or all repository ownership rules.
- Treating CODEOWNERS approval as a substitute for least privilege and data/execution separation.

## Decisions

### 1. Use separate untrusted preparation and trusted analysis workflows

The existing `pull_request` workflow remains the only place where pull-request code is built and tested. It has no `SONAR_TOKEN` and uploads an inert Sonar report artifact for the Linux test leg containing only the expected Go coverage and JUnit report files.

A new default-branch workflow is triggered by `workflow_run` completion of `Lint and Test`. It obtains protected credentials because its workflow definition comes from `main`, but it performs only trusted orchestration and static analysis. It runs for pull-request events regardless of whether the head repository is the base repository or a fork. A pull request that cannot produce a usable report may still receive source analysis with an explicitly handled missing-coverage result; it must not silently disappear.

Alternatives considered:

- Put `SONAR_TOKEN` in the ordinary PR workflow: rejected because fork workflows do not receive the secret and pull-request code could exfiltrate it.
- Use `pull_request_target` and build the PR: rejected because privileged untrusted execution violates GitHub guidance and repository policy.
- Scan only same-repository PRs: rejected because it does not satisfy every-PR coverage.
- Use SonarCloud automatic analysis: rejected because automatic analysis does not support the Go coverage reports required by this project.

### 2. Resolve and validate immutable upstream provenance before scanning

Trusted code resolves the originating workflow run through the GitHub API and verifies all of the following before any secret-bearing step:

- workflow identity is the expected `Lint and Test` workflow;
- upstream event is `pull_request` and base repository is `Ensono/eirctl`;
- base branch is `main`;
- pull-request number exists and belongs to this repository;
- the recorded head SHA is a full immutable SHA and matches the PR revision being analyzed;
- the report artifact belongs to the exact upstream run ID and run attempt;
- artifact name, expected files, extraction paths, file types, and bounded sizes match the passive-data contract.

The workflow uses PR-number/revision concurrency so a newer revision supersedes stale analysis without conflating artifacts between revisions. Provenance failures fail closed and never expose `SONAR_TOKEN`.

### 3. Materialize PR source and reports as passive data, not an executable workspace

The trusted workflow first verifies the upstream workflow run, pull request, full head SHA, run attempt, and report artifact. It then uses an immutable full-SHA `actions/checkout` with `persist-credentials: false` to materialize exactly that revision into `analysis/source`. This is deliberately a constrained source-materialization primitive, not a general privileged checkout: no branch, tag, mutable ref, event value, stored credential, or default workspace path is permitted.

Before source materialization, trusted default-branch orchestration creates the scanner configuration in the isolated `analysis` root and validates the passive report artifact. The scanner therefore uses `analysis` as its project base and `analysis/source` only as source data; a pull-request-controlled `sonar-project.properties` cannot become the loaded configuration file. After source materialization, only the pinned official scanner action may consume the isolated source and report data. No shell command, cache, local action, container, package manager, dependency hook, binary, or alternate external action may run.

The workflow does not restore or save caches and does not use the analysis directory as a source of workflow code, scripts, actions, tools, dependency metadata, containers, or commands. Coverage and JUnit artifacts are parser inputs only. The structural policy gains an explicit model for this one scanner topology instead of a generic exception for privileged untrusted checkout.

### 4. Use trusted scanner configuration and command-line security invariants

The official `SonarSource/sonarqube-scan-action` replaces the container-backed scanner in GitHub Actions. The main job and trusted PR job use the same pinned action and trusted property set. The action avoids the container UID mismatch and currently bundles a newer supported scanner runtime.

The trusted workflow writes a configuration file in the trusted `analysis` root, uses that root as the scanner project base, and ignores the PR revision's `analysis/source/sonar-project.properties`. Security-sensitive values are supplied from trusted default-branch configuration or highest-precedence scanner arguments, including:

- `sonar.host.url=https://sonarcloud.io`;
- `sonar.projectKey=Ensono_eirctl`;
- `sonar.organization=ensono`;
- source, test, coverage, and JUnit paths;
- `sonar.pullrequest.key`, `sonar.pullrequest.branch`, and `sonar.pullrequest.base`;
- `sonar.scm.revision` set to the verified immutable SHA;
- `sonar.qualitygate.wait=true`.

`SONAR_TOKEN` is scoped to the scanner step rather than workflow or job scope. No earlier validation/materialization step receives it. The `GITHUB_TOKEN` remains read-only and is supplied to the scanner only if SonarCloud integration requires it.

### 5. Encode the narrow scanner boundary in structural policy

The existing policy continues to reject privileged workflows that execute pull-request-controlled content. The only accepted trusted PR analysis path must structurally prove:

- expected `workflow_run` trigger and upstream workflow;
- read-only job permissions with only the minimum artifact/API read permission;
- immutable full-SHA revision and provenance validation before scan;
- isolated `persist-credentials: false` checkout into `analysis/source` only after that validation;
- no caches;
- no local actions;
- no shell/build/package/container execution after untrusted materialization;
- exactly the approved, immutable-SHA-pinned Sonar action consumes the passive source;
- secret reference appears only on the approved scan step;
- trusted endpoint and project identity are explicit.

Tests cover bypass attempts, including a PR-controlled scanner configuration, mutable refs, alternate actions, commands after materialization, cache use, environment-wide secrets, missing provenance checks, and YAML syntax variants.

### 6. Add CODEOWNERS and activate code-owner review

Create `.github/CODEOWNERS` with explicit entries for:

- `/.github/CODEOWNERS`;
- `/.github/workflows/**`;
- `/sonar-project.properties`.

Each entry names `@Ensono/digital-tools-maintainers`. The repository's active `main-is-main` ruleset is updated to require code-owner review while retaining the existing general approval and required checks. Protecting `CODEOWNERS` itself prevents an unreviewed ownership-removal change from bypassing the intended review path.

### 7. Pin current, allow-listed Actions and avoid immediate version churn

At implementation time, query each introduced or touched action's latest stable release, resolve annotated tags to the underlying commit where necessary, pin the full commit SHA, and retain a readable release comment. Update other occurrences of the same action in the touched workflow set when needed for consistency and to avoid immediate Dependabot follow-up PRs.

The selected-action policy is checked before introducing an action. `SonarSource/sonarqube-scan-action` and GitHub-owned `actions/*` are currently allow-listed; no new third-party action is added unless it is explicitly approved first. The implementation records release/tag/SHA evidence and runs the immutable-dependency validator.

### 8. Require the external SonarCloud quality-gate check

The trusted workflow passes explicit PR metadata and revision so SonarCloud associates analysis with the pull-request head SHA and publishes its stable external check. After a live successful analysis confirms the exact check context and integration, update `main-is-main` to require that SonarCloud check alongside `Lint` and `Test (Linux)`. The GitHub Actions `workflow_run` job itself is not used as the required PR-head check because it executes against the default-branch workflow context.

## Risks / Trade-offs

- **[Scanner parses attacker-controlled source while holding a token]** → Use only the latest pinned official scanner, force trusted endpoint/project settings, remove untrusted scanner configuration, expose the token only to the scan step, prohibit all repository execution, and document this as the sole narrow parser trust boundary.
- **[A scanner or analyzer vulnerability could turn passive input into execution]** → Keep scanner dependencies current, avoid caches and write credentials, minimize token scope, and retain CODEOWNERS review for scanner/configuration changes.
- **[A malicious artifact could exploit extraction or parser behavior]** → Download only the exact run artifact, use current GitHub-owned artifact handling, validate paths/types/sizes, reject symlinks or unexpected files, and keep artifacts outside trusted code directories.
- **[Immutable checkout materializes attacker-controlled source in a privileged workflow]** → Require verified full-SHA provenance before checkout, `persist-credentials: false`, an isolated analysis path, no post-materialization command or cache, and the pinned scanner as the sole source consumer; validate SonarCloud SCM/new-code fidelity in a live fork PR.
- **[A completed upstream run may refer to a superseded PR revision]** → Bind analysis to the recorded SHA and use per-PR concurrency; newer revisions supersede stale runs without mixing provenance.
- **[Ruleset update can block all merges if the observed check name is wrong]** → Add the required check only after a successful live PR scan establishes the exact context and integration ID; document rollback.
- **[CODEOWNERS does not prevent pre-review workflow execution]** → Continue to rely on default-branch workflow code, secret scoping, provenance validation, and passive-data isolation.
- **[Latest releases can move between proposal and implementation]** → Resolve and record versions immediately before editing, then validate SHA pins and Dependabot configuration.

## Migration Plan

1. Capture baseline failures/skips and current ruleset/action-policy state.
2. Add CODEOWNERS and policy/spec tests without enabling a new secret path.
3. Update the untrusted PR workflow to publish the bounded Sonar reports artifact.
4. Add the trusted PR analysis workflow and structural policy enforcement.
5. Replace the trusted-main container scan with the same official pinned scanner action.
6. Run static workflow, policy, immutable-dependency, unit, and OpenSpec validation.
7. Exercise a same-repository PR, a fork PR, and a trusted `main` push; inspect source revision, coverage, quality-gate result, PR decoration, and logs for secret isolation.
8. Enable required code-owner review in `main-is-main`.
9. After observing the external SonarCloud check identity, add it to required status checks.

Rollback removes the SonarCloud required check first, disables the trusted PR workflow, and restores main analysis only if the previous path is known to work. CODEOWNERS and the strengthened privileged-workflow policy remain safe to retain independently.

## Open Questions

- Confirm the exact SonarCloud external quality-gate check context and integration ID from the first successful live PR analysis before changing the ruleset.
- Confirm through a live fork PR that the constrained immutable checkout provides adequate SonarCloud SCM/new-code fidelity while the static policy continues to enforce its passive-only boundary.
- Confirm whether missing coverage after a failed upstream test should produce source-only analysis or an explicit failed Sonar preparation check; it must not silently skip the PR.
