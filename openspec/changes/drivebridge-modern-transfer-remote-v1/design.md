# Design

## Ownership and surfaces

cli/drivebridge is the sole owner. The Go CLI, MCP server, HTTP/GORM service,
worker, web client, and Electron desktop host share one application contract.
MCP actions are registered only when the remote surface advertises the
capability. Bytes move through bounded HTTP part endpoints, never through MCP
text payloads.

## Lifecycle

remote profile -> create operation (idempotency key + request fingerprint) ->
queued / transferring -> verified parts + final digest readback -> completed |
interrupted | failed | unknown -> status / resume / reconcile

The operation records its original source reference, target reference, source
SHA-256, target revision/version, idempotency key, request fingerprint, and
result certainty before the remote write. A response interruption is resolved
by querying the original operation and key. The client never retries with a
new key, source, target, provider, or operation identity.

Service mode stores durable transfer truth in PostgreSQL and workers claim
leases asynchronously. Local SQLite is a development/fixture path selected by
--no-remote; it is not evidence of remote or production acceptance.

## CLI contract

    drivebridge upload --remote-url <url> --remote-token-file <0600-file> \
      --local-root <root> --space <space> --file <path> \
      --idempotency-key <key>
    drivebridge operation --remote-url <url> --remote-token-file <0600-file> \
      --id <operation-id>
    drivebridge resume --remote-url <url> --remote-token-file <0600-file> \
      --id <operation-id>
    drivebridge cancel --remote-url <url> --remote-token-file <0600-file> \
      --id <operation-id>

The remote profile validates an absolute local root and a regular token file;
secrets are never copied to logs or evidence. Upload, operation, resume, and
cancel return the same operation IDs and certainty values used by the service
and MCP projections.

## Compatibility boundary

rclone may import or bridge legacy paths, but new canonical operations,
idempotency records, part acknowledgements, digest verification, and recovery
state remain in DriveBridge application service and GORM models.
