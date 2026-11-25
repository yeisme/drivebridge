## ADDED Requirements

### Requirement: Path-free typed agent attribution and gateway call correlation on operations
`GET /api/v1/operations` SHALL expose optional `actor_type` (`human | agent | service | unknown`) derived from the authenticated credential/surface and optional validated `origin_refs` (`gateway_call_ref`, `mcp_session_ref`) without exposing local paths, filenames, or temporary download URLs.

#### Scenario: File transfer initiated by Agent token via MCP Gateway
- **WHEN** a space-scoped Agent token initiates an upload or transfer carrying a valid `X-Gateway-Call-Ref`
- **THEN** `GET /api/v1/operations` includes `actor_type: "agent"` and `origin_refs.gateway_call_ref` for authorized space readers
- **AND** legacy operations without correlation metadata remain valid and omit `origin_refs`
