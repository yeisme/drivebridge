import type { ReactNode } from "react";
import { Button } from "./ui/button";
import { Skeleton } from "./ui/skeleton";
export function AsyncState({
  loading,
  error,
  onRetry,
  children,
}: {
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
  children: ReactNode;
}) {
  if (loading)
    return (
      <div role="status" className="space-y-3 p-4">
        <span className="sr-only">正在加载…</span>
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-3/4" />
      </div>
    );
  if (error)
    return (
      <div
        role="alert"
        className="space-y-2 rounded-lg border border-destructive/30 p-4 text-sm text-destructive"
      >
        {error}
        {onRetry && (
          <Button type="button" variant="outline" onClick={onRetry}>
            重试
          </Button>
        )}
      </div>
    );
  return children;
}
