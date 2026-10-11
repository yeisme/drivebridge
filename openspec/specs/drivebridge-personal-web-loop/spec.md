# drivebridge-personal-web-loop Specification

## Purpose
让用户在个人文件网页中定位、读取和归档 Agent 交付的结果，保留来源说明、既有文件身份和固定版本引用。重试沿原传输身份恢复，不将文件服务扩展为内容生成任务系统。
## Requirements
### Requirement: Personal file loop preserves ownership and compatibility
The system SHALL retain existing file identities, version references and authorization while adding the personal Agent file loop through the owning application service.

#### Scenario: Store a result with optional provenance
- **WHEN** an authorized caller saves a report, build artifact or creative file with caller and task labels
- **THEN** the result can be located and retrieved with its applicable version and provenance without creating a domain generation task

#### Scenario: Recover without changing identity
- **WHEN** a transfer is retried with the original inputs and idempotency key
- **THEN** the original operation is reused and a different existing file is not silently overwritten

#### Scenario: Preserve pinned versions and boundaries
- **WHEN** a caller requests a pinned reference or an unavailable storage capability
- **THEN** the selected version is respected or an explicit error is returned without changing identity or widening permission

#### Scenario: Distinguish offline evidence from live readiness
- **WHEN** local fixtures and automated checks pass
- **THEN** the system records offline evidence without claiming live account, deployment or market validation

