import type { ReactNode } from 'react';

// Product pages share one compact hierarchy without another framed card.
export function WorkspacePageHeader({ title, description, actions }: { title: string; description?: ReactNode; actions?: ReactNode }) {
  return <header className="flex min-w-0 flex-wrap items-center justify-between gap-4 border-b border-border pb-4">
    <div className={description ? "min-w-0 flex-[1_1_18rem]" : "min-w-0 flex-[1_1_8rem]"}>
      <h2 className="m-0 text-xl leading-7 font-semibold tracking-tight">{title}</h2>
      {description && <div className="mt-1.5 max-w-2xl text-[13px] leading-5 text-muted-foreground">{description}</div>}
    </div>
    {actions && <div className="flex max-w-full flex-wrap items-center gap-2">{actions}</div>}
  </header>;
}
