# DriveBridge modern transfer and remote profile

This owner-local change records the canonical DriveBridge contract for remote,
resumable transfers. DriveBridge owns the CLI, MCP surface, HTTP/GORM service,
and worker recovery path; the web and desktop clients consume the same typed
operation projections.

The existing implementation already contains the remote adapter, operation
journal, bounded part upload, digest checks, and transfer-plan service models.
This change documents that boundary and the remaining service-mode work. It
does not authorize deployment, credential changes, or remote writes.

rclone remains a compatibility import/legacy adapter. It is not the owner of
canonical operation state or recovery semantics.
