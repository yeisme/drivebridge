import { useState, type ReactNode } from 'react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
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
  return <Tabs value={view} onValueChange={value => {
    if ((value === 'primary' || value === 'legacy') && value !== view && confirmRouteLeave()) setView(value);
  }} className="min-w-0 gap-4">
    <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
    <TabsList aria-label={label} variant="line" className="max-w-full min-h-11 flex-wrap justify-start gap-3 border-b border-border px-0 pb-1 md:min-h-9">
      <TabsTrigger value="primary" className="min-h-11 px-1 text-[13px] md:min-h-8">{primaryLabel}</TabsTrigger>
      <TabsTrigger value="legacy" className="min-h-11 px-1 text-[13px] md:min-h-8">{legacyLabel}</TabsTrigger>
    </TabsList>
    {actions && <div className="flex max-w-full flex-wrap items-center gap-2">{actions}</div>}
    </div>
    <TabsContent value="primary" className="min-w-0">{view === 'primary' && primary}</TabsContent>
    <TabsContent value="legacy" className="min-w-0">{view === 'legacy' && legacy}</TabsContent>
  </Tabs>;
}
