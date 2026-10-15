# DriveBridge 当前产品交互原型

本原型复用同仓正式 React 客户端的页面、组件、路由和样式，构建为独立静态资源。`bridge.js` 将客户端请求接到浏览器中的示例状态；原型操作不访问正式 DriveBridge 服务。示例响应由 `capture.mjs` 在可丢弃的本地 fixture 中通过已有 HTTP 合同生成，再移除凭据和服务 URL。

已通过 OpenDesign MCP 发布到既有项目 `drivebridge-personal-agent-workspace`：[默认预览](http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/index.html#/home)、[桌面和手机截图浏览](http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/replicas/current-product-20261012/review/index.html)。原默认入口已保留为[历史版本](http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/history/current-product-before-20261012.html)。

覆盖当前 22 个主入口：工作台、资源态势、网盘、知识库、资源库、集合、标签与属性、搜索、文件流程、任务、分享、团队、回收站、存储、设备、整理规则、能力说明、收件、Agent、访问凭据、治理和设置。正式产品尚未提供的原生设备能力继续显示原有边界说明。

文件菜单、拖拽、上传、精确搜索、Markdown 阅读与版本保存、Agent 接入、侧栏和主题交互沿用正式组件；原型只模拟这些组件使用的响应。右下角“交互原型”可切换有内容、空空间、只读协作、服务错误和登录失效，并恢复初始状态。原型中的 Agent 凭据是明确的示例字符串。

从 DriveBridge 根目录构建当前客户端静态资源：

```bash
pnpm --dir web exec vite build --base ./ --outDir ../temp/opendesign-replica/bundle
node web/prototypes/current-product/capture.mjs
node web/prototypes/current-product/package.mjs
python3 -m http.server 32016 --bind 127.0.0.1 --directory temp/opendesign-replica/bundle
```

采集前需要在另一个终端运行可丢弃的 fixture：

```bash
go -C server run ./cmd/drivebridge-fixture --listen 127.0.0.1:32015 --web-dir ../web/dist --governance --property-editing --label-management --automation-management --share-receivers --hold-management --transfer-plans
```

验证使用项目已有 Playwright。`verify.mjs` 遍历桌面和手机页面、检查关键交互和真实 API 请求隔离；产物写入传入的 evidence artifacts 目录。`DRIVEBRIDGE_REPLICA_URL` 可设为 OpenDesign 原型入口，使用相同检查验证发布结果。
