## Why

The public OpenSSF Best Practices record for eirctl leaves four evidence-backed security criteria unanswered and marks SSH key-length handling as unmet. The project needs truthful, reproducible evidence for its Go-only runtime, cryptographic defaults, and forward secrecy while making users aware of potentially undersized SSH identity keys without overriding their OpenSSH configuration.

## What Changes

- Add a maintained OpenSSF Best Practices evidence record and update `.bestpractices.json` with evidence-backed statuses and justifications for `dynamic_analysis_unsafe`, `crypto_working`, `crypto_weaknesses`, and `crypto_pfs`.
- Detect and warn when an SSH identity used for Git import does not meet the BadgeApp's 2030 minimum key-strength thresholds, without rejecting the key or changing its selection.
- Configure the Git-over-SSH transport with explicit non-legacy key-exchange, cipher, MAC, host-key, and public-key authentication algorithms rather than inheriting `golang.org/x/crypto/ssh` compatibility fallbacks.
- Preserve OpenSSH configuration resolution, supported private-key formats, and explicit `StrictHostKeyChecking=no` compatibility behavior.
- Add focused tests for warning classification and ensure the existing secure SSH and CI controls remain validated.
- Emit Go vulnerability findings as SARIF and upload them to GitHub code scanning, where vulnerability triage, mitigation, and documented risk acceptance are managed without suppressing raw scanner results.
- Complete the change through the repository delivery process: archive the accepted OpenSpec change, create a branch, commit the approved implementation, push it, and open a pull request.

## Capabilities

### New Capabilities
- `openssf-best-practices-evidence`: Maintains verifiable project evidence and BadgeApp answer-file entries for applicable dynamic-analysis and cryptography criteria.

### Modified Capabilities
- `verified-git-ssh`: Warns when a configured SSH identity is weaker than the documented cryptographic-strength baseline while retaining user-selected OpenSSH compatibility behavior.

## Impact

- **Code:** `internal/config/loader_git.go` and its tests, to inspect an already selected SSH identity, emit a safe warning when appropriate, and construct an explicit secure SSH client algorithm policy.
- **Security evidence:** `.bestpractices.json` and supporting project documentation or evidence, with statuses limited to claims verified from the repository and authoritative public criteria.
- **CI security reporting:** The Go vulnerability workflow and GitHub Actions permissions, to produce and upload SARIF without filtering or suppressing findings.
- **Compatibility:** Identity selection, parsing, and host-verification behavior remain unchanged, and no new service is introduced. Git servers or identities that support only legacy cryptographic algorithms are intentionally no longer compatible with the hardened transport policy.
- **Delivery:** Validation must cover focused Go tests, the relevant existing CI/security checks, OpenSpec validation, and the branch/commit/push/pull-request sequence after archival.
