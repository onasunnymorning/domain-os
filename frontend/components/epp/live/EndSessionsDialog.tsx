"use client";

import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useEndEPPSession, useEndEPPSessionsMatching, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPSessionListParams } from "@/lib/types/epp";

export type EndSessionsTarget =
  | { kind: "one"; id: string; label: string }
  | {
      kind: "matching";
      filter: Pick<EPPSessionListParams, "clid" | "login" | "client_ip" | "credential_id">;
      /** How many are listed on the current page: a lower bound, never the total. The API ends every match. */
      listed: number;
      label: string;
    };

/**
 * Ends one session, or every session a filter matches. Says what the API
 * does: it accepts the request, and the node holding the session closes it
 * within moments; the registrar's client can log in again at once.
 */
export function EndSessionsDialog({ target, onOpenChange }: { target: EndSessionsTarget | null; onOpenChange: (open: boolean) => void }) {
  const endOne = useEndEPPSession();
  const endMatching = useEndEPPSessionsMatching();
  const pending = endOne.isPending || endMatching.isPending;

  const confirm = async () => {
    if (!target) return;
    try {
      if (target.kind === "one") {
        await endOne.mutateAsync(target.id);
        toast.success("Session end requested. It disappears from the list within a few seconds.");
      } else {
        const res = await endMatching.mutateAsync(target.filter);
        toast.success(`${res.killed} session${res.killed === 1 ? "" : "s"} asked to end. They disappear within a few seconds.`);
      }
      onOpenChange(false);
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not end the session"));
    }
  };

  return (
    <AlertDialog open={target !== null} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {target?.kind === "one" ? `End session of ${target.label}?` : `End every session ${target?.label ?? ""}?`}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {target?.kind === "matching" && (
              <>
                Every matching session on every node is ended, not only the {target.listed} listed here.{" "}
              </>
            )}
            The registrar&apos;s client is disconnected and can log in again at once. The API accepts the request; the
            node holding the session closes it within moments. Nothing else changes: no credential is retired.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={(e) => {
              e.preventDefault();
              confirm();
            }}
            disabled={pending}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {pending ? "Ending…" : target?.kind === "one" ? "End session" : "End all matching"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
