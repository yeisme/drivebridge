# Proposal

DriveBridge currently exposes remote transfer, response-lost recovery, part
uploads, and transfer-plan persistence through several packages. Without one
owner contract, CLI, MCP, and service callers can drift in idempotency,
certainty, and resume behavior.

This change freezes one modern transfer entry point with durable operation
identity, verified resumable parts, final digest readback, and explicit
completed, failed, interrupted, and unknown outcomes. A response-lost request
must reconcile the original operation and idempotency key; it must not create a
replacement operation.

Local SQLite/loopback remains available through an explicit --no-remote path.
Service mode uses PostgreSQL-backed truth and asynchronous workers. This round
documents and verifies the existing contract; production deployment,
credentials, object-store setup, and cross-device validation remain separate
gates.
