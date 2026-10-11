// UI locale defaults to Simplified Chinese; protocol codes remain stable.
export const errors: Record<string, string> = {
  preview_timeout:"预览请求超时，请重试；文件和版本记录不会改变。",
  not_found: "资源不存在，或当前身份无权访问。",
  request_failed: "操作未完成，请稍后重试。",
  network_error: "网络连接失败，请检查网络后重试。",
 team_archived:"团队已归档，只允许保留的读取操作。",
 last_team_admin:"不能移除最后一位团队管理员，请先任命继任管理员。",
 last_space_custodian:"不能移除最后一位网盘数据负责人，请先完成交接。",
 storage_in_use:"连接仍被团队网盘使用，不能取消分配。",
 storage_not_assigned:"该存储未分配给当前团队。",
 revision_required:"缺少团队版本，请刷新后重新操作。",
 invitation_exists:"该用户已有有效的团队邀请。",
 invitation_unavailable:"邀请已被处理、取消或过期。",
 invitation_not_pending:"只能拒绝或续期仍待处理的邀请。",
  invalid_team:"请检查团队名称和说明长度。",
  invalid_display_name: "显示名称需要 1–80 个字符。",
  invalid_avatar_ref: "头像必须使用受控资源引用。",
  invalid_profile_update: "请至少修改一项个人资料。",

 preview_busy:"预览队列繁忙，请稍后重试。",
 preview_encrypted_scratch_limit:"加密素材的转换预览超过内存暂存上限，请直接播放或下载原文件。",
 preview_encrypted_scratch_unavailable:"此环境没有可用的加密素材内存暂存区，请直接读取原文件。",
 active_share_destination_immutable:"还有活动分享，请先撤销或迁移后再更换远程目标。",

 share_destination_unconfigured:"请先连接远程分享空间。",
 share_user_credential_required:"远程分享需要有效的用户密钥，不能使用 机器人授权。",
 share_listing_limit:"目录超过当前分享扫描范围，请选择更小的目录。",
 share_empty:"当前没有可分享的素材。",
 share_credentials_unavailable:"无法读取本机分享连接，请重新配置或恢复密钥。",

 approval_required: "当前为只读访问，请先在授权中心申请修改。",
 reauthentication_required: "请重新输入当前登录密钥，或重新通过 OIDC 登录后再操作。",
 account_recovery_required: "请使用本人领取的账号恢复流程。",
 recovery_not_ready: "恢复尚未获批、已过期或领取证明无效。",
 unified_login_required: "本租户已切换为统一登录，请通过 Aigora 账号重新登录；旧的用户 Key 与 机器人授权不再可用。",
 identity_mapping_required: "当前身份尚未映射到本租户数据，请联系管理员完成身份映射后再访问。",
 identity_unavailable: "统一身份服务暂不可用，新的写入已暂停；请稍后重试。",
 service_admission_required: "此操作暂未获得托管准入（计量或凭据未配置），请联系管理员或稍后重试。",
 insufficient_funds: "服务账户余额不足以完成本次操作，请先充值或联系管理员。",
 cross_tenant_admin_context_required: "此操作需要单独授权的跨租户管理上下文，请通过授权中心申请。",
 invalid_reason: "请填写 3 至 1000 字的操作原因，不要包含凭据。",
 invalid_duration: "授权时长需为 1 至 8 小时。",
 invalid_role: "请选择有效的空间角色。",
 membership_exists: "该用户已是本租户成员。",

 confirmation_mismatch: "用户名不匹配，请输入当前要操作的用户名称。",
 account_state_conflict: "账号状态已变化，或已归档；请刷新列表后重新操作。",
 last_platform_admin: "不能停用最后一位平台管理员，请先完成交接。",
 last_tenant_admin: "不能停用最后一位租户管理员，请先完成交接。",
 key_material_unavailable: "旧 Key 尚未保存可取回的密文，请用原 Key 登录一次，或轮转为新 Key。",
 key_inactive: "此 Key 已失效，请刷新列表后使用有效 Key。",
 credential_unavailable: "密钥暂时无法解密，请恢复实例密钥或轮转后重试。",
 creative_unavailable: "创作服务尚未连接，或连接已断开。",
 creative_owner_rejected: "领域服务拒绝了此次请求，请检查项目权限、参考格式、模型与额度。",
 creative_transport_unknown: "与创作服务的连接中断，请查询原交接记录，不要重复生成。",
 creative_confirmation_required: "提交前需要确认素材使用权与费用。",
 creative_reference_limit: "当前视频入口支持一个参考素材，请检查模型与时长。",
 creative_subject_confirmation_required: "请明确参考素材的人物类型。",
  creative_upload_transport_unsupported: "此连接的上传方式尚未适配，请在领域服务启用同源上传。",
 invalid_creative_connection: "请检查服务地址、项目引用和访问凭据。",

  version_required: "预览需要固定版本，请重新选择文件。",
  preview_unsupported: "此格式暂不支持预览，可下载原文件。",
  preview_load_failed: "预览读取失败，请重新打开或检查访问权限。",
  preview_converter_unavailable: "预览转换组件尚未安装，原文件可下载。",
  preview_sandbox_unavailable: "文档预览隔离组件不可用，原文件可下载。",
  preview_conversion_failed: "此文件转换失败，可重试或下载原文件。",
  preview_interrupted: "预览处理已中断，请重试。",
  preview_size_limit: "文件超出当前转换大小限制，原文件可下载。",
  last_admin_required: "这是最后一个可用管理员密钥，请先签发并验证另一把管理员密钥。",
  last_active_key: "请先新建另一把登录 Key，再禁用、吊销或删除最后一把有效 Key。",
  key_not_disabled: "此 Key 未处于禁用状态。",
  invalid_batch: "请选择 1 到 100 个文件。",
  invalid_trash_policy: "自动清理天数需为 1 到 365。",
  file_in_use: "文件正被分享、交接或传输使用，请完成或取消后再操作。",
  share_unavailable: "分享已失效、已撤销或不存在。",
  share_passcode_required: "请输入正确的提取码。",
  invalid_share_selection: "选择中包含待审核、已删除或不在当前范围的文件。",
  invalid_recipient: "接收方用户 ID 无效。",

  insights_unavailable: "统计暂不可用，请检查采集状态",
  audit_unavailable: "审计暂不可写，关键操作已暂停；仍可尝试撤销权限",
  scan_not_enabled: "请先明确启用所选空间的目录扫描",
  scan_busy: "已有扫描在进行，请稍后查看结果",
  inventory_unsupported: "此后端暂不提供目录观测",
  invalid_time: "时间范围无效，最多支持一年",
  origin_rejected: "本机视图只接受同源访问",
  unauthorized: "登录已失效，请重新登录",
  forbidden: "当前授权不允许此操作",
  csrf_rejected: "会话校验失败，请重新登录；若仍失败，请检查访问地址与服务配置是否一致。",
  login_origin_mismatch: "当前访问地址与服务配置不一致，无法登录。请使用配置的地址，或调整本地预览的访问地址配置。",
  invalid_quota: "空间配额必须为有效正整数",
  quota_below_occupied: "配额不能低于已用与上传预留之和",
  quota_exceeded: "空间容量不足，请释放空间后重试",
  storage_disconnected: "存储未连接，请先设置存储",
  storage_write_failed: "写入存储失败，可以恢复原传输",
  storage_read_failed: "暂时无法读取存储",
  name_conflict: "已有同名文件，请更换名称或上传新版本",
  version_conflict: "文件版本已改变，请重新选择",
  content_mismatch: "文件校验不一致，请重新选择原文件",
  source_version_changed: "OneDrive 文件已改变，请重新选择",
  onedrive_not_configured: "管理员尚未配置 OneDrive 连接",
  oidc_not_configured: "管理员尚未配置登录服务",
  disabled: "服务尚未启用",
  invalid_storage_config: "请检查存储地址、桶名与凭据",
  storage_unavailable: "无法访问存储，请检查连接",
  delivery_unavailable: "下载暂时不可用，请稍后恢复",
  storage_delete_failed: "存储端删除失败，请稍后重试",
  object_missing: "存储对象缺失，请检查文件是否已在云端被改动",
  throttled: "请求被云端限流，请稍后再试",
  export_conflict: "导出与现有或进行中的导出冲突，请先完成或取消原操作",
  onedrive_authorization_required: "OneDrive 连接需要重新授权",
  user_key_duplicate: "批量中存在重复密钥，整批未写入，请调整后重试",
  invalid_provenance: "来源标识格式无效，请使用简短标识而非链接或正文",
  file_trashed: "文件在回收站中，请先恢复",
  encrypted_space_reference_rejected:
    "此加密空间需通过授权下载使用，暂不支持直接参考交接",
  incomplete_part: "上传被中断，请恢复原传输",
  invalid_archive: "归档参数不完整，请重新选择文件和目标路径",
  invalid_copy: "复制参数不完整，请重新选择目标目录后重试",
  invalid_directory: "不能把文件夹移动到自身或子目录，请换一个目标。",
  directory_too_large: "文件夹超过 1000 个文件，请拆分后再复制。",
  unsupported: "此存储类型暂不支持该受管操作",
  invalid_query: "查询条件无效，请调整筛选后重试。",
  cursor_expired: "翻页游标已过期，请返回第一页重新查看。",
  budget_exceeded: "请求超出预算，请收窄时间或范围后重试。",
  dependency_unavailable: "依赖服务暂不可用，稍后自动重试；已展示的数据不会当作实时。",
  authorization_projection_stale: "授权投影尚未追平，暂不能按旧权限聚合统计。",
  analytics_backend_unavailable: "分析后端不可用，文件上传等基础操作不受影响。",
  query_budget_exceeded: "查询超出预算，请收窄条件或申请显式导出。",
  query_changed: "查询条件已变化，请从第一页重新加载。",
};

const messages: Record<string,string> = {
 "history.search":"搜索文件、来源或任务", "history.status":"筛选状态", "history.source":"筛选来源", "history.allSources":"全部来源", "history.records":"条记录", "history.clear":"清理历史", "history.restore":"显示已清理记录", "history.scope":"仅清理当前浏览器的列表历史，不删除文件；进行中和可恢复的任务会保留。", "history.confirm":"清理当前筛选中的已完成及已取消记录？", "history.confirmButton":"确认清理", "history.cleared":"历史记录已从列表收起。", "history.empty":"没有符合条件的记录", "history.adjust":"调整筛选条件，或显示已清理记录。", "history.storageFailed":"浏览器无法保存清理设置，请检查存储权限。", "cancel":"取消", "undo":"撤销", "overview":"总览", "analytics":"使用分析", "analytics.description":"按日期、空间与来源查看用量和审计。"
};
export const locale = "zh-CN";
export function t(key: string) { return messages[key] || "暂无翻译"; }
export function statusLabel(status: string) { return ({all:"全部状态",active:"进行中 / 待处理",completed:"已完成",cancelled:"已取消",verifying:"正在校验",failed:"等待重试",interrupted:"等待恢复",awaiting_upload:"等待文件",transferring:"正在传输",queued:"排队中",running:"执行中",pending:"等待处理"} as Record<string,string>)[status] || "状态待更新"; }
export function kindLabel(kind: string) {
  return (
    (
      {
        copy: "复制文件",
        directory_copy: "复制文件夹",
        directory_export: "复制到 OneDrive",
        export: "复制到 OneDrive",
        upload: "上传",
        archive: "归档副本",
        import: "从 OneDrive 导入",
      } as Record<string, string>
    )[kind] || ""
  );
}
export function localizedError(error: unknown) {
 const code = error instanceof Error ? error.message : "request_failed";
 if (errors[code]) return errors[code];
 if (["Failed to fetch","NetworkError when attempting to fetch resource.","Load failed"].includes(code)) return errors.network_error;
 if (code === "The referenced resource does not exist or is not visible to this identity.") return errors.not_found;
 // Never display untranslated upstream messages or provider payloads.
 return errors.request_failed;
}
