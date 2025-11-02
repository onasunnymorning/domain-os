"use client";

import { use, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";
import { ArrowLeft, Globe } from "lucide-react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTLD, useUpdateTLD } from "@/lib/hooks/useTLDs";

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
  const { mutateAsync, isPending } = useUpdateTLD();

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
    await mutateAsync({ name: tldName, data: values });
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
                  <Button type="submit" disabled={isPending}>Save Changes</Button>
                </div>
              </form>
            </Form>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
