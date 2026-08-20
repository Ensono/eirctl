## Context

The debug-build flow currently has three files but only two effective trust domains. `debug-build-request.yml` safely authorizes an exact `/build-debug` comment and dispatches `debug-build.yml`; the dispatched workflow then checks out and executes the validated pull-request head. Although that builder has only `contents: read`, disables checkout credential persistence, and disables `setup-go` caching, GitHub grants `workflow_dispatch` a token that can write the default-branch cache scope. Pull-request-controlled code can use the cache service directly, so CodeQL 2.26.3 reports alert 23 at the build step.

This topology resulted from a genuine earlier failure. Commit `663dfe10465e` used a `GITHUB_TOKEN` broker to remove and re-add `build-debug`, expecting `pull_request:labeled` to start the builder. GitHub suppresses that token-generated activity from starting another workflow. Static analysis closed the earlier cache alert, but the command path did not work end to end. Commit `d22ee989cecd` restored reliable startup through `workflow_dispatch`, but also restored default-branch cache-write authority.

GitHub changed the relevant platform boundary on June 26, 2026. On GitHub.com, low-trust events that execute in the default-branch context, including `issue_comment`, receive read-only access to the default-branch cache; `workflow_dispatch` remains one of the explicitly write-capable events. The current CodeQL cache-poisoning library models the same event set and states that reusable jobs inherit the caller's trigger. This repository can therefore keep the reliable comment event, authorize the request in trusted default-branch code, and invoke a `workflow_call` builder in the same run without adding a GitHub App or PAT.

The affected parties are maintainers requesting and publishing debug builds, contributors whose revisions are built, and repository administrators responsible for workflow policy, CodeQL, and the protected `debug-release` environment.

## Goals / Non-Goals

**Goals:**

- Close CodeQL alert 23 by ensuring pull-request code never executes in a run with default-branch cache-write authority.
- Preserve the exact `/build-debug` maintainer experience without a PAT, GitHub App, or manual-label prerequisite.
- Keep authorization and reusable-workflow definitions on the protected default branch while executing only the validated current pull-request head SHA.
- Make the final build identity and provenance originate from a clean trusted job that does not execute downloaded binaries or pull-request content.
- Preserve the separate protected publication boundary and update it for the request workflow's run identity.
- Encode GitHub's cache-access distinction and reusable-call inheritance in structural workflow policy and regression tests.
- Correct the historical record without deleting or silently rewriting the archived rationale that explains how the repository arrived at the current design.
- Verify functionality and security on GitHub, not only through local tests or a disappearing CodeQL data-flow path.

**Non-Goals:**

- Trusting, signing, or promoting debug binaries as production release artifacts.
- Redesigning normal pull-request CI, SonarCloud analysis, production releases, or the `debug-release` environment's reviewer membership.
- Supporting self-hosted runners for pull-request-controlled execution.
- Restoring general cache writes for low-trust events or relying on disabled convenience caching as the primary control.
- Introducing a long-lived credential solely to manufacture a `pull_request:labeled` event.
- Erasing archived change artifacts, checked task state, or commit history.

## Decisions

### 1. Call a reusable builder from the authorized `issue_comment` run

`debug-build-request.yml` will retain `issue_comment: created` and the exact-command job gate. Its first job will verify that the event belongs to a pull request, require the commenter to have `write`, `maintain`, or `admin` repository permission, resolve the current pull request, require this repository as the base, and emit only the pull-request number and full current head SHA as job outputs.

A dependent job will call `./.github/workflows/debug-build.yml` at job level. `debug-build.yml` will expose only `workflow_call` with typed required inputs; it will no longer expose `workflow_dispatch`. The called workflow will re-read the pull request before checkout and fail if its base repository or current head differs from the supplied immutable identity. It will use a GitHub-hosted runner, explicit `contents: read`, no `secrets: inherit`, no environment, no secret references, `persist-credentials: false`, and disabled automatic dependency caching.

Because a reusable job inherits the caller's `issue_comment` event, GitHub supplies read-only default-branch cache access and current CodeQL does not classify the call as writable. The trusted authorization job performs no checkout. The reusable build job is the only job that executes pull-request content.

**Alternatives considered:**

- Keep `workflow_dispatch`: rejected because GitHub and CodeQL explicitly classify it as default-branch cache-write-capable.
- Reuse the earlier `GITHUB_TOKEN` label signal: rejected because the generated `labeled` activity does not start the workflow.
- Apply the label with a GitHub App or PAT: secure but rejected because a new credential and installation lifecycle are unnecessary now that `issue_comment` cache writes are platform-blocked.
- Build directly in the authorization job: technically viable under the new cache policy, but rejected because a reusable workflow keeps validation, permissions, execution, and policy review focused and testable.
- Run every debug build from normal `pull_request` CI: secure but rejected because debug binaries are an on-demand operational artifact.

### 2. Revalidate mutable pull-request identity at the execution boundary

The authorization output captures the current full head SHA, but the pull request can advance before the called job starts. The builder will therefore re-query the pull request immediately before checkout and require an exact repository, pull-request number, and case-insensitive full-SHA match. A superseded request fails; it does not silently build the newer revision. Concurrent requests for one pull request remain serialized, and the workflow will not cancel an already-running immutable build.

Inputs will be passed through expression-safe `with` values and then through environment variables to scripts. They will never be interpolated directly into shell source. API-returned identity remains the authority.

**Alternatives considered:**

- Automatically replace the captured SHA with the new head: rejected because it breaks the request/run/provenance binding.
- Trust only the broker output: rejected because queue delay creates a time-of-check/time-of-use window.

### 3. Finalize provenance on a clean trusted runner

The reusable build job will upload binaries under an intermediate run-specific artifact name. After the reusable call succeeds, a separate finalization job in the trusted request workflow will run on a fresh GitHub-hosted runner. It will revalidate the PR/SHA, download the intermediate artifact as opaque data, enforce the expected bounded file layout without executing or sourcing any downloaded content, create `debug-build-provenance.json` from trusted authorization outputs and GitHub run metadata, and upload the final `debug-build-${github.run_id}` artifact.

The fresh job prevents pull-request code from altering the provenance file through workspace state, `$GITHUB_PATH`, `$GITHUB_ENV`, or a substituted tool. Artifact names are immutable and run-specific; if untrusted code pre-creates the trusted final artifact name, the trusted upload fails and the whole run is ineligible for publication rather than accepting attacker-selected contents.

**Alternatives considered:**

- Continue creating provenance after the build in the same job: rejected because pull-request code can mutate the workspace and runner command environment before the provenance step.
- Treat provenance as a signature: rejected because no signing authority is introduced; provenance binds trusted run metadata but the binaries remain explicitly untrusted debug output.

### 4. Authenticate the request run before publication

The publisher will continue to be manually dispatched only from `main`, with validation separated from the `debug-release` environment-gated write job. It will accept only a successful run whose workflow path is `.github/workflows/debug-build-request.yml`, event is `issue_comment`, repository is this repository, and final artifact has the exact run-derived name and is not expired. It will re-read the requested pull request and require its current head to equal the supplied full SHA, then compare repository, event, PR, SHA, run ID, run attempt, and artifact identity with the trusted final provenance.

The publish job will download only the artifact ID emitted by validation, recheck provenance on its own fresh runner, and pass only expected binary paths to the pinned release action. It will not check out the pull request, invoke a downloaded binary, source downloaded text, add downloaded paths to the command environment, or grant write permission before validation succeeds.

**Alternatives considered:**

- Continue identifying `debug-build.yml` / `workflow_dispatch`: rejected because reusable execution belongs to the caller run.
- Publish directly from the request run: rejected because it would combine untrusted execution with release authority.

### 5. Model cache authority separately from workflow source and repository permissions

The policy checker currently treats several default-branch triggers uniformly and then exempts the current debug builder. That abstraction hid the distinction that matters to alert 23. Policy will instead model at least three independent properties:

1. whether the workflow definition is selected from the protected default branch;
2. whether a job receives repository, environment, OIDC, package, or secret privilege; and
3. whether the effective root trigger can write the default-branch cache.

The cache-write event set will match GitHub's documented and current CodeQL set: `push` on the default branch, `workflow_dispatch`, `repository_dispatch`, `delete`, `registry_package`, `page_build`, and `schedule`. Reusable calls inherit the caller event. The checker will reject checkout/download followed by execution of untrusted content whenever the effective event has cache-write authority, even if repository permissions are read-only and convenience caching is disabled.

Repository-specific topology validation will require the exact comment authorization → local reusable call → clean finalization → protected publisher chain. It will reject `GITHUB_TOKEN` label signaling, direct workflow dispatch of the builder, secret inheritance, self-hosted execution, missing revalidation, same-job provenance, and any privileged publisher checkout or execution.

The event set will be documented in code with the GitHub cache reference and covered by table-driven tests so a future platform change requires an explicit reviewed policy update.

### 6. Add outcome records to the two misleading archives

Each of these archived changes will receive an `outcome.md` describing what actually happened and linking to this change:

- `2026-07-16-fix-debug-build-cache-poisoning`: CodeQL closed the original finding, but `GITHUB_TOKEN` label mutation did not trigger the labeled builder; static success was not end-to-end success.
- `2026-07-17-fix-security-review-findings`: direct dispatch restored the command path, but `workflow_dispatch` retained default-branch cache-write authority and later produced alert 23.

A short prominent historical-outcome notice will be added to the top-level archived proposal, design, and tasks files, pointing to `outcome.md`. Original decisions, task checkboxes, validation claims, and dates will remain intact beneath the notice. This makes the archive honest without rewriting historical evidence.

Current `docs/ci-security.adoc` will explain the new topology, the June 26, 2026 GitHub change, the exact write-capable event set, the reusable inheritance assumption, the reason disabled `setup-go` caching is only defense in depth, operator instructions, and rollback criteria.

### 7. Require local, pre-merge, and staging acceptance evidence

Local validation will cover workflow YAML, action pins, permissions, policy fixtures, unit tests, documentation, and strict OpenSpec validation. Pre-merge validation requires successful protected policy, CodeQL Actions analysis, and no replacement high-severity workflow finding for the exact cutover SHA. Live validation uses an exact production-tree staging repository and an authorized comment to prove one request run invokes one reusable build, produces the intermediate and trusted final artifacts, passes publisher provenance checks, completes a successful deployment, and publishes only the seven expected binaries.

The implementation records run URLs, run/event/workflow identities, artifact names, CodeQL version, alert state, cleanup actions, and the exact staging/production tree identity in change evidence. After merge, maintainers repeat the default-branch request and confirm alert 23 closes without dismissal. Because that observation is only possible after the implementing pull request has merged, it is an operational verification recorded on the merged pull request rather than an implementation-completion or archival gate.

### 8. Deliver the protected-policy bootstrap as a native GitHub PR stack

The authoritative `pull_request_target` workflow executes checker code from the repository's default branch, even when a stacked pull request targets an intermediate branch. A single pull request cannot both replace the debug topology and teach that protected default-branch checker to accept it: the old checker rejects the target before candidate code can run. An exact `/build-debug` pre-merge test is also unsafe because `issue_comment` loads the old workflow definition from the default branch.

Delivery will therefore use GitHub's public-preview stacked pull requests through the official `github/gh-stack` extension:

1. The bottom policy-transition branch targets `main`. It adds the structural parser, effective-caller model, tests, and a narrowly reviewed migration rule that accepts either the exact legacy debug topology or the exact target topology. It does not change executable debug workflows. The old base checker therefore sees unchanged workflow topology and can approve this layer.
2. The top workflow-cutover branch targets the policy-transition branch. It changes the request, builder, finalizer, publisher, documentation, and archive outcomes, and tightens policy so the legacy dispatch and label topologies are rejected. Before the bottom layer merges, its protected check still runs the old default-branch checker and is expected to fail.
3. Merge bottom-up. GitHub retargets the top pull request to `main`; rerun required checks so the now-default transition checker validates the strict target, then merge the cutover. The repository spends only the interval between layer merges under the exact dual-topology transition rule.

The transition rule must compare the complete legacy workflow envelope rather than introduce a general dispatch exemption. It remains covered by a removal test in the top layer. If the top layer cannot merge promptly, disable the debug request path rather than leave a broadly permissive migration rule.

**Alternatives considered:** a manual pair of unrelated pull requests was rejected because stack metadata makes ordering and base relationships explicit; temporarily weakening the required protected check was rejected because candidate code must never select its own authority; posting `/build-debug` before cutover was rejected because it would execute the known-vulnerable default-branch predecessor.

## Risks / Trade-offs

- **[Risk] GitHub changes cache-token policy or CodeQL's event model.** → Keep the documented event set in one policy function with source references and tests; require pre-merge CodeQL and an exact production-tree staging request before archival; repeat both against the default branch after merge and disable the debug path if the platform no longer guarantees read-only `issue_comment` cache access.
- **[Risk] A new writable caller invokes the reusable builder.** → Structural policy resolves reusable callers and rejects any cache-write-capable or privileged call path; `debug-build.yml` exposes no manual trigger.
- **[Risk] A pull request advances between authorization and build.** → Revalidate immediately before checkout and fail stale requests rather than substituting a revision.
- **[Risk] Pull-request code tampers with artifacts or attempts denial of service.** → Treat binaries as untrusted, use run-specific immutable artifact names, generate provenance on a fresh runner, never execute artifacts in trusted jobs, and accept that an authorized hostile build may consume its bounded job resources or make its own run fail.
- **[Risk] Fork or contributor workflow approval delays execution.** → The root run is trusted `issue_comment` code from the default branch rather than a fork-defined workflow; document any organization approval behavior observed during the live test and do not weaken permissions to bypass it.
- **[Risk] Publisher run association differs for reusable workflows.** → Validate the actual request workflow path and `issue_comment` event through the Actions API in tests and the post-push exercise; fail closed on missing or ambiguous identity.
- **[Risk] Archive notices are mistaken for rewriting history.** → Add additive outcome records and notices only; preserve original contents and link concrete commits, alerts, and successor changes.
- **[Trade-off] Intermediate plus final artifacts add storage and workflow time.** → Use short retention and delete or omit intermediate artifacts where GitHub's supported APIs permit after finalization; prioritize trusted provenance over minimal artifact count.
- **[Risk] The transition policy remains merged while the cutover is delayed.** → Accept only byte-for-byte reviewed legacy and target trust envelopes, keep the stack relationship visible, merge the top layer promptly, and disable the request path if the cutover cannot proceed.
- **[Trade-off] GitHub stacked pull requests are in public preview.** → Retain ordinary branches and PR bases as the source of truth; if stack tooling fails, preserve the same bottom-up base relationship manually without weakening checks.

## Migration Plan

1. Create a bottom policy-transition branch from `main`. Add parser/call-graph improvements and exact dual-topology migration validation without changing debug workflow files.
2. Initialize a native GitHub PR stack with the policy-transition branch below the workflow-cutover branch. Submit both PRs so the top PR targets the bottom branch.
3. Require the bottom PR's old protected-base policy check, local policy tests, workflow lint, and broader validation to pass; merge it first.
4. Keep the top layer atomic: convert `debug-build.yml` to `workflow_call`, split authorization/call/finalization, update publisher identity/provenance, update documentation and archives, and remove the legacy transition allowance.
5. After GitHub retargets the top PR to `main`, rerun protected policy and CodeQL for the exact top SHA. Require no replacement high-severity workflow finding; alert 23 remains undismissed until the fixing topology reaches the default branch.
6. Before merge, mirror the exact top-layer tree into the staging repository; exercise authorized, non-exact, stale-head, malformed-publisher, and successful publication paths; record request/called jobs, permissions, runner, artifacts, provenance, deployment, release contents, and cleanup.
7. After merge and archival, repeat one exact authorized `/build-debug` against the new default-branch definitions and confirm alert 23 closes without dismissal. Record this operational verification on the merged pull request; disable the request path rather than restoring either predecessor if it fails.

Rollback is to disable the request job or remove the reusable build call while retaining the protected publisher. Do not roll back to `workflow_dispatch` execution of pull-request code or the `GITHUB_TOKEN` label signal.

## Open Questions

No implementation decision remains open. Exact Actions API fields, reusable caller behavior, artifact association, publisher behavior, and pre-merge CodeQL were verified locally and in staging. Default-branch request and alert-closure checks remain an explicitly non-blocking post-merge operational verification.
