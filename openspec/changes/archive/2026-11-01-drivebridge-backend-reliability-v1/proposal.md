## Why

DriveBridge already has the core file, Agent, outbox, Blob/CAS, and desktop protection primitives, but recovery evidence is uneven: a crashed BlobGC claim can remain stuck, metadata verification can report matching counts while state differs, and the backend test path does not consistently exercise both SQLite and PostgreSQL. This change closes those reliability gaps before real storage, deployment, and multi-state desktop acceptance.

## What Changes

- Add recoverable BlobGC claim leases with startup reconciliation and token-bound deletion.
- Extend metadata export/import/verify to include workspace authorization and receipt state, with deterministic fingerprints in addition to counts.
- Add a dual-database backend verification path that runs the existing focused suites against disposable SQLite and PostgreSQL databases.
- Record the local evidence boundary clearly; this does not enable production cloud writes or deployment.

## Capabilities

### New Capabilities

- `backend-reliability`: Crash-safe BlobGC, complete metadata migration verification, and dual-database verification.

### Modified Capabilities

- `team-runtime`: Metadata migration verification must detect content and workspace-state drift, not only row counts.

## Impact

- `server/internal/service` Blob and metadata migration models/services.
- `server/cmd/drivebridge-server` migration target schema setup.
- Root Makefile and server verification scripts.
- New focused evidence under `temp/integration-test-runs/`.
