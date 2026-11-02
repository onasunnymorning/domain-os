"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ArrowLeft, SlidersHorizontal } from "lucide-react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DeleteConfirmDialog } from "@/components/shared/DeleteConfirmDialog";
import { PolicyForm } from "./PolicyForm";
import { builtInLimits, DEFAULT_POLICY_LIMITS, isBuiltInPolicy } from "@/lib/constants/eppPolicyDefaults";
import { useDeleteEPPPolicy, useEPPPolicy, useSaveEPPPolicy, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPPolicyLimits } from "@/lib/types/epp";

/** Creating (its own route) or editing the policy `name`. */
export function PolicyEditorPage({ name = "", creating = false }: { name?: string; creating?: boolean }) {
  const router = useRouter();
  // A fresh read: a cached copy could be older than a change made elsewhere since.
  const policy = useEPPPolicy(name, !creating, { fresh: true });
  const save = useSaveEPPPolicy();
  const del = useDeleteEPPPolicy();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const builtIn = !creating && isBuiltInPolicy(name);
  // The form mounts once the first fetch has completed, never from a cached copy;
  // a later background refetch does not remount it over unsaved edits.
  const [fetchedAt, setFetchedAt] = useState<number | null>(null);
  if (!creating && policy.data && !policy.isFetching && fetchedAt === null) setFetchedAt(policy.dataUpdatedAt);

  const onSave = async (v: { name: string; description: string; limits: EPPPolicyLimits }) => {
    try {
      await save.mutateAsync({ name: v.name, body: { description: v.description, limits: v.limits }, createOnly: creating });
      toast.success(creating ? `Policy ${v.name} created.` : `Policy ${v.name} saved. Nodes pick it up within about 30 seconds.`);
      router.push(creating ? `/epp/policies/${encodeURIComponent(v.name)}` : "/epp?tab=policies");
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not save the policy"));
    }
  };

  const onDelete = async () => {
    try {
      await del.mutateAsync(name);
      toast.success(`Policy ${name} deleted.`);
      router.push("/epp?tab=policies");
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not delete the policy"));
      setConfirmDelete(false);
    }
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/epp?tab=policies">
            <ArrowLeft className="mr-1 h-4 w-4" /> All policies
          </Link>
        </Button>
        <div className="space-y-2">
          <div className="flex items-baseline gap-3">
            <SlidersHorizontal className="h-10 w-10 text-muted-foreground" />
            <h1 className="text-5xl font-bold tracking-tight">{creating ? "New policy" : name}</h1>
            {builtIn && <Badge variant="secondary">Built-in</Badge>}
          </div>
          <p className="ml-[52px] text-sm text-muted-foreground">
            A named set of limits. Registrars and families are assigned one; there are no per-registrar overrides.
          </p>
        </div>

        {creating ? (
          <PolicyForm
            mode="create"
            initial={{ name: "", description: "", limits: DEFAULT_POLICY_LIMITS }}
            saving={save.isPending}
            onSave={onSave}
          />
        ) : fetchedAt === null && !policy.error ? (
          <Skeleton className="h-64 w-full" />
        ) : policy.error || !policy.data ? (
          <div className="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
            {eppErrorMessage(policy.error, "Could not load the policy.")}
          </div>
        ) : (
          <PolicyForm
            key={fetchedAt}
            mode="edit"
            initial={{ name: policy.data.name, description: policy.data.description, limits: policy.data.limits }}
            builtIn={builtIn}
            resetValues={builtInLimits(name)}
            saving={save.isPending}
            onSave={onSave}
            onDelete={() => setConfirmDelete(true)}
          />
        )}

        <DeleteConfirmDialog
          open={confirmDelete}
          onOpenChange={setConfirmDelete}
          title={`Delete policy ${name}?`}
          description="Only a policy no registrar or family is assigned can be deleted. Registrars on it would have to be moved first; the API refuses otherwise and says so."
          onConfirm={onDelete}
          isDeleting={del.isPending}
        />
      </div>
    </DashboardLayout>
  );
}
