# Purpose

TBD: Define verified host-key behavior for Git-over-SSH connections.

# Requirements

## Requirement: Git SSH connections verify server host keys by default
The system SHALL authenticate the effective SSH server hostname and port against trusted known-host entries before exchanging Git repository data, and SHALL fail closed when verification cannot be completed.

### Scenario: Server presents a trusted key
- **WHEN** a Git SSH server presents a key matching a known-host entry for the resolved hostname and port
- **THEN** the connection proceeds to Git authentication and repository access

### Scenario: Server is unknown
- **WHEN** the resolved SSH server has no matching known-host entry
- **THEN** the connection fails with an actionable unknown-host error before repository data is accepted

### Scenario: Server key changed
- **WHEN** the SSH server presents a key that conflicts with an existing known-host entry
- **THEN** the connection fails with a host-key-mismatch error and does not silently replace or bypass the trusted key

## Requirement: Known-host configuration follows SSH configuration
The system SHALL resolve user and system known-host files using OpenSSH-compatible configuration, including the effective hostname and port after host aliases and supported `GIT_SSH_COMMAND` options are applied. Parsing SHALL preserve configured path boundaries, quoting, escaping, and platform-native path syntax.

### Scenario: SSH host alias changes destination
- **WHEN** SSH configuration maps a requested host alias to a different hostname or non-default port
- **THEN** host-key verification uses the effective destination and the correct known-host host-and-port form

### Scenario: User selects a known-host file
- **WHEN** `UserKnownHostsFile` is set through the selected SSH configuration or a supported `GIT_SSH_COMMAND` option
- **THEN** the system uses the configured file for host verification according to the documented precedence

### Scenario: Configured path contains spaces
- **WHEN** a known-host directive contains a quoted or escaped path with spaces
- **THEN** the system treats that value as the intended single path rather than splitting it into nonexistent files

### Scenario: Multiple known-host files are configured
- **WHEN** SSH configuration supplies multiple known-host files using supported OpenSSH syntax
- **THEN** the system preserves each configured path and loads the usable files in precedence order

### Scenario: No custom file is selected
- **WHEN** no custom known-host file is configured
- **THEN** the system uses readable standard user and operating-system-specific system known-host locations

### Scenario: Windows system trust file exists
- **WHEN** the program runs on Windows and the standard ProgramData OpenSSH known-host file exists
- **THEN** the system includes that file among the default system trust sources without requiring a custom override

### Scenario: No usable trust source exists
- **WHEN** no configured or default known-host source can be used
- **THEN** the connection fails with guidance for provisioning a trusted host key

## Requirement: Insecure host-key bypass is explicit and observable
The system SHALL permit host-key verification bypass only when the user explicitly configures `StrictHostKeyChecking=no`, and SHALL emit a clear warning for every connection that uses the bypass.

### Scenario: Explicit compatibility opt-out
- **WHEN** the effective SSH configuration sets `StrictHostKeyChecking=no`
- **THEN** the connection may proceed without host-key verification and the system warns that server identity is not being verified

### Scenario: Verification fails without opt-out
- **WHEN** normal host-key verification fails and no explicit opt-out is configured
- **THEN** the system returns the verification error and does not fall back to insecure behavior

## Requirement: SSH trust failures protect sensitive data
Host verification errors and warnings SHALL identify the host and corrective action without exposing private key contents, passphrases, tokens, or other credentials.

### Scenario: Host verification error is logged
- **WHEN** an SSH host is unknown or presents a mismatched key
- **THEN** diagnostic output contains safe host context and remediation guidance but no private credential material

## Requirement: Git SSH transport excludes legacy cryptographic fallbacks
The system SHALL construct the Git-over-SSH client configuration with explicit key-exchange, cipher, MAC, host-key, and public-key authentication algorithm allowlists. The allowlists SHALL exclude SHA-1 compatibility fallbacks, SSH CBC, RC4, DSA, and `ssh-rsa` and SHALL not rely on the `golang.org/x/crypto/ssh` default algorithm lists.

### Scenario: A secure Git SSH client is constructed
- **WHEN** the Git import path constructs SSH authentication after resolving the identity and host-key callback
- **THEN** the client configuration offers only the explicit non-legacy algorithms and continues to use the selected user, signer, and host-key callback

### Scenario: A server or identity needs a legacy algorithm
- **WHEN** the selected Git server or identity supports only an excluded legacy algorithm
- **THEN** the SSH connection MAY fail without weakening the configured transport policy, replacing the identity, or changing host-key verification behavior

## Requirement: SSH identity strength warnings preserve compatibility
The system SHALL assess the public strength characteristics of an SSH identity selected through the existing configuration-resolution flow and SHALL emit a non-blocking warning when the identity is below the documented cryptographic-strength baseline or cannot be classified safely.

### Scenario: Undersized RSA identity is selected
- **WHEN** a selected RSA identity has a modulus smaller than the documented minimum baseline
- **THEN** the system emits a warning identifying the safe algorithm category, observed strength, and recommended baseline, and continues the existing authentication flow

### Scenario: Supported elliptic-curve identity meets the baseline
- **WHEN** a selected supported elliptic-curve or Ed25519 identity meets the documented minimum baseline
- **THEN** the system does not emit an understrength-key warning and preserves existing authentication behavior

### Scenario: Identity strength cannot be classified
- **WHEN** a selected supported identity form does not expose sufficient safe public metadata for the system to classify its strength
- **THEN** the system leaves the identity usable and emits a non-blocking diagnostic that its strength could not be verified

## Requirement: SSH strength diagnostics protect credential material
SSH identity-strength diagnostics SHALL contain only non-sensitive algorithm and strength metadata and SHALL NOT include private key contents, passphrases, tokens, repository URLs, or local identity-file paths.

### Scenario: Warning is emitted
- **WHEN** the system emits an understrength or unclassifiable identity diagnostic
- **THEN** its output excludes private credential material and identity-file location details

## Requirement: Existing SSH trust behavior remains unchanged
The identity-strength assessment SHALL not alter OpenSSH configuration resolution, selected identity-file precedence, supported private-key parsing, known-host verification defaults, or the explicit `StrictHostKeyChecking=no` compatibility opt-out.

### Scenario: Existing OpenSSH configuration selects an identity
- **WHEN** an identity is selected through the existing SSH configuration or supported `GIT_SSH_COMMAND` options
- **THEN** the system assesses and warns after selection without replacing the identity or changing selection precedence

### Scenario: Explicit host-key compatibility opt-out is used
- **WHEN** the effective SSH configuration sets `StrictHostKeyChecking=no`
- **THEN** the existing host-key bypass warning and connection behavior remain in effect independently of any identity-strength diagnostic
