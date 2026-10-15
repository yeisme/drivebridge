import { Button } from "./components/ui/button";
import React, { useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export interface MenuAction {
  type?: "item";
  label: string;
  run: () => void;
  disabled?: boolean;
  danger?: boolean;
  shortcut?: string;
  icon?: React.ReactNode;
}
export type MenuItem = MenuAction | { type: "separator" };

export function isMenuAction(item: MenuItem): item is MenuAction {
  return item.type !== "separator";
}

function useActionSheet() {
  if (typeof window === "undefined") return false;
  return (
    window.matchMedia("(pointer: coarse)").matches || window.innerWidth < 640
  );
}

export function FinderMenu({
  x,
  y,
  items,
  close,
}: {
  x: number;
  y: number;
  items: MenuItem[];
  close: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const sheet = useActionSheet();
  const [position, setPosition] = useState(
    sheet ? { left: 8, top: 8 } : { left: x, top: y },
  );
  const closeRef = useRef(close);
  closeRef.current = close;
  useLayoutEffect(() => {
    const previous = document.activeElement as HTMLElement;
    const place = () => {
      const box = ref.current!.getBoundingClientRect();
      if (
        window.matchMedia("(pointer: coarse)").matches ||
        window.innerWidth < 640
      ) {
        setPosition({
          left: 8,
          top: Math.max(8, innerHeight - box.height - 8),
        });
        return;
      }
      setPosition({
        left: Math.max(8, Math.min(x, innerWidth - box.width - 8)),
        top: Math.max(8, Math.min(y, innerHeight - box.height - 8)),
      });
    };
    place();
    ref.current
      ?.querySelector<HTMLButtonElement>("button:not(:disabled)")
      ?.focus({ preventScroll: true });
    const outside = (e: Event) => {
      if (!ref.current?.contains(e.target as Node)) closeRef.current();
    };
    document.addEventListener("pointerdown", outside);
    window.addEventListener("resize", place);
    window.addEventListener("wheel", outside, { passive: true });
    return () => {
      document.removeEventListener("pointerdown", outside);
      window.removeEventListener("resize", place);
      window.removeEventListener("wheel", outside);
      previous?.focus({ preventScroll: true });
    };
  }, [x, y]);
  const sheetNow =
    typeof window !== "undefined" &&
    (window.matchMedia("(pointer: coarse)").matches || window.innerWidth < 640);
  return createPortal(
    <div
      ref={ref}
      className={
        "finder-context-menu" + (sheetNow ? " sheet" : "")
      }
      role="menu"
      aria-label="文件操作菜单"
      style={position}
      onContextMenu={(e) => e.preventDefault()}
      onKeyDown={(e) => {
        const buttons = Array.from(
          ref.current!.querySelectorAll<HTMLButtonElement>(
            "button:not(:disabled)",
          ),
        );
        const index = buttons.indexOf(
          document.activeElement as HTMLButtonElement,
        );
        if (e.key === "Escape" || e.key === "Tab") {
          e.preventDefault();
          close();
        }
        if (["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) {
          e.preventDefault();
          const next =
            e.key === "Home"
              ? 0
              : e.key === "End"
                ? buttons.length - 1
                : (index + (e.key === "ArrowDown" ? 1 : -1) + buttons.length) %
                  buttons.length;
          buttons[next]?.focus({ preventScroll: true });
        }
      }}
    >
      {items.map((item, index) =>
        isMenuAction(item) ? (
          <Button variant="ghost" type="button"
            role="menuitem"
            key={item.label}
            disabled={item.disabled}
            className={`w-full justify-between gap-4 text-left ${item.danger ? "danger" : ""}`}
            aria-keyshortcuts={item.shortcut}
            onClick={() => {
              close();
              item.run();
            }}
          >
            <span className="menu-label">
              {item.icon}
              <span>{item.label}</span>
            </span>
            {item.shortcut && (
              <span className="menu-shortcut" aria-hidden="true">
                {item.shortcut}
              </span>
            )}
          </Button>
        ) : (
          <div key={"sep-" + index} className="menu-separator" role="separator" />
        ),
      )}
    </div>,
    document.body,
  );
}

export function newEntryError(value: string, multiple: boolean) {
  const name = value.trim();
  if (!name) return "请输入名称。";
  if (/[\\\u0000-\u001f\u007f]/.test(name))
    return "名称不能包含反斜杠或控制字符。";
  if (!multiple && name.includes("/"))
    return "名称不能包含 /；创建多级目录请开启对应选项。";
  if (
    name
      .split("/")
      .some(
        (part) =>
          !part || part === "." || part === ".." || part !== part.trim(),
      )
  )
    return "目录层级不能为空，也不能使用 .、.. 或首尾空格。";
  if (new TextEncoder().encode(name).length > 900)
    return "名称或目录层级过长。";
  return "";
}
