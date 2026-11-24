import type { Transition } from "motion/react";

/** Snappy macOS-grade spring for sliding active pills and view toggles. */
export const SPRING_PILL: Transition = {
  type: "spring",
  stiffness: 460,
  damping: 36,
  mass: 0.75,
};

/** Tactile spring for floating docks, transfer drawers, and popovers. */
export const SPRING_DOCK: Transition = {
  type: "spring",
  stiffness: 380,
  damping: 30,
  mass: 0.85,
};

/** Gentle hover/card elevation spring without overshoot wobble. */
export const SPRING_CARD: Transition = {
  type: "spring",
  stiffness: 340,
  damping: 28,
  mass: 0.8,
};

/** Fast micro-fade for file grid/list items on directory or view switch. */
export const EASE_OUT_CRISP: Transition = {
  duration: 0.18,
  ease: [0.16, 1, 0.3, 1],
};
