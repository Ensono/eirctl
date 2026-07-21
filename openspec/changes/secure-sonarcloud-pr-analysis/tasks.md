## 1. Capture Current Security and Dependency Baselines

- [x] 1.1 Record the current `main-is-main` ruleset, selected-action allow list, SHA-pinning requirement, `SONAR_TOKEN` secret presence, and latest failing/skipped SonarCloud run URLs without exposing secret values.
- [x] 1.2 Query the latest stable releases and immutable commit SHAs for every action introduced or touched by this change, resolve annotated tags to commits, and record release/tag/SHA evidence for review.
- [x] 1.3 Confirm that each selected action is already allowed; if any is not allowed, stop and obtain allow-list approval before adding it.
- [x] 1.4 Add a focused verification fixture or documented probe that reproduces the current container `.scannerwork` permission failure and proves the replacement path no longer uses that failing container invocation in GitHub Actions.

## 2. Protect Security-Sensitive Configuration

- [x] 2.1 Create `.github/CODEOWNERS` entries for `/.github/CODEOWNERS`, `/.github/workflows/**`, and `/sonar-project.properties`, each owned by `@Ensono/digital-tools-maintainers`.
- [x] 2.2 Add automated validation that the required CODEOWNERS paths and owner remain present and that executable workflow and Sonar configuration changes cannot silently lose ownership coverage.
- [x] 2.3 Update CI security documentation to state that CODEOWNERS is merge governance and does not replace runtime isolation, provenance validation, or secret scoping.

## 3. Produce Bounded Untrusted Sonar Reports

- [x] 3.1 Update the Linux pull-request test path to upload a dedicated Sonar reports artifact containing only the expected Go coverage and JUnit report files, with a bounded retention period and deterministic name.
- [x] 3.2 Ensure the pull-request workflow retains read-only permissions, never references `SONAR_TOKEN`, and does not gain a protected environment or other privileged credential.
- [x] 3.3 Define and test the artifact contract for expected paths, regular-file types, maximum sizes, missing coverage, and rejection of symlinks, special files, traversal paths, or unexpected content.

## 4. Extend Trusted Workflow Policy

- [x] 4.1 Extend the structural workflow model to represent the trusted SonarCloud `workflow_run` topology, passive source/report materialization, scanner action, secret scope, caches, permissions, and execution after untrusted input is introduced.
- [x] 4.2 Require the expected upstream workflow/event, immutable PR identity, read-only permissions, no caches, no local actions, and `SONAR_TOKEN` only on the approved pinned scanner step before accepting the topology.
- [x] 4.3 Continue rejecting any shell command, build tool, package installation, container, binary, local action, alternate external action, or cache operation that can consume pull-request-controlled content in the privileged analyzer.
- [x] 4.4 Add table-driven policy tests for valid same-repository and fork analyzer structures plus bypasses using mutable refs, missing provenance checks, job/workflow-scoped secrets, untrusted scanner settings, post-materialization commands, caches, alternate actions, and equivalent YAML syntax.
- [x] 4.5 Update trusted candidate materialization and policy workflow fixtures so the new workflow and any trusted Sonar configuration are inspected as data by protected base-branch checker code.

## 5. Implement Trusted Pull-Request SonarCloud Analysis

- [x] 5.1 Add a default-branch-controlled workflow triggered on completion of `Lint and Test` pull-request runs, with explicit least-privilege permissions and per-pull-request/revision concurrency.
- [x] 5.2 Implement trusted provenance resolution that verifies workflow identity, event, base repository and branch, pull-request number, full head SHA, run ID, run attempt, and current analysis revision before any secret-bearing step.
- [x] 5.3 Download only the dedicated artifact from the verified upstream run/attempt, validate it against the bounded passive-data contract, and fail closed before scanning on any mismatch.
- [x] 5.4 After provenance validation and before any secret-bearing step, materialize the exact full PR head SHA with an immutable `persist-credentials: false` checkout into `analysis/source`; generate trusted scanner configuration in the separate `analysis` root before that checkout and invoke no post-materialization command or action other than the approved scanner.
- [x] 5.5 Remove, ignore, or replace pull-request-controlled scanner configuration and supply trusted endpoint, organization, project, source, test, report, PR, revision, and quality-gate settings at non-overridable precedence.
- [x] 5.6 Invoke only the latest stable allow-listed `SonarSource/sonarqube-scan-action`, pinned to its full commit SHA with a readable version comment, and scope `SONAR_TOKEN` to that scanner step only.
- [x] 5.7 Implement the specified missing-coverage behavior so a PR receives either explicit source-only analysis or a visible failed-preparation result rather than a silently skipped Sonar check.
- [x] 5.8 Add workflow-level assertions or tests proving that fork source, scripts, actions, dependencies, containers, and binaries are never executed and that no privileged cache is restored or saved.

## 6. Repair Trusted Main Analysis and Align Action Versions

- [x] 6.1 Replace the `main` SonarCloud container-task invocation with the same latest stable immutable-SHA-pinned official scan action used by the trusted PR analyzer.
- [x] 6.2 Preserve trusted report generation, full revision metadata, trusted `sonar-project.properties`, `SONAR_TOKEN` step scope, and `sonar.qualitygate.wait=true` for `main` pushes.
- [x] 6.3 Remove GitHub Actions-only Docker scanner environment settings that are no longer required, while leaving unrelated local task-runner behavior out of scope.
- [x] 6.4 Update other occurrences of action families touched by this change where required for consistent latest stable versions, full SHA pins, readable comments, and avoidance of immediate grouped Dependabot churn.
- [x] 6.5 Run the immutable CI dependency check and verify every introduced or updated action remains inside the repository selected-action policy.

## 7. Document and Validate the Trust Boundary

- [x] 7.1 Update `docs/ci-security.md` with the untrusted preparation/trusted passive-analysis data flow, provenance invariants, scanner parser risk, forced settings, secret scope, and rollback procedure.
- [x] 7.2 Document why ordinary secret-bearing PR jobs, privileged PR builds, same-repository-only scanning, and automatic analysis were rejected.
- [x] 7.3 Run focused policy unit tests, immutable-dependency validation, workflow security validation, YAML validation, and the repository's relevant Go tests; resolve every failure.
- [x] 7.4 Run `openspec validate secure-sonarcloud-pr-analysis` and confirm the implementation still satisfies every modified and added scenario.

## 8. Exercise Live Workflows and Enforce Repository Rules

- [ ] 8.1 Exercise a trusted `main` push and confirm reports upload, scanner completion, quality-gate waiting, correct project association, and absence of the `.scannerwork` permission failure.
- [ ] 8.2 Exercise a same-repository PR and confirm the exact head SHA, coverage, SonarCloud PR decoration, quality-gate result, and no secret exposure in the untrusted workflow.
- [ ] 8.3 Exercise a fork-originated PR with adversarial changes to `sonar-project.properties` and inert executable-looking files; confirm trusted settings win, no fork content executes, and SonarCloud decorates the exact fork revision.
- [ ] 8.4 Use the fork exercise to confirm and document the constrained immutable checkout's SonarCloud SCM/new-code fidelity and passive-only boundary, then rerun static policy validation for the selected form.
- [ ] 8.5 Update `main-is-main` to require code-owner review and verify workflow, CODEOWNERS, and Sonar property changes request `@Ensono/digital-tools-maintainers` approval.
- [ ] 8.6 Observe the stable external SonarCloud check context and integration ID from a successful live PR analysis, add that exact check to `main-is-main`, and verify a missing or failing quality gate blocks merge while a passing gate satisfies the rule.
- [ ] 8.7 Record final workflow URLs, ruleset state, action release/SHA evidence, and residual scanner-parser risk in the change verification notes without recording secret values.
