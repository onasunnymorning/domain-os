"use client";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { format } from "date-fns";
import type { EPPCredentialLifecycle } from "@/lib/types/epp";

/**
 * One badge, one colour per state, the same for passwords, certificates and
 * networks. Certificates add "Expired" and "Not yet valid" from their dates.
 */
export function CredentialStateBadge({
  credential,
  notBefore,
  notAfter,
  className,
}: {
  credential: Pick<EPPCredentialLifecycle, "state" | "retire_at" | "retire_kind">;
  notBefore?: string;
  notAfter?: string;
  className?: string;
}) {
  const now = Date.now();
  const retired = credential.state === "retired";

  let label: string;
  let classes: string;
  if (retired) {
    label = credential.retire_kind === "compromise" ? "Revoked" : "Retired";
    classes = "bg-muted text-muted-foreground border-transparent";
  } else if (notAfter && new Date(notAfter).getTime() <= now) {
    label = "Expired";
    classes = "bg-red-100 text-red-800 border-transparent dark:bg-red-950 dark:text-red-300";
  } else if (notBefore && new Date(notBefore).getTime() > now) {
    label = "Not yet valid";
    classes = "bg-blue-100 text-blue-800 border-transparent dark:bg-blue-950 dark:text-blue-300";
  } else if (credential.state === "retiring") {
    label = credential.retire_at ? `Retiring until ${format(new Date(credential.retire_at), "MMM d, HH:mm")}` : "Retiring";
    classes = "bg-amber-100 text-amber-900 border-transparent dark:bg-amber-950 dark:text-amber-300";
  } else {
    label = "Active";
    classes = "bg-emerald-100 text-emerald-800 border-transparent dark:bg-emerald-950 dark:text-emerald-300";
  }

  return (
    <Badge variant="outline" className={cn("whitespace-nowrap", classes, className)}>
      {label}
    </Badge>
  );
}
