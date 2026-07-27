## Context

Trusted-main run `30250212009` analyzed exact revision `bb3d10dc2360a50752e2c73fe507c427bf14af92`. Versioning, lint, schema generation, tests, coverage generation/import, duplication, bug, and vulnerability conditions passed; the scanner failed only while waiting for the quality gate because `new_code_smells_severity=20` (Critical) exceeded the required `<14` (below Major). Authenticated issue history and the archived `secure-sonarcloud-pr-analysis` verification attribute the remaining gate debt to older pull requests. One of the seven recorded issues is now closed as fixed, leaving six expected gate blockers. `evidence.md` records the token-free current analysis, gate conditions, previous-version provenance, and per-key rule, severity, assignment, component, status, and tier for all 23 scoped issues.

Sonar reports 82 open smells overall, but most are outside the gate-relevant previous-version period or require changes whose risk is disproportionate to this recovery. The implementation therefore needs an evidence-based boundary rather than a repository-wide mechanical cleanup. Workflow-policy and Git SSH code are security-sensitive; test refactors must preserve assertions and cases; all Sonar administration and token handling must avoid credential disclosure.

The implementation is intended to be taskable to a smaller coding model. Each task must therefore state observable preconditions, one bounded transformation, observable postconditions, checks, and an escalation condition. A task is not permission to reinterpret behavior merely to satisfy a rule.

## Goals / Non-Goals

**Goals:**

- Close the six Critical findings expected to block the protected pull-request quality gate.
- Resolve all remaining Richard-assigned or `scripts/` findings in the investigated set.
- Resolve a bounded set of other Critical/Major findings whose clean fix is private, behavior-preserving, and supported by focused tests.
- Preserve workflow trust boundaries, Git SSH precedence and fail-closed host-key behavior, test coverage, public APIs, scanner configuration, coverage mapping, and quality-gate policy.
- Correct independently reviewed Git SSH trust-source drift so command-side `GlobalKnownHostsFile` and SSH-file quoted, escaped, repeated, and multi-path directives follow the existing verified-SSH contract.
- Provide issue-by-issue tasks that a smaller model can execute safely by confirming preconditions and postconditions.
- Produce authenticated post-change evidence from the exact protected pull-request revision and restore a green protected pull-request analysis.

**Non-Goals:**

- Clearing all 82 open Sonar findings.
- Suppressing, accepting, reclassifying, dismissing, excluding, or lowering the severity of findings.
- Changing the quality gate, new-code definition, previous-version boundary, required checks, Sonar project identity, token model, scanner pin, report paths, or `source/` coverage namespace.
- Altering public APIs, recursive import semantics, environment inheritance, pipeline/graph semantics, scheduler or watcher concurrency, container privilege policy, or service lifecycle behavior. Git SSH parsing changes remain limited to the two reviewed known-host selection/path-boundary defects.
- Adding dependencies or combining unrelated security/correctness findings discovered during reconnaissance.

## Decisions

### 1. Use three explicit remediation tiers

**Tier 0 — gate blockers:**

- `AZ-N-ywQ3_y5QfimGTZy` — `internal/config/cache_test.go:53`
- `AZ-N-ywQ3_y5QfimGTZz` — `internal/config/cache_test.go:126`
- `AZ-N-yxh3_y5QfimGTZ0` — `internal/config/loader_git.go:559`
- `AZ-N-y0t3_y5QfimGTZ2` — `scripts/check-workflow-policy/main.go:344`
- `AZ-N-y0t3_y5QfimGTZ7` — `scripts/check-workflow-policy/main.go:365`
- `AZ-N-y0t3_y5QfimGTZ4` — `scripts/check-workflow-policy/main.go:387`

**Tier 1 — mandatory Richard/`scripts/` priority:**

- `AZ-N-y1d3_y5QfimGTaC` — `scripts/materialize-sonar-source/main.go:96`
- `AZ-N-y1d3_y5QfimGTaD` — `scripts/materialize-sonar-source/main.go:102`

**Tier 2 — selected low-risk Critical/Major debt:**

- `AZWKA6kX03lIpGBL3rEn` — duplicate `no-summary` key
- `AZWKA6j703lIpGBL3rEk` — duplicate `no-prompt` key
- `AZ1t2WyVtvGp9fbwa0Kx` — command-test helper complexity
- `AZ1t2Wy_tvGp9fbwa0K3` — `main` test complexity
- `AZ1t2WvrtvGp9fbwa0Km` — prefixed-output test complexity
- `AZ1t2WzPtvGp9fbwa0K4` — variables merge-test complexity
- `AZbPupl0Qf_52EOIHgtb` — command summary complexity
- `AZ1t2WjltvGp9fbwa0IF` — generated-GitHub ordering-test complexity
- `AZ1t2WqptvGp9fbwa0Ir` — map-key test complexity and weak exactness
- `AZWKA6je03lIpGBL3rEh` — task compiler complexity
- `AZWKA6je03lIpGBL3rEi` — private compiler function parameter count
- `AZZ8glSK1wqWcqTip8XF` — intentional container reset no-op lacks rationale
- `AZxSHQDOhNFpWUf3zR1y` — timeout cancellation placement
- `AZ1t2W0ytvGp9fbwa0K8` — duplicated scheduler test
- `AZ1t2W1PtvGp9fbwa0LE` — graph test complexity

Tier 2 is deliberately finite and subordinate to Tier 0/Tier 1 recovery. Each Tier 2 key is admitted only if refreshed authenticated evidence still matches `evidence.md`, focused tests characterize current behavior, and the implementation remains private and behavior-preserving. A drifted or riskier Tier 2 key receives an explicit defer disposition in `evidence.md`, moves to a separate proposal, and cannot delay Tier 0/Tier 1 acceptance. Any newly observed issue or materially different code shape requires proposal review rather than automatic scope expansion.

**Alternative considered:** fix all open Critical/Major issues. Rejected because it would mix public API redesign, recursive imports, pipeline construction, scheduler/watcher concurrency, container cancellation, and very large asynchronous tests into a quality-gate recovery, making regressions and review attribution substantially harder.

### 2. Apply a uniform issue-task contract

Every implementation task SHALL use this sequence:

1. **Confirm preconditions:** query or inspect the named issue, confirm it is open at the expected path/rule, inspect the complete containing function/test plus direct callers and focused tests, and confirm the task's stated invariants still match the repository.
2. **Stop on drift:** if the key is closed, moved into materially different behavior, overlaps an uncommitted/user change, requires an excluded semantic/API change, or lacks the expected tests, stop and report the mismatch; do not improvise.
3. **Make one bounded fix:** modify only the named concern and the minimum directly related tests. Preserve error text, ordering, precedence, trust predicates, JSON tags, assertions, and public signatures unless the task explicitly says otherwise.
4. **Confirm postconditions:** format changed Go files, run the named focused tests/checks, inspect the diff for scope and assertion loss, and confirm the original rule condition no longer exists structurally.
5. **Record evidence:** update `evidence.md` with changed paths, commands and outcomes, unrun checks, per-key disposition, review links, and residual risk without claiming Sonar closure before authenticated analysis.

This protocol is preferred over broad instructions such as “clean up the function” because a smaller model needs bounded authority and unambiguous stop conditions.

### 3. Preserve behavior at the two security-sensitive boundaries

For `processSSHConfig`, extraction may separate scalar defaults, identity-file handling, known-host-list handling, and strict-host-key handling, but MUST preserve explicit-command-over-file precedence, defaults (`22`, `git`, requested host), repeated known-host entries, and fail-closed host-key verification. The approved review follow-up MUST additionally recognize OpenSSH `GlobalKnownHostsFile` in supported `GIT_SSH_COMMAND -o` forms and parse known-host path lists from SSH configuration without losing quoting, escaping, repetition, order, or legacy first-entry compatibility. It must not remove the current error signature or broaden SSH parsing beyond these two defects.

For `validateRepositoryTopology`, extraction must preserve check order, fail-fast behavior, exact predicates, error semantics, trusted/untrusted boundaries, static-main checkout checks, action-prefix matching, and independent immutable-action validation. Constants may centralize identifiers but must not change their values or matching semantics.

**Alternative considered:** generic table-driven validators. Rejected because explicit phases are easier to audit and less likely to conceal an omitted security predicate.

**Alternative considered:** record the independent Git SSH findings as unrelated pre-existing debt. Rejected because silently ignoring an explicit global trust source or misreading configured path boundaries can select the wrong host-key trust material. The correction is therefore mandatory within this security-sensitive slice and must receive renewed independent approval.

### 4. Preserve test strength while reducing test complexity

Test-only remediation must move fixture creation, execution, or assertions into named helpers or focused subtests. It must preserve the original case count and every meaningful assertion. A duplicated scheduler test may only be removed if the surviving test proves identical coverage; if its name implies a missing behavior, it must be replaced with a genuine test for that behavior.

No task may reduce complexity by deleting error paths, weakening exact comparisons, replacing assertions with logging, or skipping platform/security cases.

### 5. Separate local verification from authoritative Sonar acceptance

Focused tests and static inspection can prove behavior and structure locally, but only authenticated SonarCloud analysis can close an issue and determine the aggregate gate. Implementation tasks therefore confirm local postconditions without claiming issue closure. Final acceptance for this change queries every scoped key and requires a fresh protected pull-request analysis of the exact source revision; post-merge trusted-main monitoring remains ordinary branch governance outside this change.

The token is loaded from the maintainer environment only for API calls, never printed, persisted, passed on the command line, or included in artifacts.

### 6. Sequence work to isolate risk

Recommended implementation order:

1. Baseline authenticated issue/gate snapshot.
2. Low-risk local constants and explanatory no-op/cancel placement.
3. Materializer named response types.
4. Cache and selected non-security test decomposition.
5. Private compiler and summary refactors.
6. Git SSH refactor, mandatory trust-source/path-boundary corrections, focused security tests, and independent re-review.
7. Workflow-policy complexity refactor with mutation/security checks and independent review.
8. Full validation, protected PR analysis, and issue/gate re-query.

The Git SSH and workflow-policy refactors must not share an implementation commit. Parallel work is permitted only in isolated worktrees on non-overlapping files; otherwise one writer proceeds sequentially. Their independent reviewer must not be the implementing model/person, must record approval or findings in a PR review or `evidence.md`, and must link every finding to its resolution and rerun evidence before the slice is complete.

## Risks / Trade-offs

- **[Risk] Closing the six keys may not lower the aggregate severity if Sonar attributes a replacement issue to moved code.** → Re-query scoped keys and the quality-gate condition after a fresh protected PR analysis; do not weaken the gate if it remains red.
- **[Risk] Workflow-policy extraction could omit or reorder a trust check.** → Preserve explicit ordered phases, run the full mutation matrix and workflow-security checks, and require independent review.
- **[Risk] SSH extraction or parser correction could change precedence or known-host behavior.** → Add/retain focused scalar, identity, command-side user/global trust-source, quoted/escaped/multi-path/repeated known-host, strict-checking, unknown-host, changed-key, and opt-out tests; require renewed independent approval.
- **[Risk] Test refactors could make tests pass by weakening them.** → Compare case/assertion inventory before and after and require diff review.
- **[Risk] Smaller models may treat task wording as permission to broaden scope.** → Each task has named files, invariants, stop conditions, and postconditions; any mismatch escalates.
- **[Trade-off] Tier 2 leaves substantial Sonar debt open.** → This is intentional; medium/high-risk or behavior-changing debt needs separate proposals rather than jeopardizing the green-build recovery.
- **[Risk] Intermediate merges may leave `main` red until the final blocker closes.** → Prepare slices before merging and merge in a tight sequence, or use one reviewed integration branch with logically isolated commits.

## Migration Plan

1. Capture the current Sonar analysis, gate conditions, previous-version period, and statuses for all scoped issue keys without exposing credentials.
2. Implement Tier 0 and Tier 1 in reviewable slices, followed by Tier 2 only while its preconditions remain true.
3. Run focused package/security checks after each slice and repository-wide checks before requesting final review.
4. Obtain independent review for Git SSH and workflow-policy slices.
5. Run protected PR Sonar analysis and confirm the exact PR revision has no new qualifying issues and a green quality gate.
6. Merge through the protected branch process under ordinary branch governance; trusted-main monitoring is not an acceptance task for this change.
7. If the protected PR gate remains red, stop and use authenticated issue/measure evidence to identify the qualifying issue; do not alter policy or expand scope automatically.

Rollback is ordinary source reversion of the offending slice. Scanner configuration, required checks, quality-gate policy, credential controls, CODEOWNERS, and trusted analysis topology remain in place during rollback.

## Open Questions

- Whether implementation will use several tightly sequenced pull requests or one integration pull request with independently reviewed commits should be decided immediately before `/opsx-apply`, based on branch-protection and review availability.
- Sonar issue locations can move after preceding refactors; tasks use issue keys as identity and must re-confirm the current path before editing.
