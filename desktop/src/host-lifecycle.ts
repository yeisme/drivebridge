// One launch lifetime. Suspend permanently revokes it; resume cannot reauthorize.
export class HostLifetime<T extends { alive: boolean; close(): Promise<void> }> {
  #revoked = false;
  #host?: T;
  #starting: Promise<T | undefined>;
  #closing?: Promise<void>;

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
  stop(): Promise<void> {
    // Synchronous invalidation also fences dialog and HTTP continuations.
    this.#revoked = true;
    return this.#closing ??= this.#starting.then(host => host?.close());
  }
}
