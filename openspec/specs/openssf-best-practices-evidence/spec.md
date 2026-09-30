# Purpose

Maintain independently reviewable evidence for the project's applicable OpenSSF Best Practices Badge answers and Go vulnerability reporting posture.

# Requirements

## Requirement: BadgeApp answers are evidence-backed and criterion-specific
The project SHALL maintain `.bestpractices.json` entries for `dynamic_analysis_unsafe`, `crypto_working`, `crypto_weaknesses`, and `crypto_pfs` only when each status and justification is supported by current authoritative BadgeApp criteria and independently reviewable repository evidence.

### Scenario: Go-only dynamic-analysis criterion is assessed
- **WHEN** the project evaluates `dynamic_analysis_unsafe`
- **THEN** it records `N/A` only when the checked-in source and release build configuration demonstrate that the software produced by the project is not written in a memory-unsafe language

### Scenario: Cryptographic criteria are assessed
- **WHEN** the project updates `crypto_working`, `crypto_weaknesses`, or `crypto_pfs`
- **THEN** each justification identifies the applicable production security mechanism, its maintained implementation boundary, and the evidence that supports the stated default behavior

### Scenario: SSH cryptographic-strength claim is assessed
- **WHEN** the project records `crypto_weaknesses` as `Met`
- **THEN** the evidence identifies the explicit Git-over-SSH client algorithm allowlists and demonstrates that the effective configuration excludes legacy SHA-1, CBC, RC4, DSA, and `ssh-rsa` fallbacks

### Scenario: Evidence is insufficient or stale
- **WHEN** current criterion text or repository evidence does not support a proposed status
- **THEN** the project retains an unknown or unmet status and does not represent a dependency declaration or policy aspiration as proof

## Requirement: Go vulnerability findings are triaged in GitHub code scanning
The project SHALL emit Go vulnerability scan findings as SARIF and upload them to GitHub code scanning. The workflow SHALL preserve raw `govulncheck` findings and SHALL use GitHub code-scanning triage, mitigation, and documented risk-acceptance controls instead of repository-local finding suppression.

### Scenario: A Go vulnerability scan reports findings
- **WHEN** the repository runs its Go vulnerability scan in CI
- **THEN** it produces a SARIF report and uploads it to GitHub code scanning even when `govulncheck` reports findings

### Scenario: A finding is risk accepted
- **WHEN** maintainers determine that a reported finding is not exploitable through the current code paths
- **THEN** they record the assessment and risk acceptance in GitHub rather than filtering the finding from the scanner input or output

## Requirement: Warning-only key handling remains truthfully reported
The project SHALL retain `crypto_keylength` as `Unmet` while undersized SSH identities remain accepted, and its justification SHALL state that warnings do not disable or reject smaller keys.

### Scenario: Undersized identities remain compatible
- **WHEN** the SSH import path permits an identity below the documented strength baseline after warning the user
- **THEN** the BadgeApp answer does not claim that smaller key lengths are completely disabled

### Scenario: Future key enforcement is introduced
- **WHEN** the project later changes default key acceptance or supplies an effective mechanism to disable smaller keys
- **THEN** the `crypto_keylength` status is reevaluated against the current authoritative criterion before it is changed
