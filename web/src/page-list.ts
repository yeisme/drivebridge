import { isTypingTarget } from "./finder-keys";

export type ListCommand = "prev" | "next" | "open" | "cancel" | "escape";

export function stepIndex(index: number, delta: number, length: number) {
  if (length <= 0) return 0;
  return (index + delta + length) % length;
}

export function listCommand(e: Pick<KeyboardEvent, "key" | "target">): ListCommand | null {
  if (isTypingTarget(e.target)) return null;
  if (e.key === "ArrowDown" || e.key === "j") return "next";
  if (e.key === "ArrowUp" || e.key === "k") return "prev";
  if (e.key === "Enter") return "open";
  if (e.key === "Delete" || e.key === "Backspace") return "cancel";
  if (e.key === "Escape") return "escape";
  return null;
}

export function focusListItem(root: HTMLElement, index: number) {
  const items = root.querySelectorAll<HTMLElement>("[data-list-item]");
  items[index]?.focus();
}

export function handleListKey(
  e: KeyboardEvent,
  root: HTMLElement,
  handlers: { open?: (el: HTMLElement) => void; cancel?: (el: HTMLElement) => void },
) {
  const cmd = listCommand(e);
  if (!cmd) return false;
  const items = Array.from(root.querySelectorAll<HTMLElement>("[data-list-item]"));
  if (!items.length) return false;
  const currentEl = (document.activeElement as HTMLElement | null)?.closest("[data-list-item]") as HTMLElement | null;
  const current = currentEl ? items.indexOf(currentEl) : -1;
  const index = current < 0 ? 0 : current;
  if (cmd === "next" || cmd === "prev") {
    e.preventDefault();
    focusListItem(root, stepIndex(index, cmd === "next" ? 1 : -1, items.length));
    return true;
  }
  const el = items[index] || items[0];
  if (cmd === "open" && handlers.open) {
    e.preventDefault();
    handlers.open(el);
    return true;
  }
  if (cmd === "cancel" && handlers.cancel) {
    e.preventDefault();
    handlers.cancel(el);
    return true;
  }
  if (cmd === "escape") {
    (document.activeElement as HTMLElement | null)?.blur();
    return true;
  }
  return false;
}
