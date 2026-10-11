// Shared presentation helpers for the information workspace. Chinese labels
// only; protocol values pass through unchanged when unrecognized.

export const utcStamp = (value: string) =>
  value ? new Date(value).toISOString().replace("T", " ").slice(0, 19) + " UTC" : "未采集";

export const coverageLabel = (coverage: string) =>
  coverage === "complete"
    ? "完整观测"
    : coverage === "partial"
      ? "部分覆盖"
      : coverage || "未知覆盖";

export const directionLabel = (key: string) =>
  ({
    client_to_server: "客户端 → 服务端",
    server_to_client: "服务端 → 客户端",
    server_to_storage: "服务端 → 存储",
    storage_to_server: "存储 → 服务端",
    local_to_storage: "本机 → 存储",
    storage_to_local: "存储 → 本机",
  })[key] || key;

export const bindingStatusLabel = (status: string) =>
  ({
    not_processed: "未处理",
    processing: "处理中",
    searchable: "可检索",
    failed: "失败",
    stale: "已过期",
    unsupported: "不支持",
  })[status] || status;

export const threadStateLabel = (state: string) =>
  ({ open: "进行中", resolved: "已解决" })[state] || state;
