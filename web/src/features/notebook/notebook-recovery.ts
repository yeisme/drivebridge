export type NotebookRecoveryReason =
  | "storage_unavailable"
  | "lock_unavailable"
  | "insecure_context"
  | "invalid_recovery"
  | "recovery_changed";

export class NotebookRecoveryError extends Error {
  readonly reason: NotebookRecoveryReason;

  constructor(reason: NotebookRecoveryReason) {
    super(reason);
    this.name = "NotebookRecoveryError";
    this.reason = reason;
  }
}

export interface NotebookPageRecovery {
  key: string;
  date: string;
  path: string;
}

export interface NotebookDirectoryRecovery {
  key: string;
  path: string;
}

type Parser<T> = (raw: string) => T;

const keyHex = /^[a-f0-9]{32}$/;
const dateValue = /^\d{4}-\d{2}-\d{2}$/;

export function notebookPageSlot(actor: string, tenant: string, space: string, path: string) {
  return "drivebridge:notebook-create:" + JSON.stringify([actor, tenant, space, path]);
}

export function notebookDirectorySlot(actor: string, tenant: string, space: string, path: string) {
  return "drivebridge:notebook-directory-create:" + JSON.stringify([actor, tenant, space, path]);
}

export function parseNotebookPageRecovery(raw: string): NotebookPageRecovery {
  if (raw.length > 1024) throw new NotebookRecoveryError("invalid_recovery");
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new NotebookRecoveryError("invalid_recovery");
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new NotebookRecoveryError("invalid_recovery");
  const row = value as Record<string, unknown>;
  const keys = Object.keys(row).sort().join(",");
  if (keys !== "date,key" && keys !== "date,key,path") throw new NotebookRecoveryError("invalid_recovery");
  if (typeof row.key !== "string" || !row.key.startsWith("notebook-") || !keyHex.test(row.key.slice("notebook-".length))) throw new NotebookRecoveryError("invalid_recovery");
  if (typeof row.date !== "string" || (row.date !== "" && !dateValue.test(row.date))) throw new NotebookRecoveryError("invalid_recovery");
  if (row.path !== undefined && (typeof row.path !== "string" || row.path.length > 2048)) throw new NotebookRecoveryError("invalid_recovery");
  return { key: row.key, date: row.date, path: typeof row.path === "string" ? row.path : "" };
}

export function parseNotebookDirectoryRecovery(raw: string): NotebookDirectoryRecovery {
  if (raw.length > 1024) throw new NotebookRecoveryError("invalid_recovery");
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new NotebookRecoveryError("invalid_recovery");
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new NotebookRecoveryError("invalid_recovery");
  const row = value as Record<string, unknown>;
  if (Object.keys(row).sort().join(",") !== "key,path" || typeof row.key !== "string" || !row.key.startsWith("directory-") || !keyHex.test(row.key.slice("directory-".length)) || typeof row.path !== "string" || row.path.length > 2048) throw new NotebookRecoveryError("invalid_recovery");
  return { key: row.key, path: row.path };
}

function storageGet(slot: string) {
  try {
    return localStorage.getItem(slot);
  } catch {
    throw new NotebookRecoveryError("storage_unavailable");
  }
}

function storageSet(slot: string, value: string) {
  try {
    localStorage.setItem(slot, value);
  } catch {
    throw new NotebookRecoveryError("storage_unavailable");
  }
}

function storageRemove(slot: string) {
  try {
    localStorage.removeItem(slot);
  } catch {
    throw new NotebookRecoveryError("storage_unavailable");
  }
}

function parseStored<T>(slot: string, parse: Parser<T>) {
  const raw = storageGet(slot);
  return raw === null ? null : parse(raw);
}

function secureOriginReason() {
  const hostname = typeof location === "undefined" ? "" : location.hostname;
  const local = hostname === "localhost" || hostname === "127.0.0.1" || hostname === "[::1]";
  if (typeof window !== "undefined" && !window.isSecureContext && !local) return new NotebookRecoveryError("insecure_context");
  return null;
}

function lockRequest<T>(slot: string, signal: AbortSignal | undefined, callback: () => T | Promise<T>): Promise<T> {
  const originReason = secureOriginReason();
  if (originReason) return Promise.reject(originReason);
  if (typeof navigator === "undefined" || !navigator.locks || typeof navigator.locks.request !== "function") return Promise.reject(new NotebookRecoveryError("lock_unavailable"));
  let entered = false;
  try {
    const options = signal ? { signal } : undefined;
    const request = options === undefined
      ? navigator.locks.request(slot, () => { entered = true; return callback(); })
      : navigator.locks.request(slot, options, () => { entered = true; return callback(); });
    return Promise.resolve(request).catch(error => {
      if (entered || error instanceof NotebookRecoveryError || (error && typeof error === "object" && "name" in error && error.name === "AbortError")) throw error;
      throw new NotebookRecoveryError("lock_unavailable");
    });
  } catch (error) {
    if (entered || error instanceof NotebookRecoveryError || (error && typeof error === "object" && "name" in error && error.name === "AbortError")) return Promise.reject(error);
    return Promise.reject(new NotebookRecoveryError("lock_unavailable"));
  }
}

function storageProbe(slot: string) {
  const probe = `${slot}:probe:${Array.from(crypto.getRandomValues(new Uint8Array(8)), value => value.toString(16).padStart(2, "0")).join("")}`;
  storageSet(probe, "1");
  storageRemove(probe);
}

export function notebookRecoveryReason(error: unknown): NotebookRecoveryReason | "unknown" {
  if (error instanceof NotebookRecoveryError) return error.reason;
  return "unknown";
}

export function notebookRecoveryMessage(reason: NotebookRecoveryReason | "unknown") {
  switch (reason) {
    case "storage_unavailable":
      return "浏览器阻止了站点存储，无法安全发起新的创建请求。请允许此站点使用本地存储、退出无痕模式后点击“重新检查”。";
    case "lock_unavailable":
      return "当前浏览器或访问地址不支持 Web Locks，不会发起新的创建请求。请使用 http://127.0.0.1、http://localhost 或 HTTPS。";
    case "insecure_context":
      return "当前通过普通 HTTP 的非本机地址访问，无法启用跨标签安全锁，不会发起新的创建请求。请使用 http://127.0.0.1、http://localhost 或 HTTPS。";
    case "invalid_recovery":
      return "发现无法解析的旧恢复记录。为避免丢失未知结果，未自动删除；确认后可清除该记录并重新开始。";
    case "recovery_changed":
      return "原请求身份已被其他标签页改变，未发送新请求；请重新打开当前创建入口。";
    default:
      return "无法确认浏览器恢复能力，未发送创建请求；请点击“重新检查”。";
  }
}

export async function inspectNotebookRecovery<T>(slot: string, parse: Parser<T>) {
  const originReason = secureOriginReason();
  if (originReason) throw originReason;
  if (typeof navigator === "undefined" || !navigator.locks || typeof navigator.locks.request !== "function") throw new NotebookRecoveryError("lock_unavailable");
  const value = parseStored(slot, parse);
  storageProbe(slot);
  return value;
}

export async function claimNotebookRecovery<T>(slot: string, parse: Parser<T>, record: T, encode: (value: T) => string, signal: AbortSignal) {
  return lockRequest(slot, signal, () => {
    const existing = parseStored(slot, parse);
    if (existing !== null) return { record: existing, claimed: false };
    storageSet(slot, encode(record));
    return { record, claimed: true };
  });
}

export async function clearNotebookRecovery<T>(slot: string, parse: Parser<T>, key: string, getKey: (value: T) => string) {
  return lockRequest(slot, undefined, () => {
    const current = parseStored(slot, parse);
    if (current === null) return;
    if (getKey(current) !== key) throw new NotebookRecoveryError("recovery_changed");
    storageRemove(slot);
  });
}

export async function clearInvalidNotebookRecovery<T>(slot: string, parse: Parser<T>) {
  return lockRequest(slot, undefined, () => {
    const raw = storageGet(slot);
    if (raw === null) return;
    try {
      parse(raw);
      throw new NotebookRecoveryError("recovery_changed");
    } catch (error) {
      if (error instanceof NotebookRecoveryError && error.reason === "invalid_recovery") {
        storageRemove(slot);
        return;
      }
      throw error;
    }
  });
}

export function encodeNotebookRecovery(value: NotebookPageRecovery | NotebookDirectoryRecovery) {
  return JSON.stringify(value);
}

export function newNotebookKey(prefix: "notebook" | "directory") {
  return `${prefix}-${Array.from(crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, "0")).join("")}`;
}
