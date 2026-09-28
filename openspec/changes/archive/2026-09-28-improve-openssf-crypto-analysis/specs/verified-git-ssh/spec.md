## ADDED Requirements

### Requirement: Git SSH transport excludes legacy cryptographic fallbacks
The system SHALL construct the Git-over-SSH client configuration with explicit key-exchange, cipher, MAC, host-key, and public-key authentication algorithm allowlists. The allowlists SHALL exclude SHA-1 compatibility fallbacks, SSH CBC, RC4, DSA, and `ssh-rsa` and SHALL not rely on the `golang.org/x/crypto/ssh` default algorithm lists.

#### Scenario: A secure Git SSH client is constructed
- **WHEN** the Git import path constructs SSH authentication after resolving the identity and host-key callback
- **THEN** the client configuration offers only the explicit non-legacy algorithms and continues to use the selected user, signer, and host-key callback

#### Scenario: A server or identity needs a legacy algorithm
- **WHEN** the selected Git server or identity supports only an excluded legacy algorithm
- **THEN** the SSH connection MAY fail without weakening the configured transport policy, replacing the identity, or changing host-key verification behavior

### Requirement: SSH identity strength warnings preserve compatibility
The system SHALL assess the public strength characteristics of an SSH identity selected through the existing configuration-resolution flow and SHALL emit a non-blocking warning when the identity is below the documented cryptographic-strength baseline or cannot be classified safely.

#### Scenario: Undersized RSA identity is selected
- **WHEN** a selected RSA identity has a modulus smaller than the documented minimum baseline
- **THEN** the system emits a warning identifying the safe algorithm category, observed strength, and recommended baseline, and continues the existing authentication flow

#### Scenario: Supported elliptic-curve identity meets the baseline
- **WHEN** a selected supported elliptic-curve or Ed25519 identity meets the documented minimum baseline
- **THEN** the system does not emit an understrength-key warning and preserves existing authentication behavior

#### Scenario: Identity strength cannot be classified
- **WHEN** a selected supported identity form does not expose sufficient safe public metadata for the system to classify its strength
- **THEN** the system leaves the identity usable and emits a non-blocking diagnostic that its strength could not be verified

### Requirement: SSH strength diagnostics protect credential material
SSH identity-strength diagnostics SHALL contain only non-sensitive algorithm and strength metadata and SHALL NOT include private key contents, passphrases, tokens, repository URLs, or local identity-file paths.

#### Scenario: Warning is emitted
- **WHEN** the system emits an understrength or unclassifiable identity diagnostic
- **THEN** its output excludes private credential material and identity-file location details

### Requirement: Existing SSH trust behavior remains unchanged
The identity-strength assessment SHALL not alter OpenSSH configuration resolution, selected identity-file precedence, supported private-key parsing, known-host verification defaults, or the explicit `StrictHostKeyChecking=no` compatibility opt-out.

#### Scenario: Existing OpenSSH configuration selects an identity
- **WHEN** an identity is selected through the existing SSH configuration or supported `GIT_SSH_COMMAND` options
- **THEN** the system assesses and warns after selection without replacing the identity or changing selection precedence

#### Scenario: Explicit host-key compatibility opt-out is used
- **WHEN** the effective SSH configuration sets `StrictHostKeyChecking=no`
- **THEN** the existing host-key bypass warning and connection behavior remain in effect independently of any identity-strength diagnostic
