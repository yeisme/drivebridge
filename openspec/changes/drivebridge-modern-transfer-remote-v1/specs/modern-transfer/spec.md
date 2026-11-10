# Modern remote transfer

## ADDED Requirements

### Requirement: durable operation identity
The service MUST persist the original operation identity, idempotency key,
request fingerprint, source digest, target reference/version, and result
certainty before issuing a remote write.

#### Scenario: response lost after submit
- WHEN the submit response is interrupted or times out
- THEN the client queries the original operation and idempotency key
- AND it does not create a second operation or change the provider, source,
  target, or key

### Requirement: resumable verified transfer
The service MUST resume from durable, verified part acknowledgements and MUST
verify the final target digest before reporting completed.

#### Scenario: interrupted upload
- WHEN a worker stops after some parts are acknowledged
- THEN a later resume reuses only verified parts
- AND a digest mismatch yields failed or unknown, never success

### Requirement: shared projections
The CLI and MCP surfaces MUST expose the same operation IDs, state, certainty,
and recovery actions. MCP MUST NOT register an action that the remote surface
does not advertise.

#### Scenario: capability absent
- WHEN a remote profile lacks a surface capability
- THEN the corresponding MCP action is not registered
- AND CLI discovery reports the capability as unavailable

### Requirement: rclone compatibility boundary
rclone MAY bridge legacy paths, but MUST NOT own new canonical operation state,
idempotency, digest, or recovery records.

#### Scenario: legacy import
- WHEN a legacy rclone path is imported
- THEN DriveBridge creates the canonical operation and digest records
- AND subsequent status/resume/reconcile uses DriveBridge state
