## ADDED Requirements

### Requirement: BlobGC claims are recoverable

The service MUST persist an opaque claim token and claim timestamp when BlobGC claims a zero-reference blob, and MUST match that token when releasing or deleting the claim.

#### Scenario: dry-run claim

- **WHEN** an operator runs BlobGC without confirmation
- **THEN** the service reports candidates and restores the blob to an unclaimed state without deleting the object

#### Scenario: restart after interrupted claim

- **WHEN** the service starts after a previous process stopped with a claimed blob
- **THEN** startup clears the abandoned claim after acquiring the process lock and the next GC run can re-evaluate references

### Requirement: Metadata verification detects state drift

Metadata verification MUST preserve the existing count summary and MUST compare deterministic fingerprints for core and workspace authorization state.

#### Scenario: equal counts with changed state

- **WHEN** two snapshots contain the same number of rows but different workspace grant or file values
- **THEN** verification returns `matching=false` and exposes source and target fingerprints without exposing secrets

### Requirement: Workspace state is migratable

Metadata export and import MUST include workspace grants, groups, organizations, requests, submissions, decisions, credential history, label suggestions, suggestion decisions, and invocation observations.

#### Scenario: empty workspace tables

- **WHEN** a snapshot has empty workspace tables
- **THEN** import succeeds without issuing empty-slice inserts or creating phantom rows

### Requirement: Database modes are explicit

Backend verification MUST run against SQLite and an explicitly selected disposable PostgreSQL database, and MUST NOT silently fall back when PostgreSQL is unavailable.

#### Scenario: PostgreSQL is unavailable

- **WHEN** the PostgreSQL verification command cannot connect to its disposable database
- **THEN** the command fails with a non-zero exit code and records the failure evidence instead of running SQLite as a substitute
