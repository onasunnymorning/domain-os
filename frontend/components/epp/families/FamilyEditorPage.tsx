"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ArrowLeft, Users } from "lucide-react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DeleteConfirmDialog } from "@/components/shared/DeleteConfirmDialog";
import { FamilyForm, type FamilyFormValues } from "./FamilyForm";
import { useDeleteRegistrarFamily, useRegistrarFamily, useSaveRegistrarFamily, eppErrorMessage, isVersionConflict } from "@/lib/hooks/useEPP";
import { useFreshRead } from "@/components/epp/useFreshRead";
import { VersionConflictNotice } from "@/components/epp/VersionConflictNotice";

/**
 * Which member a 409 "already in another family" message is about. The API
 * says "…: <clid> is in <family>"; the client ID is read whole and compared
 * exactly, so rar-001 is never taken for rar-0011.
 */
export function memberNamedIn(message: string, members: string[]): string | undefined {
  const m = /:\s*(\S+) is in \S+\s*$/.exec(message);
  if (!m) return undefined;
  return members.find((c) => c === m[1]);
}

/** Creating (its own route) or editing the family `name`. */
export function FamilyEditorPage({ name = "", creating = false }: { name?: string; creating?: boolean }) {
  const router = useRouter();
  const family = useRegistrarFamily(name, !creating, { fresh: true });
  const save = useSaveRegistrarFamily();
  const del = useDeleteRegistrarFamily();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [memberError, setMemberError] = useState<{ clid: string; message: string } | null>(null);
  const { fetchedAt, initialError } = useFreshRead(family);
  const [loaded, setLoaded] = useState<{ at: number; updatedAt: string } | null>(null);
  if (fetchedAt !== null && family.data && loaded?.at !== fetchedAt) setLoaded({ at: fetchedAt, updatedAt: family.data.updated_at });
  const [conflict, setConflict] = useState(false);

  const onSave = async (v: FamilyFormValues) => {
    setMemberError(null);
    setConflict(false);
    try {
      await save.mutateAsync({
        name: v.name,
        body: { description: v.description, policy: v.policy, members: v.members },
        createOnly: creating,
        ifUnchangedSince: creating ? undefined : loaded?.updatedAt,
      });
      toast.success(creating ? `Family ${v.name} created.` : `Family ${v.name} saved.`);
      router.push("/epp?tab=families");
    } catch (err) {
      if (isVersionConflict(err)) {
        setConflict(true);
        return;
      }
      const msg = eppErrorMessage(err, "Could not save the family");
      const clid = memberNamedIn(msg, v.members);
      if (clid) setMemberError({ clid, message: msg });
      toast.error(msg);
    }
  };

  const onDelete = async () => {
    try {
      await del.mutateAsync(name);
      toast.success(`Family ${name} deleted. Its members keep their own policies.`);
      router.push("/epp?tab=families");
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not delete the family"));
      setConfirmDelete(false);
    }
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/epp?tab=families">
            <ArrowLeft className="mr-1 h-4 w-4" /> All families
          </Link>
        </Button>
        <div className="space-y-2">
          <div className="flex items-baseline gap-3">
            <Users className="h-10 w-10 text-muted-foreground" />
            <h1 className="text-5xl font-bold tracking-tight">{creating ? "New family" : name}</h1>
          </div>
          <p className="ml-[52px] text-sm text-muted-foreground">Affiliated registrars whose sessions and commands are also counted together.</p>
        </div>

        {creating ? (
          <FamilyForm mode="create" initial={{ name: "", description: "", policy: "", members: [] }} saving={save.isPending} memberError={memberError} onSave={onSave} />
        ) : initialError ? (
          <div className="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
            {eppErrorMessage(initialError, "Could not load the family.")}
          </div>
        ) : fetchedAt === null || !family.data ? (
          <Skeleton className="h-64 w-full" />
        ) : (
          <>
          {conflict && <VersionConflictNotice what="family" onReload={() => window.location.reload()} />}
          <FamilyForm
            key={fetchedAt}
            mode="edit"
            initial={{ name: family.data.name, description: family.data.description ?? "", policy: family.data.policy ?? "", members: family.data.members ?? [] }}
            saving={save.isPending}
            memberError={memberError}
            onSave={onSave}
            onDelete={() => setConfirmDelete(true)}
          />
          </>
        )}

        <DeleteConfirmDialog
          open={confirmDelete}
          onOpenChange={setConfirmDelete}
          title={`Delete family ${name}?`}
          description="Its members keep their own policies; they just stop being counted together."
          onConfirm={onDelete}
          isDeleting={del.isPending}
        />
      </div>
    </DashboardLayout>
  );
}
