# DriveBridge 实现盘点（脚本生成）

由 `python3 scripts/implementation_inventory.py --write` 生成；不可手改状态。

## 判读规则

- 这是静态源码和已有验收任务的盘点，不是运行时 capability 或发布承诺。
- “已发现”只证明指定代码/字面路由存在；不证明可达、授权正确、视觉一致或完整闭环。
- 专属页面缺失不代表旧界面没有同类操作；必须迁移时保留旧功能。
- 任务计数不是完成百分比；细分任务通过不自动关闭父任务。
- Electron/Pinax/三平台须独立真实验收，不能由普通文件或 fixture 通过推导。

## 页面 → 正式路由 → 当前组件 → 验收责任

| 页面/操作组 | 当前入口 | 指定组件 | 验收任务 |
| --- | --- | --- | --- |
| 工作台 | `#/home` | 已发现：`web/src/info-workspace/InfoWorkspace.tsx` | F21.1/B03.1 |
| 网盘 | `#/files` | 已发现：`web/src/features/files/FilesPage.tsx` | F04.1/F04.2 |
| 上传与恢复 | `#/flows` | 已发现：`web/src/consumer-flows/ConsumerFlows.tsx` | F05.1/F05.2 |
| 知识库 | `#/knowledge` | 已发现：`web/src/features/notebook/KnowledgePage.tsx` | F07.1/F25.2 |
| 文档编辑 | `#/knowledge` | 已发现：`web/src/features/notebook/MarkdownFileDocument.tsx` | F07.2/F26.1/B17.2 |
| 资源库 | `#/resources` | 已发现：`web/src/features/resources/ResourcesPage.tsx` | F08/B10 |
| 集合 | `#/collections` | 已发现：`web/src/features/collections/CollectionsPage.tsx` | F09.1/B09.1 |
| 标签与属性 | `#/properties` | 已发现：`web/src/features/properties/PropertiesPage.tsx` | F09.1/B09.2 |
| 搜索 | `#/search` | 已发现：`web/src/consumer-flows/FlowPage.tsx` | F09.2 |
| Agent | `#/agents` | 已发现：`web/src/features/agents/LegacyAgentPage.tsx` | F10/F20/B04/B15 |
| 任务 | `#/tasks` | 已发现：`web/src/features/tasks/LegacyTransfersPage.tsx` | F11/B03.2 |
| 收件 | `#/inbox` | 已发现：`web/src/features/tasks/LegacyInboxPage.tsx` | F12/B07/B08 |
| 存储 | `#/storage` | 已发现：`web/src/features/storage/ConnectionsPage.tsx` | F13 |
| 设备与离线 | `#/devices` | 已发现：`web/src/features/devices/DevicesPage.tsx` | F15.1 |
| 自动化 | `#/automation` | 已发现：`web/src/features/automation/AutomationPage.tsx` | F15.2 |
| 回收站 | `#/trash` | 已发现：`web/src/features/trash/TrashPage.tsx` | F21.2 |
| 能力说明 | `#/capabilities` | 已发现：`web/src/features/capabilities/CapabilitiesPage.tsx` | F21.2/B11 |
| 设置 | `#/settings` | 已发现：`web/src/PersonalSettings.tsx` | F21.2 |
| 分享 | `#/shares` | 已发现：`web/src/Sharing.tsx` | F14 |
| 团队 | `#/teams` | 已发现：`web/src/Teams.tsx` | F16.1 |
| 治理 | `#/governance` | 已发现：`web/src/Governance.tsx` | F16.1 |
| 访问凭据 | `#/members` | 已发现：`web/src/KeyDirectory.tsx` | F16.1 |

## 新工作区合同 → 字面 HTTP 注册 → Web 消费线索

以下扫描只覆盖现有 Go Route 字面声明和 `mux.HandleFunc`；动态注册不在此静态扫描范围。

| 方法/路径 | 服务注册文件 | 非生成 Web 路径字面量 |
| --- | --- | --- |
| `DELETE /api/v1/agent-groups/{id}` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `DELETE /api/v1/favorites/{id}` | `server/internal/service/workspace_favorite_http.go` | `web/src/features/notebook/NotebookFavorite.tsx` |
| `GET /api/v1/agent-grants` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `GET /api/v1/agent-grants/{id}` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `GET /api/v1/agent-groups` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `GET /api/v1/agent-groups/{id}` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `GET /api/v1/agent-invocations` | `server/internal/service/http.go` | `web/src/features/agents/AgentInvocations.tsx` |
| `GET /api/v1/agent-organizations` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `GET /api/v1/agent-organizations/{agent_id}` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `GET /api/v1/favorites` | `server/internal/service/workspace_favorite_http.go` | `web/src/features/notebook/NotebookFavorite.tsx` |
| `GET /api/v1/file-collections/{id}/members/decisions` | `server/internal/service/organization_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/collections/CollectionMembersDialog.tsx`, `web/src/features/collections/CreateCollectionDialog.tsx`, `web/src/features/collections/CreateSnapshotDialog.tsx`, `web/src/features/collections/SnapshotHistory.tsx`, `web/src/features/home/HomeCollection.tsx` |
| `GET /api/v1/file-collections/{id}/snapshots` | `server/internal/service/organization_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/collections/CollectionMembersDialog.tsx`, `web/src/features/collections/CreateCollectionDialog.tsx`, `web/src/features/collections/CreateSnapshotDialog.tsx`, `web/src/features/collections/SnapshotHistory.tsx`, `web/src/features/home/HomeCollection.tsx` |
| `GET /api/v1/files/{id}/backlinks` | 未在字面注册中发现 | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `GET /api/v1/files/{id}/document-capability` | `server/internal/service/document.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `GET /api/v1/files/{id}/label-suggestions` | `server/internal/service/organization_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `GET /api/v1/files/{id}/links` | 未在字面注册中发现 | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `GET /api/v1/files/{id}/properties` | `server/internal/service/workspace_property_sidecar_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `GET /api/v1/files/{id}/retention-holds` | `server/internal/service/workspace_hold_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `GET /api/v1/inbox-submissions` | `server/internal/service/http.go` | `web/src/features/inbox/submission-client.ts` |
| `GET /api/v1/inbox-submissions/{id}` | `server/internal/service/http.go` | `web/src/features/inbox/submission-client.ts` |
| `GET /api/v1/organization-plans/{id}` | `server/internal/service/agentorg_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/properties/LabelDictionary.tsx` |
| `GET /api/v1/property-migrations/{id}` | `server/internal/service/workspace_property_migration_http.go` | `web/src/features/properties/PropertyMigration.tsx` |
| `GET /api/v1/resource-descriptors` | `server/internal/service/workspace_resource_http.go` | `web/src/features/resources/RegisterResourceDialog.tsx`, `web/src/features/resources/ResourceIntegrity.tsx`, `web/src/features/resources/ResourcesPage.tsx` |
| `GET /api/v1/resource-descriptors/{id}` | `server/internal/service/workspace_resource_http.go` | `web/src/features/resources/RegisterResourceDialog.tsx`, `web/src/features/resources/ResourceIntegrity.tsx`, `web/src/features/resources/ResourcesPage.tsx` |
| `GET /api/v1/spaces/{id}/automation-management` | `server/internal/service/automation_management_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `GET /api/v1/spaces/{id}/label-definitions` | `server/internal/service/workspace_label_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `GET /api/v1/spaces/{id}/property-definitions` | `server/internal/service/workspace_property_definitions_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `GET /api/v1/spaces/{id}/property-migrations/capability` | `server/internal/service/workspace_property_migration_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `GET /api/v1/workspace/home` | `server/internal/service/http.go` | `web/src/features/home/WorkspaceHome.tsx` |
| `GET /api/v1/workspace/tasks` | `server/internal/service/http.go` | `web/src/features/tasks/BatchTaskItems.tsx`, `web/src/features/tasks/OperationTaskDetail.tsx`, `web/src/features/tasks/OperationTasks.tsx` |
| `GET /api/v1/workspace/tasks/upload_batch/{id}/items` | `server/internal/service/http.go` | `web/src/features/tasks/BatchTaskItems.tsx` |
| `GET /api/v1/workspace/tasks/{kind}/{id}` | `server/internal/service/http.go` | `web/src/features/tasks/BatchTaskItems.tsx`, `web/src/features/tasks/OperationTaskDetail.tsx` |
| `PATCH /api/v1/agent-grants/{id}` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `PATCH /api/v1/agent-groups/{id}` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `PATCH /api/v1/agent-organizations/{agent_id}` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `PATCH /api/v1/files/{id}/properties` | `server/internal/service/workspace_property_sidecar_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/agent-grants` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `POST /api/v1/agent-grants/lookup` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `POST /api/v1/agent-grants/{id}/changes/lookup` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `POST /api/v1/agent-grants/{id}/checks` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `POST /api/v1/agent-grants/{id}/revoke` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `POST /api/v1/agent-grants/{id}/rotate` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `POST /api/v1/agent-grants/{id}/rotate/lookup` | `server/internal/service/http.go` | `web/src/features/agents/grant-client.ts` |
| `POST /api/v1/agent-groups` | `server/internal/service/http.go` | `web/src/features/agents/organization-client.ts` |
| `POST /api/v1/automation-rules/{id}/execute` | `server/internal/service/automation_management_http.go` | `web/src/features/automation/AutomationActionDialog.tsx` |
| `POST /api/v1/automation-rules/{id}/execute/lookup` | 未在字面注册中发现 | `web/src/features/automation/AutomationActionDialog.tsx` |
| `POST /api/v1/automation-rules/{id}/preview` | `server/internal/service/automation_management_http.go` | `web/src/features/automation/AutomationActionDialog.tsx` |
| `POST /api/v1/favorites` | `server/internal/service/workspace_favorite_http.go` | `web/src/features/notebook/NotebookFavorite.tsx` |
| `POST /api/v1/favorites/lookup` | `server/internal/service/workspace_favorite_http.go` | `web/src/features/notebook/NotebookFavorite.tsx` |
| `POST /api/v1/file-collections` | `server/internal/service/organization_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/collections/CollectionMembersDialog.tsx`, `web/src/features/collections/CollectionsPage.tsx`, `web/src/features/collections/CreateCollectionDialog.tsx`, `web/src/features/collections/CreateSnapshotDialog.tsx`, `web/src/features/collections/SnapshotHistory.tsx`, `web/src/features/home/HomeCollection.tsx` |
| `POST /api/v1/file-collections/lookup` | `server/internal/service/organization_http.go` | `web/src/features/collections/CreateCollectionDialog.tsx` |
| `POST /api/v1/file-collections/{id}/members/changes` | `server/internal/service/organization_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/collections/CollectionMembersDialog.tsx`, `web/src/features/collections/CreateCollectionDialog.tsx`, `web/src/features/collections/CreateSnapshotDialog.tsx`, `web/src/features/collections/SnapshotHistory.tsx`, `web/src/features/home/HomeCollection.tsx` |
| `POST /api/v1/file-collections/{id}/members/changes/lookup` | `server/internal/service/organization_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/collections/CollectionMembersDialog.tsx`, `web/src/features/collections/CreateCollectionDialog.tsx`, `web/src/features/collections/CreateSnapshotDialog.tsx`, `web/src/features/collections/SnapshotHistory.tsx`, `web/src/features/home/HomeCollection.tsx` |
| `POST /api/v1/file-collections/{id}/snapshots/creations` | `server/internal/service/organization_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/collections/CollectionMembersDialog.tsx`, `web/src/features/collections/CreateCollectionDialog.tsx`, `web/src/features/collections/CreateSnapshotDialog.tsx`, `web/src/features/collections/SnapshotHistory.tsx`, `web/src/features/home/HomeCollection.tsx` |
| `POST /api/v1/file-collections/{id}/snapshots/creations/lookup` | `server/internal/service/organization_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/collections/CollectionMembersDialog.tsx`, `web/src/features/collections/CreateCollectionDialog.tsx`, `web/src/features/collections/CreateSnapshotDialog.tsx`, `web/src/features/collections/SnapshotHistory.tsx`, `web/src/features/home/HomeCollection.tsx` |
| `POST /api/v1/files/{id}/document/lookup` | `server/internal/service/document.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/files/{id}/label-suggestions` | `server/internal/service/organization_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/files/{id}/links/lookup` | `server/internal/service/workspace_link_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/files/{id}/links/rebuild` | `server/internal/service/workspace_link_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/files/{id}/properties/lookup` | `server/internal/service/workspace_property_sidecar_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/files/{id}/retention-holds` | `server/internal/service/workspace_hold_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/files/{id}/retention-holds/lookup` | `server/internal/service/workspace_hold_http.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `POST /api/v1/inbox-submissions` | `server/internal/service/http.go` | `web/src/features/inbox/submission-client.ts` |
| `POST /api/v1/inbox-submissions/lookup` | `server/internal/service/http.go` | 未发现（不含生成类型；动态构造需人工复核） |
| `POST /api/v1/inbox-submissions/{id}/archive` | `server/internal/service/http.go` | `web/src/features/inbox/submission-client.ts` |
| `POST /api/v1/inbox-submissions/{id}/decision` | `server/internal/service/http.go` | `web/src/features/inbox/submission-client.ts` |
| `POST /api/v1/label-suggestions/{id}/decision` | `server/internal/service/organization_http.go` | `web/src/features/files/suggestion-client.ts` |
| `POST /api/v1/organization-plans/{id}/confirm` | `server/internal/service/agentorg_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/properties/LabelDictionary.tsx` |
| `POST /api/v1/organization-plans/{id}/execute` | `server/internal/service/agentorg_http.go` | `web/src/consumer-flows/client.ts`, `web/src/features/properties/LabelDictionary.tsx` |
| `POST /api/v1/property-migrations/{id}/confirm` | 未在字面注册中发现 | `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/property-migrations/{id}/execute` | 未在字面注册中发现 | `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/resource-descriptors` | `server/internal/service/workspace_resource_http.go` | `web/src/features/resources/RegisterResourceDialog.tsx`, `web/src/features/resources/ResourceIntegrity.tsx`, `web/src/features/resources/ResourcesPage.tsx` |
| `POST /api/v1/resource-descriptors/lookup` | `server/internal/service/workspace_resource_http.go` | `web/src/features/resources/RegisterResourceDialog.tsx` |
| `POST /api/v1/resource-descriptors/{id}/checks` | `server/internal/service/workspace_resource_http.go` | `web/src/features/resources/RegisterResourceDialog.tsx`, `web/src/features/resources/ResourceIntegrity.tsx`, `web/src/features/resources/ResourcesPage.tsx` |
| `POST /api/v1/resource-descriptors/{id}/checks/lookup` | `server/internal/service/workspace_resource_http.go` | `web/src/features/resources/RegisterResourceDialog.tsx`, `web/src/features/resources/ResourceIntegrity.tsx`, `web/src/features/resources/ResourcesPage.tsx` |
| `POST /api/v1/retention/holds/{id}/release-cas` | `server/internal/service/workspace_hold_http.go` | `web/src/features/trash/TrashHolds.tsx` |
| `POST /api/v1/spaces/{id}/automation-rules/changes` | `server/internal/service/automation_management_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/spaces/{id}/automation-rules/changes/lookup` | 未在字面注册中发现 | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/spaces/{id}/label-definitions/lookup` | `server/internal/service/workspace_label_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/spaces/{id}/label-merge-plans` | `server/internal/service/workspace_label_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/spaces/{id}/label-merge-plans/lookup` | `server/internal/service/workspace_label_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/spaces/{id}/property-definitions/lookup` | `server/internal/service/workspace_property_definitions_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/spaces/{id}/property-migrations` | `server/internal/service/workspace_property_migration_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `POST /api/v1/spaces/{id}/property-migrations/lookup` | `server/internal/service/workspace_property_migration_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `PUT /api/v1/files/{id}/document` | `server/internal/service/document.go` | `web/src/Finder.tsx`, `web/src/LocalWorkspace.tsx`, `web/src/Media.tsx`, `web/src/Sharing.tsx`, `web/src/Workspace.tsx`, `web/src/WorkspaceApp.tsx`, `web/src/api.ts`, `web/src/consumer-flows/client.ts`, `web/src/features/files/suggestion-client.ts`, `web/src/features/inbox/SubmissionPreview.tsx`, `web/src/features/inbox/submission-file.ts`, `web/src/features/notebook/document-client.ts`, `web/src/features/notebook/notebook-adapters.ts`, `web/src/features/notebook/notebook-relations.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/trash/TrashHolds.tsx`, `web/src/info-workspace/client.ts` |
| `PUT /api/v1/spaces/{id}/label-definitions` | `server/internal/service/workspace_label_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |
| `PUT /api/v1/spaces/{id}/property-definitions` | `server/internal/service/workspace_property_definitions_http.go` | `web/src/WorkspaceApp.tsx`, `web/src/consumer-flows/client.ts`, `web/src/features/automation/AutomationActionDialog.tsx`, `web/src/features/automation/AutomationPage.tsx`, `web/src/features/automation/client.ts`, `web/src/features/properties/FileProperties.tsx`, `web/src/features/properties/LabelDictionary.tsx`, `web/src/features/properties/PropertiesPage.tsx`, `web/src/features/properties/PropertyEditor.tsx`, `web/src/features/properties/PropertyMigration.tsx` |

## 既有任务状态（按 owner CLI 维护）

| change | 已勾选/总数 |
| --- | --- |
| `drivebridge-personal-workspace-rebuild-v1` | 126/167 |
| `drivebridge-personal-workspace-contracts-v1` | 75/101 |
| `drivebridge-desktop-knowledge-reader-v1` | 24/42 |

Electron `desktop/`：目录存在，尚需逐项验收。

## 定向测试发现与证据

静态发现 177 个 `TestWorkspace*` / `TestDocument*` 函数；存在测试不表示测试通过。
本报告未附本次运行证据；历史记录以任务与 owner evidence 为准。

| 测试文件 | 已声明测试 |
| --- | --- |
| `server/internal/service/document_test.go` | `TestDocumentSaveHTTPVersionsReplayAndIsolation` |
| `server/internal/service/document_test.go` | `TestDocumentConcurrentCASAndHumanGate` |
| `server/internal/service/workspace_archive_bind_test.go` | `TestWorkspaceArchiveAllocationBindsSubmissionAtomically` |
| `server/internal/service/workspace_archive_conflict_test.go` | `TestWorkspaceArchiveConflictDoesNotRenameOrReserve` |
| `server/internal/service/workspace_archive_resume_test.go` | `TestWorkspaceArchiveResumeAfterReadbackFailure` |
| `server/internal/service/workspace_auth_guard_test.go` | `TestWorkspaceIdentityCannotFallThroughLegacyAuthentication` |
| `server/internal/service/workspace_auth_guard_test.go` | `TestWorkspaceProfileMarkerRejectsMissingGrantAndStalePrincipal` |
| `server/internal/service/workspace_auth_guard_test.go` | `TestWorkspaceLegacyGuardRejectsMissingIdentity` |
| `server/internal/service/workspace_auth_guard_test.go` | `TestWorkspaceLegacyGuardFailsClosedOnDatabaseError` |
| `server/internal/service/workspace_authorize_test.go` | `TestWorkspaceDedicatedActionGate` |
| `server/internal/service/workspace_automation_tasks_test.go` | `TestWorkspaceAutomationIntentProjection` |
| `server/internal/service/workspace_automation_tasks_test.go` | `TestWorkspaceAutomationIntentBudgetAndMissingDependency` |
| `server/internal/service/workspace_batch_task_items_boundary_test.go` | `TestWorkspaceBatchTaskItemsBudgetAndIdentity` |
| `server/internal/service/workspace_batch_task_items_test.go` | `TestWorkspaceBatchTaskItems` |
| `server/internal/service/workspace_batch_tasks_boundary_test.go` | `TestWorkspaceBatchTaskBoundaries` |
| `server/internal/service/workspace_batch_tasks_boundary_test.go` | `TestWorkspaceBatchTaskRequestItemBudget` |
| `server/internal/service/workspace_batch_tasks_test.go` | `TestWorkspaceBatchTaskProjection` |
| `server/internal/service/workspace_contract_compat_test.go` | `TestWorkspaceContractExistingErrorEnvelope` |
| `server/internal/service/workspace_contract_compat_test.go` | `TestWorkspaceContractExistingSuccessEnvelope` |
| `server/internal/service/workspace_credential_model_test.go` | `TestWorkspaceCredentialHistoryConstraints` |
| `server/internal/service/workspace_downgrade_check_test.go` | `TestWorkspaceDowngradeRequiresExplicitGrantRetirement` |
| `server/internal/service/workspace_downgrade_test.go` | `TestWorkspaceCredentialLifecycleKeepsLegacyColumnsClosed` |
| `server/internal/service/workspace_download_failure_test.go` | `TestWorkspaceDownloadSpaceFailureClosesBody` |
| `server/internal/service/workspace_download_http_test.go` | `TestWorkspaceFixedDownloadHTTPRequiresReadAndVersion` |
| `server/internal/service/workspace_download_test.go` | `TestWorkspaceDownloadUsesStoredBytesAndCurrentAuthority` |
| `server/internal/service/workspace_favorite_budget_test.go` | `TestWorkspaceFavoriteDefaultMaximumPagesAndBudget` |
| `server/internal/service/workspace_favorite_http_test.go` | `TestWorkspaceFavoriteHTTPIdentityAndRollback` |
| `server/internal/service/workspace_favorite_identity_test.go` | `TestWorkspaceFavoriteDirectoryRootAndTagIdentity` |
| `server/internal/service/workspace_favorite_model_test.go` | `TestWorkspaceFavoriteIdentityIncludesScopeAndCanonicalTarget` |
| `server/internal/service/workspace_favorite_page_test.go` | `TestWorkspaceFavoriteHTTPPaginationAndCursorBinding` |
| `server/internal/service/workspace_favorite_recovery_test.go` | `TestWorkspaceFavoriteConcurrentCASAndRestart` |
| `server/internal/service/workspace_favorite_test.go` | `TestWorkspaceFavoriteCASRecoveryIsolation` |
| `server/internal/service/workspace_file_read_test.go` | `TestWorkspaceFixedVersionReaderRechecksAuthority` |
| `server/internal/service/workspace_file_search_test.go` | `TestWorkspaceFileSearchScopeAndStagingIsolation` |
| `server/internal/service/workspace_file_stat_test.go` | `TestWorkspaceStatPreservesCurrentMetadataAndPinsHistoricalMetadata` |
| `server/internal/service/workspace_governance_failure_test.go` | `TestWorkspaceGovernanceSpaceFailureIsNotVisibilityDenial` |
| `server/internal/service/workspace_grant_cas_test.go` | `TestWorkspaceGrantConcurrentRevisionSingleWinner` |
| `server/internal/service/workspace_grant_cas_test.go` | `TestWorkspaceGrantCASAndRollback` |
| `server/internal/service/workspace_grant_change_lookup_test.go` | `TestWorkspaceGrantChangeLookupOriginalReceipt` |
| `server/internal/service/workspace_grant_checks_test.go` | `TestWorkspaceGrantChecksLocalOnlyAndRevisionBound` |
| `server/internal/service/workspace_grant_combined_test.go` | `TestWorkspaceGrantCombinedPatchAtomicHTTP` |
| `server/internal/service/workspace_grant_create_http_test.go` | `TestWorkspaceGrantCreateHTTP` |
| `server/internal/service/workspace_grant_create_test.go` | `TestWorkspaceGrantCreationDurationMatrix` |
| `server/internal/service/workspace_grant_create_test.go` | `TestWorkspaceGrantCreationAtomicIdentityAndReplay` |
| `server/internal/service/workspace_grant_expiry_http_test.go` | `TestWorkspaceGrantExpiryHTTP` |
| `server/internal/service/workspace_grant_expiry_test.go` | `TestWorkspaceExpiryAndRevocationCompeteOnSameRevision` |
| `server/internal/service/workspace_grant_expiry_test.go` | `TestWorkspaceExpiryChangeAndReplay` |
| `server/internal/service/workspace_grant_list_test.go` | `TestWorkspaceGrantListFiltersAndCursor` |
| `server/internal/service/workspace_grant_name_test.go` | `TestWorkspaceGrantRenameHTTPPreservesCredentialAndExpiry` |
| `server/internal/service/workspace_grant_organization_list_test.go` | `TestWorkspaceGrantOrganizationProjection` |
| `server/internal/service/workspace_grant_patch_test.go` | `TestWorkspaceGrantPatchEnableDisableHTTP` |
| `server/internal/service/workspace_grant_read_test.go` | `TestWorkspaceGrantReadHTTPIsolation` |
| `server/internal/service/workspace_grant_request_test.go` | `TestWorkspaceGrantRequestReplayAndAtomicRollback` |
| `server/internal/service/workspace_grant_revoke_test.go` | `TestWorkspaceGrantRevokeHTTPTransaction` |
| `server/internal/service/workspace_grant_scope_http_test.go` | `TestWorkspaceGrantScopeHTTPAtomicReplay` |
| `server/internal/service/workspace_grant_scope_test.go` | `TestWorkspaceGrantScopePatchAtomicAuthority` |
| `server/internal/service/workspace_grant_state_test.go` | `TestWorkspaceGrantStateBoundaries` |
| `server/internal/service/workspace_group_create_http_test.go` | `TestWorkspaceGroupCreateHTTPReceiptsAndRollback` |
| `server/internal/service/workspace_group_delete_http_test.go` | `TestWorkspaceGroupDeleteHTTPAtomicReplay` |
| `server/internal/service/workspace_group_delete_test.go` | `TestWorkspaceGroupDeleteUnbindsWithoutChangingPinsOrAuthority` |
| `server/internal/service/workspace_group_http_test.go` | `TestWorkspaceGroupGetHTTPIsolation` |
| `server/internal/service/workspace_group_list_test.go` | `TestWorkspaceGroupListScopedPagination` |
| `server/internal/service/workspace_group_rename_http_test.go` | `TestWorkspaceGroupRenameHTTPRevisionAndReplay` |
| `server/internal/service/workspace_group_test.go` | `TestWorkspaceGroupNamesAndRevisionIsolation` |
| `server/internal/service/workspace_hold_capability_test.go` | `TestWorkspaceHoldCapabilitiesAndSourceRevoke` |
| `server/internal/service/workspace_hold_failure_test.go` | `TestWorkspaceHoldAtomicReceiptFailures` |
| `server/internal/service/workspace_hold_failure_test.go` | `TestWorkspaceHoldIsolationAndImmutableDigest` |
| `server/internal/service/workspace_hold_http_test.go` | `TestWorkspaceHoldHTTPAndSDK` |
| `server/internal/service/workspace_hold_legacy_provenance_test.go` | `TestWorkspaceHoldLegacyProvenanceFailureDeniesRelease` |
| `server/internal/service/workspace_hold_legacy_review_test.go` | `TestWorkspaceHoldLegacyReleaseCannotBypassManagedCAS` |
| `server/internal/service/workspace_hold_legacy_review_test.go` | `TestWorkspaceHoldLegacyOmittedVersionReplay` |
| `server/internal/service/workspace_hold_legacy_review_test.go` | `TestWorkspaceHoldLegacyReadSourceAuthority` |
| `server/internal/service/workspace_hold_read_test.go` | `TestWorkspaceHoldReadAndSourceFailures` |
| `server/internal/service/workspace_hold_read_test.go` | `TestWorkspaceHoldCurrentTargetMove` |
| `server/internal/service/workspace_hold_recovery_authority_test.go` | `TestWorkspaceHoldRecoveryRechecksCurrentSource` |
| `server/internal/service/workspace_hold_restart_test.go` | `TestWorkspaceHoldRestartOriginalReceipts` |
| `server/internal/service/workspace_hold_source_test.go` | `TestWorkspaceHoldShareBinding` |
| `server/internal/service/workspace_hold_source_test.go` | `TestWorkspaceHoldLegacyCurrentAgentAndTarget` |
| `server/internal/service/workspace_hold_source_test.go` | `TestWorkspaceHoldCurrentSourceManage` |
| `server/internal/service/workspace_hold_test.go` | `TestWorkspaceHoldRecoveryCAS` |
| `server/internal/service/workspace_hold_test.go` | `TestWorkspaceHoldSourceTargetAuthority` |
| `server/internal/service/workspace_hold_test.go` | `TestWorkspaceHoldListCurrentAuthorityCursor` |
| `server/internal/service/workspace_home_submission_test.go` | `TestWorkspaceHomeSubmissionOwnerFlow` |
| `server/internal/service/workspace_home_test.go` | `TestWorkspaceHomePublishedFilesAndIsolation` |
| `server/internal/service/workspace_home_test.go` | `TestWorkspaceHomeSectionFailures` |
| `server/internal/service/workspace_home_test.go` | `TestWorkspaceHomeBoundedSources` |
| `server/internal/service/workspace_invocation_list_test.go` | `TestWorkspaceInvocationListScopeTimeAndCursor` |
| `server/internal/service/workspace_invocation_list_test.go` | `TestWorkspaceInvocationListBudget` |
| `server/internal/service/workspace_invocation_order_test.go` | `TestWorkspaceInvocationStartedOrder` |
| `server/internal/service/workspace_invocation_state_test.go` | `TestWorkspaceInvocationConservativeOutcomeAndMigration` |
| `server/internal/service/workspace_invocation_test.go` | `TestWorkspaceInvocationObservedTransportsAndFailures` |
| `server/internal/service/workspace_label_http_test.go` | `TestWorkspaceLabelHTTPClosedReadOnlyAndDefaultGate` |
| `server/internal/service/workspace_label_lock_order_test.go` | `TestWorkspaceLabelMergeStableFileLockOrder` |
| `server/internal/service/workspace_label_merge_test.go` | `TestWorkspaceLabelMergeHTTPLifecycleAndUnknownRecovery` |
| `server/internal/service/workspace_label_merge_test.go` | `TestWorkspaceLabelMergeRollbackDriftAndRevocation` |
| `server/internal/service/workspace_label_partial_test.go` | `TestWorkspaceLabelMergePartialFrozenOutcomesAndScopedCursor` |
| `server/internal/service/workspace_label_recovery_test.go` | `TestWorkspaceLabelMergePreviewRollbackAndRestart` |
| `server/internal/service/workspace_label_test.go` | `TestWorkspaceLabelDictionaryCASPaginationRecovery` |
| `server/internal/service/workspace_label_test.go` | `TestWorkspaceLabelDictionaryAtomicRollback` |
| `server/internal/service/workspace_legacy_agents_test.go` | `TestWorkspaceLegacyAgentDefaultsAndArrayContract` |
| `server/internal/service/workspace_link_authority_test.go` | `TestWorkspaceLinksAuthorityAndBudgets` |
| `server/internal/service/workspace_link_budget_test.go` | `TestWorkspaceLinksDefaultMaximumPagesAndScanBudget` |
| `server/internal/service/workspace_link_http_test.go` | `TestWorkspaceLinksHTTPReadOnlyAndRecovery` |
| `server/internal/service/workspace_link_model_test.go` | `TestWorkspaceLinkProjectionPinsSourceVersionAndAnchor` |
| `server/internal/service/workspace_link_page_test.go` | `TestWorkspaceLinksHTTPPaginationAndCursorBinding` |
| `server/internal/service/workspace_link_parse_test.go` | `TestWorkspaceLinkParserBoundaries` |
| `server/internal/service/workspace_link_parse_test.go` | `TestWorkspaceLinkParserExcludesNestedCodeAndDefinitions` |
| `server/internal/service/workspace_link_revoke_test.go` | `TestWorkspaceLinksDownloadRevocationCommitsNothing` |
| `server/internal/service/workspace_link_test.go` | `TestWorkspaceLinksFixedVersionRebuildAndVisibility` |
| `server/internal/service/workspace_list_failure_test.go` | `TestWorkspaceGrantListReturnsAuthorizationStorageFailure` |
| `server/internal/service/workspace_migration_failure_test.go` | `TestWorkspaceMigrationFailureRollsBackAndRetries` |
| `server/internal/service/workspace_migration_process_test.go` | `TestWorkspaceMigrationRecoveryAcrossProcesses` |
| `server/internal/service/workspace_migration_process_test.go` | `TestWorkspaceMigrationProcessHelper` |
| `server/internal/service/workspace_models_test.go` | `TestWorkspaceMigrationPreservesLegacyAgent` |
| `server/internal/service/workspace_models_test.go` | `TestWorkspaceMigrationAfterPartialSchema` |
| `server/internal/service/workspace_models_test.go` | `TestWorkspaceGroupNamesAreScoped` |
| `server/internal/service/workspace_models_test.go` | `TestWorkspaceGrantExpiryConstraint` |
| `server/internal/service/workspace_models_test.go` | `TestWorkspaceOrganizationScopedUniqueness` |
| `server/internal/service/workspace_operation_authorize_test.go` | `TestWorkspaceOperationRequiresCurrentGrantAndSubmissionBinding` |
| `server/internal/service/workspace_operation_migration_test.go` | `TestWorkspaceOperationUpgradePreservesRecoveryIdentity` |
| `server/internal/service/workspace_operation_test.go` | `TestWorkspaceOperationBindingRejectsLegacyAccess` |
| `server/internal/service/workspace_organization_concurrency_test.go` | `TestWorkspaceConcurrentGroupDeleteAndAssignment` |
| `server/internal/service/workspace_organization_http_test.go` | `TestWorkspaceOrganizationHTTPReplayAndRollback` |
| `server/internal/service/workspace_organization_list_test.go` | `TestWorkspaceOrganizationListVirtualRowsAndCursor` |
| `server/internal/service/workspace_organization_test.go` | `TestWorkspaceOrganizationPatchPreservesPinsAndAuthority` |
| `server/internal/service/workspace_plan_tasks_test.go` | `TestWorkspaceOrganizationPlanProjection` |
| `server/internal/service/workspace_plan_tasks_test.go` | `TestWorkspaceOrganizationPlanRequestBudget` |
| `server/internal/service/workspace_profile_migration_test.go` | `TestWorkspaceProfileColumnUpgradePreservesLegacyIdentity` |
| `server/internal/service/workspace_property_models_test.go` | `TestWorkspacePropertyMigrationKeepsVersionIdentity` |
| `server/internal/service/workspace_publish_binding_test.go` | `TestWorkspaceLegacyPublicationRejectsStaleUnboundSnapshot` |
| `server/internal/service/workspace_publish_guard_test.go` | `TestWorkspacePublishRechecksAuthorizationInsideTransaction` |
| `server/internal/service/workspace_read_failure_test.go` | `TestWorkspaceReadVersionDoesNotHideQueryFailures` |
| `server/internal/service/workspace_request_model_test.go` | `TestWorkspaceRequestIdentityAndRollback` |
| `server/internal/service/workspace_resource_http_test.go` | `TestWorkspaceResourceHTTP` |
| `server/internal/service/workspace_resource_http_test.go` | `TestWorkspaceResourceListCumulativeBudget` |
| `server/internal/service/workspace_resource_lookup_test.go` | `TestWorkspaceResourceLookupOriginalReceiptAndNoWrites` |
| `server/internal/service/workspace_resource_models_test.go` | `TestWorkspaceResourceTargetsAndCheckRevisions` |
| `server/internal/service/workspace_resource_test.go` | `TestWorkspaceResourceFixedFileAndReplay` |
| `server/internal/service/workspace_resource_test.go` | `TestWorkspaceResourceSnapshotAndAtomicFailure` |
| `server/internal/service/workspace_restart_test.go` | `TestWorkspaceGrantReceiptSurvivesDatabaseReopen` |
| `server/internal/service/workspace_rotation_http_test.go` | `TestWorkspaceRotationHTTPOneTimeDelivery` |
| `server/internal/service/workspace_rotation_lookup_test.go` | `TestWorkspaceRotationLookupHTTPMetadataOnly` |
| `server/internal/service/workspace_rotation_request_test.go` | `TestWorkspaceRotationRequestDoesNotReplayCredential` |
| `server/internal/service/workspace_rotation_test.go` | `TestWorkspaceRotationAtomicityAndExpiry` |
| `server/internal/service/workspace_route_matrix_test.go` | `TestWorkspaceAuthenticationRouteMatrix` |
| `server/internal/service/workspace_submission_archive_test.go` | `TestWorkspaceArchivePreparationRequiresAcceptedOwnedVersion` |
| `server/internal/service/workspace_submission_complete_test.go` | `TestWorkspaceSubmissionCompleteVerifiesAndPublishesOnce` |
| `server/internal/service/workspace_submission_create_test.go` | `TestWorkspaceSubmissionAtomicAllocationAndReplay` |
| `server/internal/service/workspace_submission_decide_test.go` | `TestWorkspaceSubmissionHumanDecisionKeepsFixedBytes` |
| `server/internal/service/workspace_submission_display_test.go` | `TestWorkspaceSubmissionDisplayUsesOriginalOperationAndAuthenticatedAgent` |
| `server/internal/service/workspace_submission_http_test.go` | `TestWorkspaceSubmissionCreateHTTPAndLookup` |
| `server/internal/service/workspace_submission_http_test.go` | `TestWorkspaceSubmissionSurvivesServiceReopen` |
| `server/internal/service/workspace_submission_isolation_test.go` | `TestWorkspaceTwoAgentsCannotCrossSubmissionBoundary` |
| `server/internal/service/workspace_submission_models_test.go` | `TestWorkspaceSubmissionIdentityConstraints` |
| `server/internal/service/workspace_submission_publish_test.go` | `TestWorkspaceSubmissionPublicationJoinsTransaction` |
| `server/internal/service/workspace_suggestion_create_test.go` | `TestWorkspaceSuggestionDedicatedActionAndReplay` |
| `server/internal/service/workspace_suggestion_decide_test.go` | `TestWorkspaceSuggestionAcceptancePreservesManualLabels` |
| `server/internal/service/workspace_suggestion_flow_test.go` | `TestWorkspaceSuggestionAuthenticatedFlow` |
| `server/internal/service/workspace_suggestion_http_test.go` | `TestWorkspaceSuggestionDecisionHTTP` |
| `server/internal/service/workspace_suggestion_models_test.go` | `TestWorkspaceSuggestionsIsolateAgentAndVersion` |
| `server/internal/service/workspace_task_detail_test.go` | `TestWorkspaceTaskDetailFixedOutput` |
| `server/internal/service/workspace_task_detail_test.go` | `TestWorkspaceTaskDetailStagingAndScope` |
| `server/internal/service/workspace_task_progress_test.go` | `TestWorkspaceTaskPublicationProgress` |
| `server/internal/service/workspace_tasks_test.go` | `TestWorkspaceOperationTaskProjection` |
| `server/internal/service/workspace_tasks_test.go` | `TestWorkspaceOperationTaskSubmissionBinding` |
| `server/internal/service/workspace_tasks_test.go` | `TestWorkspaceOperationTaskScanBudget` |
| `server/internal/service/workspace_tool_matrix_test.go` | `TestWorkspaceRegisteredToolSetDeniesLegacyDispatch` |
| `server/internal/service/workspace_transfer_tasks_boundary_test.go` | `TestWorkspaceTransferPlanRequestBudget` |
| `server/internal/service/workspace_transfer_tasks_boundary_test.go` | `TestWorkspaceTransferSameSourcePlansKeepDistinctCopies` |
| `server/internal/service/workspace_transfer_tasks_test.go` | `TestWorkspaceTransferPlanProjection` |
| `server/internal/service/workspace_transfer_tasks_test.go` | `TestWorkspaceTransferPlanGate` |
| `server/internal/service/workspace_visibility_test.go` | `TestWorkspaceVisibilityDoesNotHideInfrastructureFailure` |
| `server/sdk/workspace_contract_test.go` | `TestWorkspaceGeneratedFixturesThroughSDK` |
| `server/sdk/workspace_test.go` | `TestWorkspaceSDKPresenceEncoding` |
| `server/sdk/workspace_test.go` | `TestWorkspaceSDKGrantLifecycleAndOrganization` |
| `server/sdk/workspace_test.go` | `TestWorkspaceSDKDocumentPropertiesAndResourceReceipts` |
| `server/sdk/workspace_test.go` | `TestWorkspaceSDKSubmissionAndSuggestionIdentity` |

## 下一执行顺序

1. 核对上表新后端已有实现及未消费端点；按原任务补缺失测试，不直接重写。
2. D01：扩展同一合同生成器的 desktop profile、固定IPC与来源/阅读引用；writer另立消费任务。
3. D03：证明全入口受管保护与旧writer降级拒绝；无法证明覆盖就拒绝绑定。
4. D02/D04：本机门面、监督进程、共享阅读状态；D05/D06/D07：同一Web与Electron。
5. 真实fixture纵切通过后，按文件、Agent/收件、知识资源、管理页四组完成全站，再做视觉和三平台门。
