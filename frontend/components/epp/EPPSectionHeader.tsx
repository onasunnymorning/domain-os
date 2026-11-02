import type { ReactNode } from "react";
import { CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

/**
 * A section title with the one sentence that explains it, and room for an
 * action button on the right. Every EPP section uses it so the page reads the
 * same way top to bottom.
 */
export function EPPSectionHeader({
  title,
  description,
  action,
  count,
}: {
  title: string;
  description: ReactNode;
  action?: ReactNode;
  /** e.g. "2 of 3 usable" */
  count?: string;
}) {
  return (
    <CardHeader className="flex flex-row items-start justify-between gap-4">
      <div className="space-y-1">
        <CardTitle className="flex items-center gap-2">
          {title}
          {count && <span className="text-xs font-normal text-muted-foreground">{count}</span>}
        </CardTitle>
        <CardDescription className="max-w-prose">{description}</CardDescription>
      </div>
      {action && <div className="shrink-0 pt-1">{action}</div>}
    </CardHeader>
  );
}
