## ADDED Requirements

### Requirement: mount vault 是 WebDAV 之外的加法路径挂载

`drivebridge mount vault` SHALL 作为 `mount webdav` 的加法子命令存在。默认 `mount webdav` 只读回环、`mount fuse` 在默认构建返回 `unsupported` 的合同 SHALL 保持原义。

#### Scenario: webdav 默认仍然只读
- **WHEN** 用户运行 `drivebridge mount webdav --space <id>` 且未传 `--writable`
- **THEN** 模式仍为只读回环，行为与本 change 之前一致

#### Scenario: fuse 默认构建仍 unsupported
- **WHEN** 默认 CGO=0 二进制执行 `drivebridge mount fuse`
- **THEN** 失败为 `unsupported`
