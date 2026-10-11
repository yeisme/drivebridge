import type { UserIdentity } from "../api";
import { api, setCSRF } from "./transport";

export interface SessionView {
  user: UserIdentity;
  csrf_token: string;
  admin: boolean;
  instance_id?: string;
  governance_enabled?: boolean;
}

// Read-only identity check: never overwrite the current shell identity or CSRF.
export async function verifySessionIdentity(userId: string, instance: string, signal: AbortSignal): Promise<void> {
  const session = await api<SessionView>("/me", "GET", undefined, {}, signal);
  signal.throwIfAborted();
  if (session.user.id !== userId || (session.instance_id || "") !== instance) {
    throw Object.assign(new Error("unauthorized"), { status: 401 });
  }
}

// api rejects stale-context responses before they can change the CSRF token.
// Session secrets remain in memory and are never persisted by the shell.
export async function loadSession(signal?: AbortSignal): Promise<SessionView> {
  const session = await api<SessionView>("/me", "GET", undefined, {}, signal);
  // Also reject responses from transports that resolved despite cancellation.
  signal?.throwIfAborted();
  setCSRF(session.csrf_token);
  return session;
}
