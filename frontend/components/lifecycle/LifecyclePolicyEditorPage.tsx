"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ArrowLeft, Clock } from "lucide-react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DeleteConfirmDialog } from "@/components/shared/DeleteConfirmDialog";
import { useFreshRead } from "@/components/epp/useFreshRead";
import { VersionConflictNotice } from "@/components/epp/VersionConflictNotice";
import { eppErrorMessage, isVersionConflict } from "@/lib/hooks/useEPP";
import { useDeleteLifecyclePolicy, useLifecyclePolicy, useSaveLifecyclePolicy } from "@/lib/hooks/useLifecyclePolicies";
import { EMPTY_LIFECYCLE_SETTINGS, isStandardLifecyclePolicy } from "@/lib/constants/lifecycleDefaults";
import type { LifecycleSettings } from "@/lib/types/lifecycle";
import { LifecyclePolicyForm } from "./LifecyclePolicyForm";

/** Creating (its own route) or editing the lifecycle policy `name`. */
export function LifecyclePolicyEditorPage({ name = "", creating = false }: { name?: string; creating?: boolean }) {
  const router = useRouter();
  // A fresh read: a cached copy could be older than a change made elsewhere since.
  const policy = useLifecyclePolicy(name, !creating, { fresh: true });
  const save = useSaveLifecyclePolicy();
  const del = useDeleteLifecyclePolicy();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const builtIn = !creating && isStandardLifecyclePolicy(name);
  const { fetchedAt, initialError } = useFreshRead(policy);
  const [loaded, setLoaded] = useState<{ at: number; updatedAt: string } | null>(null);
  if (fetchedAt !== null && policy.data && loaded?.at !== fetchedAt) setLoaded({ at: fetchedAt, updatedAt: policy.data.updatedAt });
  const [conflict, setConflict] = useState(false);

  const onSave = async (v: { name: string; description: string; settings: LifecycleSettings }) => {
    setConflict(false);
    try {
      await save.mutateAsync({
        name: v.name,
        body: { description: v.description, settings: v.settings },
        createOnly: creating,
        ifUnchangedSince: creating ? undefined : loaded?.updatedAt,
      });
      const reach = policy.data?.usedBy?.length ?? 0;
      toast.success(creating ? `Policy ${v.name} created.` : `Policy ${v.name} saved${reach ? `; it applies to ${reach} phase${reach === 1 ? "" : "s"} from now on` : ""}.`);
      router.push(creating ? `/lifecycle-policies/${encodeURIComponent(v.name)}` : "/lifecycle-policies");
    } catch (err) {
      if (!creating && isVersionConflict(err)) {
        setConflict(true);
        return;
      }
      toast.error(eppErrorMessage(err, "Could not save the policy"));
    }
  };

  const onDelete = async () => {
    try {
      await del.mutateAsync(name);
      toast.success(`Policy ${name} deleted.`);
      router.push("/lifecycle-policies");
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not delete the policy"));
      setConfirmDelete(false);
    }
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/lifecycle-policies">
            <ArrowLeft className="mr-1 h-4 w-4" /> All lifecycle policies
          </Link>
        </Button>
        <div className="space-y-2">
          <div className="flex items-baseline gap-3">
            <Clock className="h-10 w-10 text-muted-foreground" />
            <h1 className="text-5xl font-bold tracking-tight">{creating ? "New lifecycle policy" : name}</h1>
            {builtIn && <Badge variant="secondary">Built-in</Badge>}
          </div>
          <p className="ml-[52px] text-sm text-muted-foreground">
            The timing every phase that names this policy follows. A setting left empty is the standard value; 0 is zero days.
          </p>
        </div>

        {creating ? (
          <LifecyclePolicyForm
            mode="create"
            initial={{ name: "", description: "", settings: EMPTY_LIFECYCLE_SETTINGS }}
            saving={save.isPending}
            onSave={onSave}
          />
        ) : initialError ? (
          <div className="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
            {eppErrorMessage(initialError, "Could not load the policy.")}
          </div>
        ) : fetchedAt === null || !policy.data ? (
          <Skeleton className="h-64 w-full" />
        ) : (
          <>
            {conflict && <VersionConflictNotice what="policy" onReload={() => window.location.reload()} />}
            <LifecyclePolicyForm
              key={fetchedAt}
              mode="edit"
              initial={{ name: policy.data.name, description: policy.data.description, settings: policy.data.settings }}
              usedBy={policy.data.usedBy ?? []}
              builtIn={builtIn}
              saving={save.isPending}
              onSave={onSave}
              onDelete={() => setConfirmDelete(true)}
            />
          </>
        )}

        <DeleteConfirmDialog
          open={confirmDelete}
          onOpenChange={setConfirmDelete}
          title={`Delete policy ${name}?`}
          description="Only a policy no phase names can be deleted. Phases on it would have to be moved to another policy first; the API refuses otherwise and says so."
          onConfirm={onDelete}
          isDeleting={del.isPending}
        />
      </div>
    </DashboardLayout>
  );
}
