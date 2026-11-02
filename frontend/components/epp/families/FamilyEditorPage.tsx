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
import { useDeleteRegistrarFamily, useRegistrarFamily, useSaveRegistrarFamily, eppErrorMessage } from "@/lib/hooks/useEPP";

/** Finds which member a 409 "already in family" message is about. */
export function memberNamedIn(message: string, members: string[]): string | undefined {
  return members.find((m) => message.includes(m));
}

export function FamilyEditorPage({ name }: { name: string }) {
  const router = useRouter();
  const creating = name === "new";
  const family = useRegistrarFamily(name, !creating);
  const save = useSaveRegistrarFamily();
  const del = useDeleteRegistrarFamily();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [memberError, setMemberError] = useState<{ clid: string; message: string } | null>(null);

  const onSave = async (v: FamilyFormValues) => {
    setMemberError(null);
    try {
      await save.mutateAsync({ name: v.name, body: { description: v.description, policy: v.policy, members: v.members } });
      toast.success(creating ? `Family ${v.name} created.` : `Family ${v.name} saved.`);
      router.push("/epp?tab=families");
    } catch (err) {
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
        ) : family.isLoading ? (
          <Skeleton className="h-64 w-full" />
        ) : family.error || !family.data ? (
          <div className="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
            {eppErrorMessage(family.error, "Could not load the family.")}
          </div>
        ) : (
          <FamilyForm
            mode="edit"
            initial={{ name: family.data.name, description: family.data.description ?? "", policy: family.data.policy ?? "", members: family.data.members ?? [] }}
            saving={save.isPending}
            memberError={memberError}
            onSave={onSave}
            onDelete={() => setConfirmDelete(true)}
          />
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
