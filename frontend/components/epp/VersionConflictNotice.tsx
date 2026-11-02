"use client";

import { Button } from "@/components/ui/button";

/** Shown when a save was refused because someone else saved first (412). */
export function VersionConflictNotice({ what, onReload }: { what: string; onReload: () => void }) {
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200"
    >
      <span>
        This {what} was changed by someone else since you opened it, so your save was not applied. Reload to see
        the current values, then make your change again.
      </span>
      <Button type="button" size="sm" variant="outline" onClick={onReload}>
        Reload
      </Button>
    </div>
  );
}
