// Prototype-only timing controller. Cancel invalidates pending callbacks as well as visible content.
export function createHoverController<T>(
  show: (candidate: T | null) => void,
  valid: (candidate: T) => boolean = () => true,
  same: (left: T, right: T) => boolean = Object.is,
) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let epoch = 0;
  let current: T | null = null;
  let visible = false;
  const clear = () => {
    clearTimeout(timer);
    timer = undefined;
    epoch++;
  };
  return {
    enter(candidate: T) {
      if (
        visible &&
        current !== null &&
        same(current, candidate) &&
        valid(candidate)
      ) {
        clear();
        current = candidate;
        show(candidate);
        return;
      }
      clear();
      visible = false;
      current = candidate;
      show(null);
      const ticket = epoch;
      timer = setTimeout(() => {
        if (ticket === epoch && current === candidate && valid(candidate)) {
          visible = true;
          show(candidate);
        }
      }, 400);
    },
    leave() {
      clear();
      const ticket = epoch;
      timer = setTimeout(() => {
        if (ticket === epoch) {
          visible = false;
          current = null;
          show(null);
        }
      }, 150);
    },
    retain() {
      clear();
    },
    cancel() {
      const active = current !== null;
      clear();
      current = null;
      visible = false;
      show(null);
      return active;
    },
    active() {
      return current !== null;
    },
  };
}
