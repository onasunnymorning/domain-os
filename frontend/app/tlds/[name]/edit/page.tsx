"use client";

import { use, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";
import { ArrowLeft, Globe, Trash2 } from "lucide-react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTLD, useUpdateTLD, useDeleteTLD } from "@/lib/hooks/useTLDs";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { toast } from 'sonner';

interface Props {
  params: Promise<{ name: string }>;
}

// Only AllowEscrowImport is currently editable; DNS toggle is removed until backend supports it
const schema = z.object({
  AllowEscrowImport: z.boolean().optional(),
});

type FormValues = z.infer<typeof schema>;

export default function EditTLDPage({ params }: Props) {
  const { name } = use(params);
  const router = useRouter();
  const tldName = decodeURIComponent(name);
  const { data: tld, isLoading } = useTLD(tldName);
  const { mutateAsync: updateTLD, isPending: isUpdating } = useUpdateTLD();
  const { mutate: deleteTLD, isPending: isDeleting } = useDeleteTLD();

  // Delete TLD Modal state
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState('');
  const [keepTLDAndPhases, setKeepTLDAndPhases] = useState(false);

  const hasActiveOrFuturePhases = useMemo(() => {
    if (!tld?.Phases || !Array.isArray(tld.Phases)) return false;
    const now = new Date();
    return tld.Phases.some((phase: any) => {
      if (!phase.startDate) return false;
      const start = new Date(phase.startDate);
      if (start > now) return true;
      if (!phase.endDate) return true;
      const end = new Date(phase.endDate);
      if (end >= now) return true;
      return false;
    });
  }, [tld]);

  const handleDelete = () => {
    deleteTLD({ name, keepTLDAndPhases }, {
      onSuccess: (data: any) => {
        setDeleteOpen(false);
        setDeleteConfirmText('');
        setKeepTLDAndPhases(false);
        if (data?.url) {
          toast.success(`Cleanup workflow started for ${tldName}`, {
            action: {
              label: 'View in Temporal',
              onClick: () => window.open(data.url, '_blank'),
            },
            duration: 10000,
          });
        } else {
          toast.success(`Cleanup workflow started for ${tldName}`);
        }
        router.push('/tlds');
      },
    });
  };

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      AllowEscrowImport: false,
    },
  });

  // populate when loaded (run in effect to avoid setState during render)
  useEffect(() => {
    if (!isLoading && tld && !form.formState.isDirty) {
      form.reset({
        AllowEscrowImport: !!tld.AllowEscrowImport,
      });
    }
  }, [tld, isLoading, form]);

  const onSubmit = async (values: FormValues) => {
    await updateTLD({ name: tldName, data: values });
    router.push(`/tlds/${encodeURIComponent(tldName)}`);
  };

  return (
    <DashboardLayout>
      <div className="max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" asChild>
            <Link href={`/tlds/${encodeURIComponent(tldName)}`}>
              <ArrowLeft className="mr-2 h-4 w-4" /> Back
            </Link>
          </Button>
          <div className="flex items-center gap-2 text-muted-foreground">
            <Globe className="h-5 w-5" />
            <span className="text-sm">Edit TLD</span>
          </div>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>{tldName}</CardTitle>
            <CardDescription>Update settings for this TLD</CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                <FormField
                  control={form.control}
                  name="AllowEscrowImport"
                  render={({ field }) => (
                    <FormItem className="flex items-center justify-between border rounded-md p-3">
                      <div>
                        <FormLabel>Escrow Import</FormLabel>
                        <FormMessage />
                      </div>
                      <FormControl>
                        <Switch checked={!!field.value} onCheckedChange={field.onChange} />
                      </FormControl>
                    </FormItem>
                  )}
                />

                <div className="flex items-center justify-end gap-2">
                  <Button type="button" variant="outline" onClick={() => router.push(`/tlds/${encodeURIComponent(tldName)}`)}>
                    Cancel
                  </Button>
                  <Button type="submit" disabled={isUpdating}>Save Changes</Button>
                </div>
              </form>
            </Form>
          </CardContent>
        </Card>

        {/* Danger Zone */}
        {!isLoading && tld && (
          <Card className="border-destructive/50">
            <CardHeader>
              <CardTitle className="text-destructive">Danger Zone</CardTitle>
              <CardDescription>
                Irreversible and destructive actions for this TLD.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 border rounded-lg bg-destructive/5">
                <div>
                  <h4 className="font-semibold text-destructive">Delete TLD</h4>
                  <p className="text-sm text-muted-foreground mt-1">
                    This will permanently delete the TLD &quot;{tld.Name}&quot; and all associated records.
                  </p>
                </div>
                
                <Dialog open={deleteOpen} onOpenChange={(val) => { setDeleteOpen(val); if (!val) setDeleteConfirmText(''); }}>
                  <Button 
                    variant="destructive" 
                    disabled={isDeleting || hasActiveOrFuturePhases}
                    title={hasActiveOrFuturePhases ? "TLDs with active or future phases cannot be deleted." : "Delete TLD"}
                    onClick={() => setDeleteOpen(true)}
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    Delete TLD
                  </Button>
                  <DialogContent>
                    <DialogHeader>
                      <DialogTitle>Delete TLD {tld.Name}</DialogTitle>
                      <DialogDescription>
                        This will cleanly orchestrate the deletion of TLD &quot;{tld.Name}&quot; and all of its associated orphaned metadata, domains, contacts, and hosts by starting a background Temporal workflow. This action is carefully executed but cannot be undone.
                      </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-2">
                      <div className="flex items-center space-x-2 border rounded-md p-3 bg-muted/30">
                        <Checkbox 
                          id="keep-tld" 
                          checked={keepTLDAndPhases}
                          onCheckedChange={(c) => setKeepTLDAndPhases(!!c)}
                        />
                        <label htmlFor="keep-tld" className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                          Keep TLD and Phases (preserve base metadata for import retry)
                        </label>
                      </div>
                      <p className="text-sm font-medium">
                        Please type <span className="font-mono text-destructive select-all">delete {tld.Name}</span> to confirm.
                      </p>
                      <Input
                        autoFocus
                        placeholder={`delete ${tld.Name}`}
                        value={deleteConfirmText}
                        onChange={(e) => setDeleteConfirmText(e.target.value)}
                      />
                    </div>
                    <DialogFooter>
                      <Button variant="outline" onClick={() => setDeleteOpen(false)} disabled={isDeleting}>
                        Cancel
                      </Button>
                      <Button 
                        variant="destructive" 
                        onClick={handleDelete} 
                        disabled={isDeleting || deleteConfirmText !== `delete ${tld.Name}`}
                      >
                        {isDeleting ? 'Starting workflow...' : 'Confirm Deletion'}
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </DashboardLayout>
  );
}
