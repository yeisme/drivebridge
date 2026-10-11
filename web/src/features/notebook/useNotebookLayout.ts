import { useLayoutEffect, useRef, useState } from "react";

export function notebookLayout(width: number) {
  return { directoryInline: width >= 1024, propertiesInline: width >= 1280 };
}

// Measure the workspace, not the viewport: it may live inside a preview dialog.
export function useNotebookLayout() {
  const container = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  useLayoutEffect(() => {
    const element = container.current;
    if (!element) return;
    const measure = () => setWidth(element.getBoundingClientRect().width);
    measure();
    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", measure);
      return () => window.removeEventListener("resize", measure);
    }
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  return { container, ...notebookLayout(width) };
}
