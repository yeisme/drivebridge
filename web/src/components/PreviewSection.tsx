import { useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { Button } from "./ui/button";
import { useId } from "react";

/** A compact disclosure using the shared shadcn button and focus styles. */
export function PreviewSection({ title, children }: { title: string; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  return <section className="border-t border-border pt-3">
    <Button variant="ghost" type="button" className="h-8 w-full justify-between px-2 text-sm" aria-expanded={open} aria-controls={id} onClick={() => setOpen(!open)}>
      {title}<ChevronDown className={open ? "rotate-180" : ""} />
    </Button>
    <div id={id} hidden={!open} className="px-2 pt-3 pb-2">{children}</div>
  </section>;
}
