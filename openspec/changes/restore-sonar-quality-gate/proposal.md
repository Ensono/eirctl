## Why

The latest trusted `main` analysis (`bb3d10dc2360a50752e2c73fe507c427bf14af92`) passes lint, tests, report generation, coverage, duplication, bug, and vulnerability conditions but fails the SonarCloud quality gate because six pre-existing Critical code smells remain in the previous-version new-code period. The token-free authenticated snapshot in `evidence.md` confirms their current status and assignment; these blockers, every other Richard-assigned or `scripts/` finding in that snapshot, and a bounded set of other low-risk Critical/Major findings should be remediated now so that `main` becomes green without weakening security controls or turning a maintenance cleanup into a broad behavioral refactor.

## What Changes

- Remediate the six authenticated Critical findings expected to keep `new_code_smells_severity` above the permitted threshold: two cache-test complexity findings, one Git SSH configuration complexity finding, and three trusted workflow-policy findings.
- Remediate the two additional Richard-assigned findings under `scripts/materialize-sonar-source` even though they are Minor and do not currently block the gate.
- Resolve the evidence-ledger Tier 2 Critical/Major findings only where refreshed issue data and repository inspection establish a behavior-preserving, low-risk change; a drifted or riskier Tier 2 item is explicitly deferred to a separate proposal and cannot delay Tier 0/Tier 1 recovery.
- Require every issue task to be executable by a smaller implementation model: confirm named preconditions, make one bounded change, run named focused checks, confirm postconditions, and stop for escalation if repository evidence differs from the task.
- Re-query SonarCloud after implementation and require a fresh exact-revision `main` analysis to pass without suppressing, accepting, reclassifying, dismissing, or excluding issues and without changing the quality gate, new-code definition, scanner trust boundary, or coverage mapping.
- Explicitly defer findings that would alter public APIs, configuration/import semantics, scheduler or watcher concurrency, container privilege/cancellation behavior, or other security-sensitive behavior beyond a mechanical extraction.

## Capabilities

### New Capabilities
- `sonar-maintenance-remediation`: Defines issue-scoped, evidence-driven remediation and verification requirements for restoring and preserving maintainable SonarCloud results safely.

### Modified Capabilities
- `secure-ci-workflows`: Strengthens trusted-main acceptance so the exact analyzed `main` revision must pass the configured SonarCloud quality gate after the documented pre-existing blockers are remediated, without weakening the established scanner, credential, provenance, or report controls.

## Impact

- Primary code areas: `scripts/check-workflow-policy`, `scripts/materialize-sonar-source`, `internal/config`, selected command/output/runner/scheduler utilities, and their tests. `evidence.md` is the token-free issue and acceptance ledger.
- CI impact: trusted `main` SonarCloud analysis is expected to change from failed to passing while retaining the same project key, quality gate, previous-version policy, coverage paths, scanner pinning, and required-check identity.
- Security impact: workflow-policy and Git SSH changes are behavior-preserving refactors at trust boundaries and require focused security regression checks and independent review.
- API and dependency impact: no breaking API changes and no new dependencies are intended.
- Deferred areas remain outside this change: recursive import and pipeline semantics, environment inheritance decisions, public context APIs, watcher/scheduler lifecycle changes, Docker privilege policy, and large concurrency-sensitive test refactors.
