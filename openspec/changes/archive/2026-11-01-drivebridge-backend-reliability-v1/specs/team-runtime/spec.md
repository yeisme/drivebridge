## MODIFIED Requirements

### Requirement: Metadata migration verification detects state drift

The team runtime metadata migration MUST retain the existing count summary and MUST compare deterministic fingerprints for core file state and workspace authorization state before reporting a match.

#### Scenario: identical snapshots

- **WHEN** source and target snapshots contain the same rows and values
- **THEN** verification returns `matching=true` with equal count and fingerprint maps

#### Scenario: equal counts with changed state

- **WHEN** a file, grant, or workspace receipt changes while row counts remain equal
- **THEN** verification returns `matching=false` and exposes differing fingerprints without secrets
