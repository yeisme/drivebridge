# Support matrix

| Capability | Status | Boundary |
| --- | --- | --- |
| OneDrive local CLI/MCP path | stable target | Login, space, browse, upload, resume, download, trash, restore, stable reference |
| Web workspace against local fixture | stable target | Fixture and controlled local service only |
| Generic rclone backends | experimental | Transfer surface; no production guarantee |
| Scaena video handoff | experimental | Owner contract and local relay only |
| Remote multi-tenant mode | planned/experimental | No production or billing claim |
| Electron desktop | planned | Cross-platform installation and signing remain release gates |
| Real OIDC, Microsoft and S3 deployment | planned | Separate self-hosted GA acceptance required |

“Stable target” means the path is the first acceptance target; it does not
claim that real cloud or production deployment has already passed.
