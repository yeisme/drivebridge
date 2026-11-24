import { useId, useState, type ReactNode } from 'react';
import { motion } from 'motion/react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { SPRING_PILL } from '../lib/motion-presets';
import { confirmRouteLeave } from './route-leave';

interface WorkspaceViewsProps {
  label: string;
  primaryLabel: string;
  legacyLabel: string;
  primary: ReactNode;
  legacy: ReactNode;
  initialView?: 'primary' | 'legacy';
  actions?: ReactNode;
}

// Keep protocol-specific surfaces separate. Only the selected view reads data;
// existing owner clients and recovery records remain the source of truth.
// The caller keys this boundary by the full actor/tenant/space identity.
export function WorkspaceViews({ label, primaryLabel, legacyLabel, primary, legacy, initialView = 'primary', actions }: WorkspaceViewsProps) {
  const [view, setView] = useState(initialView);
  const pillId = useId();
  return <Tabs value={view} onValueChange={value => {
    if ((value === 'primary' || value === 'legacy') && value !== view && confirmRouteLeave()) setView(value);
  }} className="min-w-0 gap-4">
    <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
    <TabsList aria-label={label} className="max-w-full min-h-11 flex-wrap justify-start gap-1 rounded-xl border border-border/80 bg-muted/60 p-1 md:min-h-9">
      {([['primary', primaryLabel], ['legacy', legacyLabel]] as const).map(([key, itemLabel]) => (
        <TabsTrigger
          key={key}
          value={key}
          className="relative isolate min-h-9 rounded-lg border-0 bg-transparent px-3 text-[13px] font-medium text-muted-foreground transition-colors data-active:bg-transparent data-active:font-semibold data-active:text-foreground data-active:shadow-none md:min-h-7.5"
        >
          {view === key && (
            <motion.span
              layoutId={`${pillId}-tab-pill`}
              transition={SPRING_PILL}
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 -z-10 rounded-lg bg-background shadow-2xs ring-1 ring-border/70"
            />
          )}
          <span>{itemLabel}</span>
        </TabsTrigger>
      ))}
    </TabsList>
    {actions && <div className="flex max-w-full flex-wrap items-center gap-2">{actions}</div>}
    </div>
    <TabsContent value="primary" className="min-w-0">{view === 'primary' && primary}</TabsContent>
    <TabsContent value="legacy" className="min-w-0">{view === 'legacy' && legacy}</TabsContent>
  </Tabs>;
}
