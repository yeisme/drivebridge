## Context

DriveBridge uses GORM with pure-Go SQLite for the personal service and PostgreSQL for the team deployment. BlobGC currently marks a zero-reference blob as claimed, performs an external delete, and removes the row. A process crash between those steps leaves the claim without a recovery identity. Metadata migration currently serializes core file tables but verifies mainly counts, so equal counts can hide state drift and workspace authorization rows are omitted.

## Goals / Non-Goals

**Goals:**

- Make BlobGC claims recoverable after process restart without allowing a late worker to delete a new reference.
- Export, import, and verify workspace grant, request, submission, credential, suggestion, and invocation state.
- Detect deterministic content drift, not only count drift.
- Provide one repeatable backend test contract for SQLite and disposable PostgreSQL.

**Non-Goals:**

- Production deployment, real cloud credentials, object-byte migration, or cross-platform desktop release.
- Replacing the existing GORM application layer or introducing a new queue/database dependency.
- Changing existing HTTP paths or legacy operation identifiers.

## Decisions

1. **Token-bound BlobGC leases.** Add nullable claim timestamp and opaque claim token fields. Startup runs after the process lock and clears claims left by the previous process. Each delete and final row removal matches the token, so a stale worker cannot clear or delete a newer claim. Dry-run releases its own claim immediately.
2. **Additive migration and deterministic fingerprints.** Keep existing JSONL kinds and add workspace kinds. Verification preserves count summaries for compatibility and adds SHA-256 fingerprints over canonical JSON arrays. Import remains all-or-nothing and refuses a non-empty target.
3. **Two database modes, one test suite.** The existing fixture receives a database selection hook. SQLite remains the default; PostgreSQL is an explicit disposable mode controlled by environment and never silently falls back. Tests use the same service/application contracts in both modes.
4. **Evidence separation.** Focused tests use `scripts/check.py`; generated evidence records database mode, command, and redacted output. Passing local fixtures does not imply production storage or deployment readiness.

## Risks / Trade-offs

- [External delete is not transactional] → Re-run is idempotent at the object key boundary and metadata deletion remains token/CAS guarded; unresolved storage errors keep the row recoverable.
- [PostgreSQL availability varies by host] → PostgreSQL verification is an explicit opt-in and reports unavailable instead of falling back to SQLite.
- [Fingerprint payloads grow with metadata] → JSONL remains streamed; fingerprints are computed per bounded table slice in memory only for the already decoded migration snapshot.

## Migration Plan

1. Deploy additive Blob columns and workspace export/import handlers.
2. On startup reconcile old GC claims after acquiring the state lock.
3. Run focused SQLite tests, then disposable PostgreSQL tests with the same filters.
4. Keep old snapshot readers and legacy API fields; rollback only disables the new verification path and retains additive columns.

## Open Questions

- Real object-store delete idempotency and PostgreSQL production sizing remain separate acceptance gates.
