export type FinderCommand =
  | "copy"
  | "cut"
  | "paste"
  | "duplicate"
  | "selectAll"
  | "trash"
  | "rename"
  | "preview"
  | "open"
  | "escape"
  | "help"
  | "focusPrev"
  | "focusNext"
  | "extendPrev"
  | "extendNext"
  | "goParent"
  | "undo";

export function isApplePlatform() {
  if (typeof navigator === "undefined") return false;
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform);
}

export function isTypingTarget(target: EventTarget | null) {
  const el = target as HTMLElement | null;
  if (!el) return false;
  if (el.isContentEditable) return true;
  const tag = el.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

export function isMediaTarget(target: EventTarget | null) {
  const tag = (target as HTMLElement | null)?.tagName;
  return tag === "VIDEO" || tag === "AUDIO";
}

export function shortcutLabel(key: string) {
  const mod = isApplePlatform() ? "⌘" : "Ctrl+";
  return mod + key;
}

export function finderCommand(e: KeyboardEvent): FinderCommand | null {
  if (isTypingTarget(e.target) || isMediaTarget(e.target)) return null;
  const key = e.key.length === 1 ? e.key.toLowerCase() : e.key;
  const mod = e.metaKey || e.ctrlKey;
  if (mod && key === "c") return "copy";
  if (mod && key === "x") return "cut";
  if (mod && key === "v") return "paste";
  if (mod && key === "d") return "duplicate";
  if (mod && key === "a") return "selectAll";
  if (mod && key === "z") return "undo";
  if (mod && key === "/") return "help";
  if (key === "?" && !mod) return "help";
  if (key === "Delete" || (key === "Backspace" && !mod)) return "trash";
  if (key === "F2") return "rename";
  if (key === "Enter") return "open";
  if (key === " ") return "preview";
  if (key === "Escape") return "escape";
  if (key === "ArrowDown")
    return e.shiftKey ? "extendNext" : "focusNext";
  if (key === "ArrowUp") return e.shiftKey ? "extendPrev" : "focusPrev";
  if (key === "ArrowRight") return "open";
  if (key === "ArrowLeft") return "goParent";
  return null;
}

export function isTypeaheadKey(e: KeyboardEvent) {
  if (isTypingTarget(e.target) || isMediaTarget(e.target)) return false;
  if (e.metaKey || e.ctrlKey || e.altKey) return false;
  if (e.key.length !== 1 || e.key === " ") return false;
  return !/[\p{Cc}\p{Cn}]/u.test(e.key);
}

export function typeaheadIndex(
  names: string[],
  buffer: string,
  from: number,
) {
  if (!names.length || !buffer) return -1;
  const q = buffer.toLocaleLowerCase("zh-CN");
  const repeated = q.length > 1 && [...q].every((c) => c === q[0]);
  const needle = repeated ? q[0] : q;
  const begin = repeated ? from + 1 : from;
  for (let i = 0; i < names.length; i++) {
    const idx = (begin + i + names.length) % names.length;
    if (names[idx].toLocaleLowerCase("zh-CN").startsWith(needle)) return idx;
  }
  return -1;
}
