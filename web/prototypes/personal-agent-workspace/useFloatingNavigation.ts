import { useCallback, useEffect, useRef, useState } from "react";
import type { PointerEvent } from "react";

// Navigation has its own timing; file hover previews retain their existing 400ms contract.
export const navigationOpenDelay = 150;
export const navigationCloseDelay = 220;
export function useFloatingNavigation({
  docked,
  disabled,
  width,
  scope,
}: {
  docked: boolean;
  disabled: boolean;
  width: number;
  scope: string;
}) {
  const [open, setOpen] = useState(false);
  const [fine, setFine] = useState(
    () => matchMedia("(hover: hover) and (pointer: fine)").matches,
  );
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const panel = useRef<HTMLElement | null>(null),
    trigger = useRef<HTMLElement | null>(null);
  const held = useRef(false);
  const inside = useRef(false),
    focusOnOpen = useRef(false);
  const latest = useRef({ open, docked, disabled });
  latest.current = { open, docked, disabled };
  const cancel = useCallback(() => {
    clearTimeout(timer.current);
    timer.current = undefined;
  }, []);
  const setVisible = useCallback(
    (value: boolean) => {
      cancel();
      if (!value) {
        held.current = false;
        inside.current = false;
        focusOnOpen.current = false;
        if (panel.current?.contains(document.activeElement))
          trigger.current?.focus({ preventScroll: true });
      }
      setOpen(value);
    },
    [cancel],
  );
  const obscured = () =>
    Array.from(
      document.querySelectorAll(
        '[role="dialog"], [role="menu"], [role="listbox"]',
      ),
    ).some(
      (el) =>
        !el.hasAttribute("data-floating-navigation") &&
        !el.closest("[data-hover-preview]") &&
        el.getClientRects().length > 0 &&
        !el.hasAttribute("data-closed"),
    );
  const canHover = fine && width >= 768 && !docked && !disabled;
  const enter = (event: PointerEvent) => {
    if (
      event.pointerType !== "mouse" ||
      event.buttons > 0 ||
      !canHover ||
      obscured()
    )
      return;
    inside.current = true;
    cancel();
    if (latest.current.open) return;
    timer.current = setTimeout(() => {
      if (
        inside.current &&
        !latest.current.docked &&
        !latest.current.disabled &&
        !obscured()
      )
        setOpen(true);
    }, navigationOpenDelay);
  };
  const retain = (event: PointerEvent) => {
    if (event.pointerType !== "mouse") return;
    inside.current = true;
    cancel();
  };
  const leave = (event: PointerEvent) => {
    if (event.pointerType !== "mouse") return;
    inside.current = false;
    cancel();
    timer.current = setTimeout(() => {
      if (
        !held.current &&
        !inside.current &&
        !panel.current?.contains(document.activeElement)
      )
        setVisible(false);
    }, navigationCloseDelay);
  };
  const toggle = (keyboard = false) => {
    focusOnOpen.current = keyboard && !latest.current.open;
    if (latest.current.open) setVisible(false);
    else {
      cancel();
      setOpen(true);
    }
  };
  useEffect(() => {
    const query = matchMedia("(hover: hover) and (pointer: fine)");
    const update = () => {
      setFine(query.matches);
      setVisible(false);
    };
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, [setVisible]);
  useEffect(
    () => setVisible(false),
    [docked, disabled, width, scope, setVisible],
  );
  useEffect(() => () => cancel(), [cancel]);
  useEffect(() => {
    const dismiss = () => setVisible(false);
    window.addEventListener("blur", dismiss);
    window.addEventListener("dragstart", dismiss, true);
    return () => {
      window.removeEventListener("blur", dismiss);
      window.removeEventListener("dragstart", dismiss, true);
    };
  }, [setVisible]);
  useEffect(() => {
    if (!open || !fine || width < 768) return;
    if (focusOnOpen.current) {
      focusOnOpen.current = false;
      panel.current
        ?.querySelector<HTMLElement>(
          'nav button[aria-current="page"], nav button',
        )
        ?.focus({ preventScroll: true });
    }
    const outside = (event: globalThis.PointerEvent) => {
      if (held.current) return;
      const target = event.target as Node;
      if (
        !panel.current?.contains(target) &&
        !trigger.current?.contains(target)
      )
        setVisible(false);
    };
    const key = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented || obscured())
        return;
      event.preventDefault();
      event.stopImmediatePropagation();
      setVisible(false);
    };
    window.addEventListener("pointerdown", outside, true);
    window.addEventListener("keydown", key, true);
    return () => {
      window.removeEventListener("pointerdown", outside, true);
      window.removeEventListener("keydown", key, true);
    };
  }, [open, fine, width, setVisible]);
  return {
    open,
    setOpen: setVisible,
    panel,
    trigger,
    canHover,
    floating: fine && width >= 768,
    enter,
    retain,
    leave,
    toggle,
    cancel,
    hold: (value: boolean) => {
      held.current = value;
      cancel();
    },
    onFocus: () => cancel(),
    onBlur: (related: EventTarget | null) => {
      if (
        held.current ||
        (related instanceof Node && panel.current?.contains(related))
      )
        return;
      setVisible(false);
    },
  };
}
export type FloatingNavigation = ReturnType<typeof useFloatingNavigation>;
