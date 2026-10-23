# Shared MCP file access

## ADDED Requirements

### Requirement: guided shared access
The system SHALL hide enumeration tools from ordinary users while explaining the replacement workflow through capabilities.

#### Scenario: missing receipt
- **WHEN** an Agent has no upload receipt
- **THEN** the response SHALL recommend selecting the file again and creating a new upload, without global search

#### Scenario: guessed enumeration
- **WHEN** an ordinary user guesses a hidden enumeration tool
- **THEN** the owner SHALL return a structured guided error with `reason`, `impact`, `next_actions`, `retryable`, and `recovery_tool`

### Requirement: safe reference handoff
The system SHALL bind a stable file version to one consumer, project, purpose, digest, and expiry before owner consumption.

#### Scenario: handoff failure
- **WHEN** binding fails
- **THEN** the response SHALL preserve the original reference and idempotency key and return an owner-specific recovery action

### Requirement: retry and audit safety
Only explicitly retryable failures may be retried with the original idempotency key; audit records SHALL omit URLs, bytes, base64, credentials, and provider payloads.

#### Scenario: transient failure
- **WHEN** a backend or capacity error is classified as retryable
- **THEN** the response SHALL set `retryable=true` and identify the recovery tool
