## Context

The `Lint and Test` workflow serves both `pull_request` and trusted `push` events. Its original SonarCloud job was restricted to a push on `main`, received `SONAR_TOKEN`, and invoked the repository's container-backed `sonar:scanner:cli` task. Trusted runs reached the scanner but failed because the image's non-root UID could not create `.scannerwork` in the bind-mounted GitHub workspace. Pull-request runs skipped the job to avoid exposing the secret. After the official scanner action removed the workspace blocker, live run `29914871551` reached SonarQube Cloud but failed to load settings for `Ensono_eirctl`, reported `NOT_FOUND` and a `NONEXISTENT` binding, and ended with “Not authorized or project not found.” Organization `ensono` and project key `Ensono_eirctl` are confirmed fixed identifiers. The unauthenticated public API exposes the organization and legacy `Ensono_taskctl` project but returns `404` for `Ensono_eirctl`; that public result cannot distinguish an inaccessible private project from missing configuration. The existing secret's owner, expiry, validity, and project authorization are not established, and the current maintainer has requested but not yet received SonarQube Cloud administration and token-generation access.

This repository already separates untrusted execution from privileged publication, pins actions by full commit SHA, restricts actions to a selected allow list, and structurally validates privileged workflow topologies. GitHub treats `workflow_run` as privileged because it executes default-branch workflow code and can access repository secrets.

The first implementation of this change verified a full pull-request head SHA, checked it out with `persist-credentials: false` into an isolated directory, and permitted only the pinned Sonar scanner afterward. Live PR validation demonstrated that this exception is not acceptable: CodeQL reported `actions/untrusted-checkout/high` because a privileged `workflow_run` checked out untrusted source before an external action. The active `main-is-main` code-scanning rule correctly blocks that high-severity finding. The design must therefore remove privileged pull-request checkout rather than dismiss, suppress, or bypass the alert.

SonarCloud analysis still requires the scanner to parse attacker-controlled Go source and reports while authenticating. The remaining narrow trust boundary must minimize the materialized input surface, eliminate repository execution surfaces, and keep credentials and configuration outside pull-request control.

At the proposal baseline, the repository had no `CODEOWNERS` file and code-owner review was not required. `@Ensono/digital-tools-maintainers` is the appropriate owner for CI trust-boundary files. CODEOWNERS strengthens merge governance but does not prevent pre-review workflow execution and cannot replace runtime isolation.

## Goals / Non-Goals

**Goals:**

- Restore successful SonarCloud analysis and quality-gate waiting for trusted `main` pushes.
- Establish one canonical SonarQube Cloud project bound to `Ensono/eirctl` with organization `ensono`, project key `Ensono_eirctl`, and main branch `main` before live analysis proceeds.
- Replace the existing repository secret value with a current, least-privilege analysis credential whose owner, scope, and expiry are operationally maintained.
- Analyze every pull-request revision, including fork-originated revisions, without giving pull-request code direct access to `SONAR_TOKEN`.
- Preserve Go coverage and JUnit report ingestion.
- Run trusted scanner orchestration only from the protected default branch.
- Eliminate all privileged checkout or Git materialization of pull-request source.
- Establish immutable provenance between the initiating PR workflow run, current pull request, head repository, full head SHA, reports, Git tree, selected source blobs, and SonarCloud analysis.
- Materialize only bounded, non-executable regular Go source files and never materialize pull-request Git metadata, workflows, scripts, local actions, scanner configuration, dependencies, containers, or binaries.
- Ensure pull-request-controlled scanner configuration cannot redirect credentials or override project identity.
- Pass the active CodeQL security threshold without dismissing or suppressing a finding.
- Keep new and touched Actions allow-listed, current at implementation time, and pinned by full commit SHA.
- Require code-owner approval for executable workflows, `CODEOWNERS` itself, and `sonar-project.properties`.
- Make the SonarCloud quality gate a required merge check after its stable identity is observed.

**Non-Goals:**

- Giving fork or pull-request workflows direct access to repository secrets.
- Checking out pull-request source in any privileged workflow, even by full SHA with stored credentials disabled.
- Running pull-request build commands, scripts, local actions, package installation, containers, dependencies, or binaries in the trusted SonarCloud workflow.
- Generically extracting an attacker-controlled source archive in the privileged workflow.
- Replacing existing lint, test, report, debug-build, release, or deployment trust domains.
- Enabling SonarCloud automatic analysis, because it does not ingest the required Go coverage reports.
- Preserving full Git checkout metadata at the cost of the privileged-workflow boundary.
- Treating CODEOWNERS approval or a security-alert dismissal as runtime authorization.

## Decisions

### 1. Use separate untrusted preparation and trusted analysis workflows

The existing `pull_request` workflow remains the only place where pull-request code is built and tested. It has no `SONAR_TOKEN` and uploads a dedicated inert report artifact containing only the expected Go coverage and JUnit files.

A protected-default-branch workflow is triggered by `workflow_run` completion of `Lint and Test`. It performs trusted orchestration and passive static analysis for same-repository and fork pull requests. A missing or invalid report produces a visible failed-preparation result rather than a silently absent Sonar check.

Alternatives rejected:

- An ordinary secret-bearing PR job exposes the token to pull-request code and does not work safely for forks.
- `pull_request_target` or another privileged PR build executes attacker-controlled content.
- Same-repository-only scanning omits fork pull requests.
- SonarCloud automatic analysis does not ingest the required reports.

### 2. Resolve immutable run, PR, artifact, repository, tree, and blob provenance

Before any step receives `SONAR_TOKEN`, protected code verifies:

- the upstream workflow is the expected `Lint and Test` workflow and event is `pull_request`;
- the base repository is `Ensono/eirctl` and base branch is `main`;
- the pull-request number is valid and still resolves through the base repository API;
- the head repository identity matches the current pull request, including fork identity;
- the head SHA is a full immutable commit SHA and still matches the current PR revision;
- the run ID and run attempt match the event and report artifact;
- exactly one unexpired report artifact has the deterministic expected name;
- the report artifact contains only the bounded regular coverage and JUnit files;
- the Git commit API in the verified head repository resolves the exact full SHA;
- the commit's tree identity matches a complete, non-truncated recursive Git tree response;
- each selected source blob response matches the tree-recorded blob SHA and size.

Per-PR/revision concurrency cancels stale analysis. The source materializer performs a final current-head check before it exits so a revision changed during API retrieval fails closed. A later race is handled by concurrency cancellation; artifacts and source identities are never mixed across revisions.

### 3. Materialize source through the Git Data API, never through checkout or a generic archive

The analyzer checks out protected `main` helper code only. It never passes pull-request repository or revision data to `actions/checkout`, `git checkout`, `git fetch`, `gh pr checkout`, or an equivalent source materializer.

A protected helper uses the GitHub Git Data API against the verified head repository:

1. Resolve the exact commit and root tree from the full head SHA.
2. Request the recursive tree and reject a truncated response.
3. Validate every tree path as a canonical relative slash-separated path with no absolute prefix, `.` or `..` segment, backslash, control character, duplicate normalized value, or excessive length.
4. Permit tree entries and regular blob modes for inspection, but reject symlinks, submodules, special modes, and unknown entry types.
5. Select only regular files ending in `.go`; all workflows, scripts, local actions, project properties, module/dependency metadata, container definitions, generated executables, archives, and other non-Go content remain unmaterialized.
6. Enforce reviewed protected constants for maximum tree entries, selected Go files, path length, per-file decoded bytes, and aggregate decoded bytes. Implementation records the current repository baseline and chooses the smallest practical bounds with documented headroom.
7. Fetch each selected blob by its tree-recorded immutable blob SHA, require the expected API encoding and identity, verify decoded length against both the tree and blob response, and reject a changed or missing value.
8. Create a fresh isolated `analysis/source` root, create parent directories without following links, and write each file once with exclusive creation and mode `0644`.
9. Revalidate the current PR head SHA before returning success.

The helper uses protected base-branch code and the Go standard library. Its GitHub token has only `contents: read`; `SONAR_TOKEN` is absent. The pinned Go setup disables dependency caching. A GitHub-hosted ephemeral runner is used, and no privileged cache is restored or saved.

This design intentionally does not preserve `.git`. Explicit Sonar pull-request metadata and `sonar.scm.revision` bind analysis to the verified revision. Live same-repository and fork exercises must prove that PR decoration and new-code behavior remain acceptable. If they do not, implementation pauses; it does not reintroduce privileged checkout.

Alternatives rejected:

- An immutable `actions/checkout` still triggers the high-severity CodeQL rule and leaves an unnecessarily broad repository surface.
- A source artifact produced by the untrusted workflow is harder to bind independently to the exact Git tree.
- A GitHub-generated tarball still requires a general archive parser and extractor against attacker-controlled paths and entry types.
- Materializing the complete repository exposes scripts, actions, scanner configuration, dependency hooks, containers, binaries, and unrelated parser inputs that Sonar does not need for Go analysis.

### 4. Keep scanner configuration, runtime selection, and credentials trusted

Protected orchestration creates scanner configuration in `analysis`, outside `analysis/source`, before source materialization. The scanner project base is `analysis`; the pull-request copy of `sonar-project.properties` is never materialized.

Trusted configuration or highest-precedence arguments force:

- `sonar.host.url=https://sonarcloud.io`;
- `sonar.projectKey=Ensono_eirctl`;
- `sonar.organization=ensono`;
- Go source, test, coverage, and JUnit paths;
- `sonar.pullrequest.key`, `sonar.pullrequest.branch`, and `sonar.pullrequest.base`;
- `sonar.scm.revision` set to the verified immutable SHA;
- `sonar.qualitygate.wait=true`.

Organization `ensono` and project key `Ensono_eirctl` are fixed inputs and are not migration choices. Before live scanning, an authorized `ensono` administrator verifies through authenticated SonarQube Cloud access that the existing canonical project is bound to `Ensono/eirctl` and uses `main` as its main branch, repairing or provisioning that binding under the same fixed identity if required. The public `Ensono_taskctl` project is recorded only as historical public evidence; it is not a substitute key or an assumed migration source. No implementation step renames the fixed project key, selects an alternate key, or creates a blind duplicate.

The official `SonarSource/sonarqube-scan-action` is pinned by full action commit SHA. The workflow explicitly fixes the scanner CLI version and approved Sonar binaries URL exposed by that action and keeps signature verification enabled. The policy rejects an omitted or mutable scanner version, an alternate binaries URL, or disabled signature verification.

The repository secret remains named `SONAR_TOKEN`, but its value is rotated before live validation after an authorized maintainer can generate the supported credential and update the repository secret. On Team or higher plans, the credential is a project-scoped Scoped Organization Token granting only **Execute analysis**. On the Free plan, it is a personal access token from a maintained identity with only the authorization required to analyze the canonical project. The credential owner and expiry are recorded in the team's secret-management process rather than in the repository. While access is pending, no token value is retrieved, copied into diagnostics, or replaced through an unauthorized workaround.

`SONAR_TOKEN` exists only in the scanner step environment. No provenance, API retrieval, report validation, source-materialization, or token-bearing diagnostic step receives it, and logs never print its value. A successful project-settings load and analysis against the exact canonical key is the credential authorization check. The scanner receives no write-capable GitHub token. The trusted `main` job uses the same reviewed scanner selection but continues to analyze its trusted checkout with full report and revision metadata. The superseded credential is revoked after the replacement succeeds, or immediately if compromise is suspected.

### 5. Permit only the exact passive analyzer topology

The structural policy requires:

- the expected `workflow_run` trigger and upstream workflow;
- exact read-only permissions;
- protected base-branch helper code;
- immutable run, PR, head-repository, head-SHA, report, tree, and blob validation;
- no pull-request checkout mechanism;
- no generic untrusted source archive extraction;
- a bounded Git Data API source helper that materializes only non-executable regular `.go` files;
- no caches or local actions;
- no shell, build, package, dependency, container, binary, or alternate action after source materialization;
- exactly the approved immutable scanner as the final step;
- scanner-step-only `SONAR_TOKEN`;
- explicit trusted endpoint, project identity, scanner version, binaries URL, signature verification, PR metadata, and revision.

Hostile fixtures cover mutable and derived refs, checkout aliases, missing provenance checks, forged head repositories, truncated trees, symlinks, submodules, unsafe or duplicate paths, wrong blob identities, size/count bypasses, job/workflow-scoped secrets, untrusted scanner settings, post-materialization commands, caches, alternate actions, and equivalent YAML syntax.

Acceptance includes the repository CodeQL workflow. A new untrusted-checkout or equivalent high-severity alert is a design failure. Dismissal, suppression, ruleset bypass, or threshold reduction is not an accepted mitigation.

### 6. Add CODEOWNERS and activate code-owner review

`.github/CODEOWNERS` explicitly assigns `/.github/CODEOWNERS`, `/.github/workflows/**`, and `/sonar-project.properties` to `@Ensono/digital-tools-maintainers`. The active `main-is-main` ruleset requires code-owner review while retaining general approval and required checks. Runtime controls remain effective before review.

### 7. Pin current, allow-listed Actions and avoid immediate version churn

Each introduced or touched action is resolved to its latest stable release at implementation time, including annotated-tag dereferencing. Workflows retain a readable version comment and full commit SHA. Other occurrences of touched action families are aligned where necessary to avoid immediate grouped Dependabot churn.

No new third-party action is introduced without selected-action approval. Immutable-dependency validation covers the scanner action, explicit scanner CLI selection, GitHub-owned actions, and any downloaded runtime verification.

### 8. Require the external SonarCloud quality-gate check

The analyzer supplies explicit PR metadata and revision so SonarCloud associates analysis with the PR head and publishes its stable external check. After a successful same-repository and fork analysis establishes the exact context and integration ID, `main-is-main` requires that external check alongside `Lint` and `Test (Linux)`. The default-branch `workflow_run` job context is not substituted for the external PR-head check.

## Risks / Trade-offs

- **[Unauthenticated public metadata is mistaken for authoritative project state]** → Treat `ensono` and `Ensono_eirctl` as fixed, require an authorized administrator to verify the canonical project's binding and `main` branch through authenticated access, and prohibit alternate keys or blind duplicates.
- **[The stored credential is expired, unauthorized, owner-dependent, or broader than analysis requires]** → Rotate it before live validation, use a project-scoped Scoped Organization Token granting only **Execute analysis** where the plan supports one, otherwise use a minimally authorized maintained-identity token, record owner and expiry out of band, validate through a real scan, and revoke the superseded credential.
- **[The scanner parses attacker-controlled Go source and reports while holding a token]** → Minimize inputs to bounded regular Go files and two bounded reports, use the latest reviewed pinned scanner/action, verify the scanner distribution, force trusted settings, expose the token only to the final step, and document this as the sole parser trust boundary.
- **[A scanner vulnerability turns passive input into execution]** → Use an ephemeral runner, no Git metadata or repository execution surface, no caches or credentials, non-executable files, minimal token scope, current dependencies, and CODEOWNERS review.
- **[The source helper mishandles malicious Git paths or modes]** → Avoid archive extraction, use standard-library API parsing, canonicalize and bound paths, reject symlinks/submodules/special modes and duplicates, use exclusive non-link-following writes, and add hostile table-driven tests.
- **[GitHub API tree retrieval is truncated, unavailable, or rate-limited]** → Require a complete response, use the read-only token, bound requests, retry only safe transient failures, and fail visibly before scanning rather than falling back to checkout or an untrusted archive.
- **[A fork changes or disappears during analysis]** → Bind every request to the verified head repository and immutable commit/blob identities; fail closed if the repository or object is unavailable.
- **[No `.git` metadata reduces blame or new-code fidelity]** → Supply explicit PR and revision metadata and verify same-repository and fork behavior live. Do not weaken the boundary if fidelity is inadequate.
- **[A newer PR revision supersedes analysis]** → Recheck the current head at the end of materialization and use per-PR/revision concurrency cancellation.
- **[CodeQL behavior changes]** → Keep an explicit no-PR-checkout policy and require security scanning to pass; do not encode query evasion as a control.
- **[The required Sonar check identity is configured incorrectly]** → Add it only after a successful live analysis establishes exact context and integration ID; document rollback.
- **[CODEOWNERS does not prevent pre-review execution]** → Continue relying on protected workflow code, least privilege, immutable provenance, bounded materialization, and secret scoping.

## Migration Plan

1. Retain the completed baseline, CODEOWNERS, report-artifact contract, trusted-main scanner repair, and action pin evidence.
2. Reopen analyzer policy, implementation, documentation, and validation tasks affected by the rejected checkout design.
3. Add and test the protected Git Data API source-materialization helper and conservative baseline-derived bounds.
4. Replace pull-request source checkout with the helper and remove the checkout exception from structural policy.
5. Require explicit scanner CLI version, approved binaries URL, and signature verification.
6. Run hostile helper fixtures, policy unit tests, immutable-dependency checks, workflow/YAML validation, full relevant Go tests, CodeQL, and OpenSpec validation.
7. Push a new PR revision and confirm that the previous high-severity CodeQL finding is absent before seeking merge.
8. Record organization `ensono` and project key `Ensono_eirctl` as fixed, preserve the unauthenticated public evidence, and wait for the requested SonarQube Cloud administration and token-generation access rather than changing identity or creating a duplicate.
9. After access is granted, verify the canonical project's binding to `Ensono/eirctl` and main branch `main`, determine the organization plan, generate the supported least-privilege analysis credential, update the repository `SONAR_TOKEN` secret without exposing its value, and record its owner and expiry in the team's secret-management process.
10. Exercise a trusted `main` push first. Require successful project-settings loading, analysis submission, coverage ingestion, quality-gate waiting, and absence of both the former `.scannerwork` failure and the `NOT_FOUND`/`NONEXISTENT` failure before revoking the superseded credential.
11. Exercise a same-repository PR and an adversarial fork PR. Verify exact revision, coverage, PR decoration, new-code behavior, token isolation, and no source execution.
12. Confirm code-owner enforcement and add the observed external SonarCloud check identity to `main-is-main`.
13. Record final workflow URLs, canonical project and binding state, credential type/owner/expiry without its value, ruleset state, release/SHA evidence, API bounds, and residual parser risk.

Rollback removes the external SonarCloud required check first, disables the trusted PR analyzer, and preserves CODEOWNERS plus the stricter no-privileged-checkout policy. It does not restore a project identity or credential already shown to be invalid; administrators either correct the canonical binding and secret or leave analysis disabled. The rejected checkout design is not a rollback target.

## Open Questions

- Confirm through authorized SonarQube Cloud access that the fixed `Ensono_eirctl` project is bound to `Ensono/eirctl`, uses `main` as its main branch, and is analyzable by the replacement credential.
- Confirm the `ensono` organization plan so operations select a project-scoped Scoped Organization Token on Team or higher, or a minimally authorized personal access token on Free.
- Confirm through live same-repository and fork PRs that an allowlisted source tree without `.git` provides acceptable SonarCloud PR decoration and new-code behavior.
- Confirm the exact SonarCloud external quality-gate context and integration ID before changing required checks.
