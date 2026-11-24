// One launch lifetime. Suspend permanently revokes it; resume cannot reauthorize.
export class HostLifetime<T extends { alive: boolean; close(): Promise<void> }> {
  #revoked = false;
  #host?: T;
  #starting: Promise<T | undefined>;
  #closing?: Promise<void>;
  #generation = 0;

  constructor(start: () => Promise<T>) {
    this.#starting = Promise.resolve().then(start).then(host => {
      this.#host = host;
      return host;
    }).catch(() => undefined);
  }
  get current(): T | undefined {
    return !this.#revoked && this.#host?.alive ? this.#host : undefined;
  }
  async ready(): Promise<void> { await this.#starting; }
  invalidateRequests(): void { this.#generation++; }
  async request<R>(host: T, validWindow: () => void, call: () => Promise<R>): Promise<R> {
    const generation = this.#generation;
    const check = () => {
      if (this.current !== host || generation !== this.#generation) throw new Error('desktop_session_invalid');
      validWindow();
    };
    check();
    try {
      const result = await call();
      check();
      return result;
    } catch (error) {
      // Revocation wins over both a late result and a late owner error.
      check();
      throw error;
    }
  }
  stop(): Promise<void> {
    // Synchronous invalidation also fences dialog and HTTP continuations.
    this.#revoked = true;
    this.invalidateRequests();
    return this.#closing ??= this.#starting.then(host => host?.close());
  }
}
