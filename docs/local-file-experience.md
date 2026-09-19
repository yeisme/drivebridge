# 本机文件操作、预览与远程交接

`drivebridge view` 仍只监听回环地址，启动链接只用于换取本地 HttpOnly 会话。文件修改由显式启用的 owner 经私有 0600 socket 执行；网页不打开第二份可写数据库，也不能传入任意远程目标。

```bash
drivebridge mcp --enabled --insights
drivebridge view
```

实时列举保留旧 `items` 文件数组，新增 `directories` 表示真实目录。旧 CLI List 行为保持文件列表；本机新页面根据实际目录显示树，不把已观察索引冒充全盘。缺少 owner 时显示索引并禁用写操作。

私有 `/files/action` 通道支持 mkdir、文件 move/copy、目录 copy/move/trash、restore、限定本机路径的上传和传输恢复／取消。复制优先走存储后端的服务端 Copy，不行则读取再上传；同名自动改名，不覆盖。目录作业最多 1000 个文件，不能把文件夹移进自己。浏览器文件以原始字节 PUT 进入私有暂存；交给 owner 后后台继续传输。未完成浏览器发送时关页仍需重新选择文件。

图片、视频、音频、PDF 通过固定引用读取，源版本改变时明确失败。缩略图与兼容预览复用 DriveBridge Server 的 `sdk/mediapreview`，不会自行维护另一套格式转换规则。转换组件缺失时保留原文件入口。Office、RAW 和 PDF 栅格化需要相应转换器与 Bubblewrap。

## 远程副本分享

用户在网页明确选择远程服务、用户访问密钥与目标空间。连接凭据及分享策略保存在本机私有加密存储，浏览器不持久保存密钥。创建分享需要用户身份，Agent 文件 token 不具备分享管理权限。

固定交付在复制前固定素材引用；持续共享由在线 owner 周期同步。复制完成后调用远程服务的同一分享合同，支持链接／指定用户、提取码、有效期和下载设置。源文件不直接暴露到公网。重复分享创建使用稳定幂等键；源离线时保留已发布副本并显示同步失败，用户可恢复原同步。

活动分享期间不允许把目标静默改成另一个远程账户。撤销先停止同步，再撤销远程授权；远程撤销失败保留待重试状态，不能声称已彻底撤销。

## 创作交接

本机创作面板将明确选择的固定引用复制到远程素材库，再使用远程 DriveBridge 的 Eikona／Scaena 消费连接。原始媒体字节走 HTTP，引用与任务回执不包含服务商凭据。远程生成与结果回存继续由原 owner 执行，本机网页不成为生成服务。

## 验证

```bash
python3 scripts/check.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge
```

`TestLocalShareRemoteCopyAndRevocation` 使用真实本机目录 provider、私有 socket 和独立远程 fixture 进程，验证复制、固定版本、重放与撤销。其余测试覆盖 CLI、索引、权限、传输和原有协议。证据在本项目 `temp/integration-test-runs/`；没有使用真实云账户或付费模型。
