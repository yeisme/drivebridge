// Storage identifiers follow backend_capabilities in the root CLI/MCP module.
// OneDrive = 传输 + 临时直链 + 参考交接；generic rclone 后端仅传输。
// 未知 kind 一律按「未识别后端」诚实呈现，不猜测能力，不静默当作已支持。
export interface StorageBackend {
  key: string;
  label: string;
  directUrl: boolean;
  handoff: boolean;
  unrecognized?: boolean;
}

const KNOWN: Record<string, StorageBackend> = {
  onedrive: {
    key: "onedrive",
    label: "OneDrive",
    directUrl: true,
    handoff: true,
  },
  s3: { key: "s3", label: "对象存储（S3 兼容）", directUrl: false, handoff: false },
  drive: {
    key: "drive",
    label: "Google Drive",
    directUrl: false,
    handoff: false,
  },
  dropbox: { key: "dropbox", label: "Dropbox", directUrl: false, handoff: false },
  box: { key: "box", label: "Box", directUrl: false, handoff: false },
  pcloud: { key: "pcloud", label: "pCloud", directUrl: false, handoff: false },
  webdav: { key: "webdav", label: "WebDAV", directUrl: false, handoff: false },
  smb: { key: "smb", label: "SMB", directUrl: false, handoff: false },
  sftp: { key: "sftp", label: "SFTP", directUrl: false, handoff: false },
  local: { key: "local", label: "本地目录", directUrl: false, handoff: false },
  alias: { key: "alias", label: "别名远端", directUrl: false, handoff: false },
};

export function backendOf(kind?: string): StorageBackend {
  const key = (kind || "").trim().toLowerCase();
  const known = KNOWN[key];
  if (known) return known;
  return {
    key: key || "unknown",
    label: key ? `未识别后端（${key}）` : "未识别后端",
    directUrl: false,
    handoff: false,
    unrecognized: true,
  };
}

export function capabilityText(backend: StorageBackend): string {
  if (backend.unrecognized) return "能力未知";
  return backend.directUrl && backend.handoff
    ? "传输 · 临时直链 · 参考交接"
    : "仅传输";
}
