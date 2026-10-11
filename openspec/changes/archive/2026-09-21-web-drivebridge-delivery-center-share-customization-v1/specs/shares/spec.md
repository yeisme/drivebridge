# 分享交付界面

## ADDED Requirements

### Requirement: Delivery center navigation
The client MUST expose one delivery center entry with views for transfer tasks, pending artifacts, and share delivery.

#### Scenario: Open delivery center
- **WHEN** the user selects 交付中心
- **THEN** the client shows transfer tasks and navigation tabs for pending artifacts and share delivery

### Requirement: Permanent and recipient controls
The share dialog MUST provide a permanent option and recipient selection by validated user ID.

#### Scenario: Select recipients and permanent expiry
- **WHEN** the user selects 指定用户 and 永久有效, then validates one or more IDs
- **THEN** the selected IDs appear as removable tags and the request submits `days=0`

### Requirement: Share presentation preview
The share dialog MUST provide local preview controls for theme, font, accent color, logo reference, and welcome message.

#### Scenario: Preview custom share page
- **WHEN** the user changes presentation controls
- **THEN** the preview updates without a network request
