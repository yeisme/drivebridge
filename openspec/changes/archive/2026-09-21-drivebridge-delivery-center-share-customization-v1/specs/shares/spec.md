# 分享交付合同

## ADDED Requirements

### Requirement: Permanent shares
The share creation endpoint MUST accept `days=0` and treat it as a share without automatic expiry.

#### Scenario: Create and open a permanent share
- **WHEN** an authenticated owner creates a valid share with `days=0`
- **THEN** the response contains `is_permanent=true`
- **AND** the share remains accessible until revoked

### Requirement: Controlled presentation
The share creation endpoint MUST accept an optional presentation object with allowlisted preset, font, accent color, logo reference, and welcome message values.

#### Scenario: Reject invalid presentation
- **WHEN** a request contains an unsupported preset, font, color format, or overlong text
- **THEN** the service rejects the request with `invalid_share`

### Requirement: Recipient validation
The service MUST validate every specified recipient against the caller's tenant membership during creation and access.

#### Scenario: Recipient loses membership
- **WHEN** a specified user is no longer an allowed member
- **THEN** that user cannot open the share even if the grant is otherwise valid
