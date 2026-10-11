// Presentation only: raw actions and identifiers remain unchanged in API/export data.
const actions: Record<string, string> = {
  "me": "查看本人身份", "tenant": "查看租户信息", "capabilities": "查看可用能力",
  "spaces.list": "查看网盘列表", "spaces.create": "创建网盘", "spaces.bind": "绑定存储",
  "spaces.trash_policy": "设置回收站保留期", "space.quota_updated": "调整网盘配额",
  "keys.list": "查看登录 Key", "keys.create": "创建登录 Key", "keys.reveal": "查看本人 Key",
  "keys.rotate": "轮转登录 Key", "keys.label": "修改 Key 名称", "keys.revoke": "吊销登录 Key",
  "keys.disable": "禁用登录 Key", "keys.enable": "启用登录 Key", "keys.delete": "删除登录 Key",
  "users.list": "查看用户目录", "users.create": "创建用户", "users.archive": "归档用户",
  "users.revoke": "撤销用户凭据", "users.rotate": "轮转用户凭据",
  "teams.list": "查看团队列表", "teams.detail": "查看团队详情", "teams.invitations": "查看团队邀请",
  "governance.users.list": "查看租户成员", "governance.users.detail": "查看成员详情",
  "governance.users.impact": "检查账号操作影响", "governance.users.action": "执行账号管理操作",
  "me.update": "修改个人资料", "me.avatar.options": "查看可选头像", "me.avatar.read": "读取头像",
  "uploads.create": "创建上传", "uploads.part": "上传文件分片", "uploads.complete": "完成文件上传",
  "operations.get": "查看传输详情", "previews.list": "查看预览列表", "previews.prepare": "准备文件预览",
  "previews.read": "读取文件预览", "media.read": "读取媒体", "blobs.stat": "查看内容信息",
  "shares.link": "获取分享链接", "shares.recipients": "设置分享接收方",
  "onedrive.browse": "浏览 OneDrive", "onedrive.import": "从 OneDrive 导入", "onedrive.export": "导出到 OneDrive",
  "scan.start": "启动目录扫描", "usage.scan": "扫描用量", "tenant.invited": "邀请租户管理员",
  "tenant.status": "修改租户状态", "member.role": "修改租户角色", "request.expired": "授权申请到期",
  "files.trash_restore": "恢复回收站文件", "account.recovery_code": "更新恢复码",
  "account.recovery_requested": "申请账号恢复", "account.recovery_verified": "核验账号恢复",
  "account.privileges_restored": "恢复账号特权",
  "governance.overview": "查看管理概览", "governance.change": "修改管理设置",
  "governance.bootstrap": "初始化租户治理", "operations.inbox": "查看待归档产物",
  "operations.list": "查看传输任务", "operations.create": "创建传输任务",
  "operations.complete": "完成传输", "operations.cancel": "取消传输",
  "operations.resume": "恢复传输", "operations.stat": "查看传输详情",
  "browser.list": "浏览目录", "data.view": "管理员查看", "data.download": "下载或导出文件",
  "files.list": "查看文件列表", "files.stat": "查看文件信息", "files.versions": "查看历史版本",
  "files.download": "下载文件", "files.access": "读取文件", "files.access_stream": "流式读取文件",
  "files.trash": "移入回收站", "files.restore": "恢复文件", "files.purge": "永久删除文件",
  "files.batch_restore": "批量恢复文件", "files.batch_purge": "批量永久删除",
  "files.copy": "复制文件", "files.move": "移动文件", "files.archive": "归档文件",
  "files.export": "导出文件", "files.resolve": "解析文件引用", "files.usage": "查看文件用量",
  "files.version_replace": "更新文件版本", "files.download_incomplete": "文件下载中断",
  "directories.create": "新建文件夹", "directories.copy": "复制文件夹",
  "directories.move": "移动文件夹", "directories.trash": "文件夹移入回收站",
  "directories.export": "导出文件夹", "archive.zip": "下载压缩包",
  "agents.list": "查看机器人授权", "agents.create": "创建机器人授权",
  "agents.revoke": "撤销机器人授权", "agents.archive": "归档机器人授权",
  "connections.list": "查看存储连接", "connections.check": "检查存储连接",
  "connections.disconnect": "断开存储连接", "connections.platform": "分配平台存储",
  "connections.onedrive": "连接 OneDrive", "shares.create": "创建分享",
  "shares.list": "查看分享列表", "shares.revoke": "撤销分享", "shares.access": "访问分享",
  "audit.events": "查看审计事件", "audit.list": "查看访问记录",
  "audit.series": "查看安全趋势", "audit.export": "导出审计记录",
  "usage.summary": "查看用量概览", "usage.series": "查看用量趋势",
  "usage.groups": "查看用量归因", "usage.files": "查看容量明细", "usage.export": "导出用量统计",
  "request.created": "提交授权申请", "request.approved": "批准申请",
  "request.rejected": "拒绝申请", "request.revoked": "撤销授权",
  "request.executing": "开始执行清单", "request.result": "记录执行结果", "request.item": "记录分项结果",
  "member.invited": "邀请租户成员", "member.accepted": "接受租户邀请", "member.removed": "移除租户成员",
  "team.created": "创建团队", "team.member": "修改团队成员", "space.created": "创建网盘",
  "space.permission": "调整网盘权限", "account.activated": "激活账号", "account.provisioned": "开通账号",
  "account.recovered": "恢复账号", "account.platform_role": "调整管理员角色",
  "account.responsibility_taken": "接管账号职责", "emergency.started": "启动应急访问",
};

export const auditActionLabel = (action: string) => actions[action] || "其他操作";
export const shortIdentifier = (value: string) => value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value;

export function auditActorName(kind: string, id: string | undefined, names: {users?: Record<string, string>; agents?: Record<string, string>}) {
  if (!id) return kind === "anonymous" ? "匿名访问" : "未记录身份";
  if (kind === "user") return names.users?.[id] || "用户";
  if (kind === "agent") return names.agents?.[id] || "机器人";
  return ({anonymous:"匿名访问",instance_operator:"实例操作员",local_user:"本机用户",local:"本机身份",service_identity:"服务身份"} as Record<string,string>)[kind] || "其他身份";
}
