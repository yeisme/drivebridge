import { sha256 } from "@noble/hashes/sha2.js";

/**
 * Compute a lowercase hex SHA-256 digest.
 * Uses native WebCrypto when available and seamlessly falls back to pure-JS
 * @noble/hashes on plain-HTTP LAN origins where crypto.subtle is undefined.
 */
export async function sha256Hex(input: string | Uint8Array): Promise<string> {
  const bytes = typeof input === "string" ? new TextEncoder().encode(input) : input;
  if (typeof globalThis.crypto !== "undefined" && globalThis.crypto?.subtle?.digest) {
    try {
      const buf = await globalThis.crypto.subtle.digest("SHA-256", bytes as BufferSource);
      return Array.from(new Uint8Array(buf), (b) => b.toString(16).padStart(2, "0")).join("");
    } catch {
      // Fall through to deterministic pure-JS SHA-256.
    }
  }
  const digest = sha256(bytes);
  return Array.from(digest, (b) => b.toString(16).padStart(2, "0")).join("");
}

/**
 * Cross-tab lease lock backed by localStorage for plain-HTTP LAN origins where
 * Web Locks API (`navigator.locks`) is not exposed by the browser.
 */
const LEASE_TTL_MS = 15_000;
const LEASE_RENEW_MS = 3_000;
const LEASE_POLL_MS = 25;

interface LeaseRecord {
  owner: string;
  token: string;
  expiresAt: number;
}

const inProcessQueues = new Map<string, Promise<unknown>>();

function randomHex(byteLength = 12): string {
  const bytes = new Uint8Array(byteLength);
  if (typeof globalThis.crypto?.getRandomValues === "function") {
    globalThis.crypto.getRandomValues(bytes);
  } else {
    for (let i = 0; i < byteLength; i++) bytes[i] = Math.floor(Math.random() * 256);
  }
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

const tabOwnerId = `tab-${randomHex(8)}`;

function readLease(key: string): LeaseRecord | null {
  const raw = localStorage.getItem(key);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as LeaseRecord;
    if (
      parsed &&
      typeof parsed.owner === "string" &&
      typeof parsed.token === "string" &&
      typeof parsed.expiresAt === "number"
    ) {
      return parsed;
    }
  } catch {
    // Ignore corrupted lease record and allow overwrite.
  }
  return null;
}

async function acquireStorageLease<T>(
  name: string,
  signal: AbortSignal | undefined,
  callback: () => T | Promise<T>,
): Promise<T> {
  const leaseKey = `drivebridge:tab-lock:${name}`;
  const previous = inProcessQueues.get(name) ?? Promise.resolve();
  let releaseQueue!: () => void;
  const currentGate = new Promise<void>((resolve) => {
    releaseQueue = resolve;
  });
  inProcessQueues.set(
    name,
    previous.then(() => currentGate, () => currentGate),
  );

  await previous.catch(() => {});
  try {
    signal?.throwIfAborted();
    const token = randomHex(12);
    while (true) {
      signal?.throwIfAborted();
      const now = Date.now();
      const existing = readLease(leaseKey);
      if (!existing || existing.expiresAt <= now) {
        const next: LeaseRecord = {
          owner: tabOwnerId,
          token,
          expiresAt: now + LEASE_TTL_MS,
        };
        localStorage.setItem(leaseKey, JSON.stringify(next));
        const verify = readLease(leaseKey);
        if (verify && verify.token === token && verify.owner === tabOwnerId) {
          break;
        }
      }
      await new Promise<void>((resolve, reject) => {
        const timer = setTimeout(() => {
          signal?.removeEventListener("abort", onAbort);
          resolve();
        }, LEASE_POLL_MS);
        const onAbort = () => {
          clearTimeout(timer);
          reject(signal?.reason ?? new DOMException("The operation was aborted.", "AbortError"));
        };
        signal?.addEventListener("abort", onAbort, { once: true });
      });
    }

    const renewTimer = setInterval(() => {
      try {
        const current = readLease(leaseKey);
        if (current && current.token === token) {
          localStorage.setItem(
            leaseKey,
            JSON.stringify({
              owner: tabOwnerId,
              token,
              expiresAt: Date.now() + LEASE_TTL_MS,
            }),
          );
        }
      } catch {
        // Ignore renewal errors; the callback itself checks storage integrity.
      }
    }, LEASE_RENEW_MS);

    try {
      return await callback();
    } finally {
      clearInterval(renewTimer);
      try {
        const current = readLease(leaseKey);
        if (current && current.token === token) {
          localStorage.removeItem(leaseKey);
        }
      } catch {
        // Ignore cleanup errors when storage was revoked mid-flight.
      }
    }
  } finally {
    releaseQueue();
    if (inProcessQueues.get(name) === currentGate) {
      inProcessQueues.delete(name);
    }
  }
}

export function createLanLockManager(): LockManager {
  return {
    request: ((
      name: string,
      optionsOrCallback: LockOptions | LockGrantedCallback<unknown>,
      maybeCallback?: LockGrantedCallback<unknown>,
    ) => {
      const options = typeof optionsOrCallback === "function" ? undefined : optionsOrCallback;
      const callback = typeof optionsOrCallback === "function" ? optionsOrCallback : maybeCallback!;
      return acquireStorageLease(name, options?.signal, () =>
        callback({ name, mode: options?.mode ?? "exclusive" }),
      );
    }) as LockManager["request"],
    query: async () => ({ held: [], pending: [] }),
  };
}

/**
 * Install the LAN HTTP compatibility polyfills on `navigator.locks` and
 * `crypto.subtle` when the browser is running in a non-secure HTTP context
 * (e.g. `http://10.10.1.101:18786` or `http://192.168.x.x`).
 * On secure contexts (`localhost` / `HTTPS`), native browser primitives remain
 * authoritative so explicit test overrides of `navigator.locks` stay intact.
 */
export function ensureLanBrowserCompat(): void {
  const hostname = typeof location === "undefined" ? "" : location.hostname;
  const local =
    hostname === "localhost" ||
    hostname === "127.0.0.1" ||
    hostname === "[::1]" ||
    hostname === "";
  const isLanPlainHttp = typeof window !== "undefined" && (window.isSecureContext === false || !local);

  if (isLanPlainHttp && typeof navigator !== "undefined" && !navigator.locks?.request) {
    try {
      Object.defineProperty(navigator, "locks", {
        configurable: true,
        writable: true,
        value: createLanLockManager(),
      });
    } catch {
      // Ignore if navigator is frozen.
    }
  }
  if (typeof globalThis.crypto !== "undefined" && !globalThis.crypto.subtle) {
    try {
      Object.defineProperty(globalThis.crypto, "subtle", {
        configurable: true,
        value: {
          digest: async (algorithm: AlgorithmIdentifier, data: BufferSource) => {
            const name = typeof algorithm === "string" ? algorithm : algorithm.name;
            if (name.toUpperCase().replace("-", "") !== "SHA256") {
              throw new DOMException(`Unsupported algorithm: ${name}`, "NotSupportedError");
            }
            const view =
              data instanceof ArrayBuffer
                ? new Uint8Array(data)
                : new Uint8Array(data.buffer, data.byteOffset, data.byteLength);
            const out = sha256(view);
            return out.buffer.slice(out.byteOffset, out.byteOffset + out.byteLength);
          },
        },
      });
    } catch {
      // Ignore if crypto is frozen.
    }
  }
}
