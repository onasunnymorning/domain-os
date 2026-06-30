"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { useParams, useRouter } from "next/navigation";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { useRegistrar, useUpdateRegistrar } from "@/lib/hooks/useRegistrars";
import { RegistrarStatus, IANARegistrarStatus } from "@/lib/types/registrar";
import posthog from "posthog-js";

const formSchema = z.object({
  Name: z.string().min(1, "Name is required").optional(),
  NickName: z.string().optional(),
  Email: z.string().email("Invalid email address").optional().or(z.literal("")),
  GurIDString: z.string().regex(/^\d+$/, "GurID must be numeric").optional().or(z.literal("")),
  URL: z.string().url("Invalid URL").optional().or(z.literal("")),
  Voice: z.string().optional(),
  Fax: z.string().optional(),
  RdapBaseURL: z.string().url("Invalid URL").optional().or(z.literal("")),
  WhoisName: z.string().optional(),
  WhoisURL: z.string().url("Invalid URL").optional().or(z.literal("")),
  PostalType: z.enum(["loc", "int"]).optional(),
  Street1: z.string().optional(),
  Street2: z.string().optional(),
  Street3: z.string().optional(),
  City: z.string().optional(),
  SP: z.string().optional(),
  PC: z.string().optional(),
  CC: z.string().max(2, "Use 2-letter country code").optional().or(z.literal("")),
  Status: z.enum(["ok", "readonly", "terminated"]).optional(),
  IANAStatus: z.enum(["Accredited", "Reserved", "Terminated", "Unknown"]).optional(),
  Autorenew: z.boolean().optional(),
});

type FormValues = z.infer<typeof formSchema>;

export default function EditRegistrarPage() {
  const params = useParams();
  const router = useRouter();
  const clid = typeof params?.clid === "string" ? params.clid : Array.isArray(params?.clid) ? params?.clid[0] : "";
  const { data: reg, isLoading, error } = useRegistrar(clid, !!clid);
  const { mutateAsync, isPending } = useUpdateRegistrar();

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Name: "",
      NickName: "",
      Email: "",
      GurIDString: "",
      URL: "",
      Voice: "",
      Fax: "",
      RdapBaseURL: "",
      WhoisName: "",
      WhoisURL: "",
      PostalType: "int",
      Street1: "",
      Street2: "",
      Street3: "",
      City: "",
      SP: "",
      PC: "",
      CC: "",
      Status: undefined,
      IANAStatus: undefined,
      Autorenew: undefined,
    },
  });

  // Prefill when registrar loads
  useEffect(() => {
    if (!reg) return;
  const normalizedStatus = typeof (reg as any).Status === "string" ? (reg as any).Status.toLowerCase().trim() : undefined;
    const allowedStatuses = new Set(["ok", "readonly", "terminated"]);
    const statusValue = allowedStatuses.has(normalizedStatus as string) ? (normalizedStatus as any) : undefined;

    const allowedIANA = new Set(["Accredited", "Reserved", "Terminated", "Unknown"]);
    const ianaRaw = (reg as any).IANAStatus;
    const ianaValue = typeof ianaRaw === "string" && allowedIANA.has(ianaRaw) ? (ianaRaw as any) : undefined;
    form.reset({
      Name: reg.Name || "",
      NickName: (reg as any).NickName || reg.Name || "",
      Email: reg.Email || "",
      GurIDString: typeof reg.GurID === "number" && reg.GurID > 0 ? String(reg.GurID) : "",
      URL: reg.URL || "",
      Voice: reg.Voice || "",
      Fax: reg.Fax || "",
      RdapBaseURL: reg.RdapBaseURL || "",
      WhoisName: (reg as any).WhoisInfo?.Name || "",
      WhoisURL: (reg as any).WhoisInfo?.URL || "",
      PostalType: ((reg as any).PostalInfo?.[0]?.Type as "loc" | "int") || "int",
      Street1: (reg as any).PostalInfo?.[0]?.Address?.Street1 || "",
      Street2: (reg as any).PostalInfo?.[0]?.Address?.Street2 || "",
      Street3: (reg as any).PostalInfo?.[0]?.Address?.Street3 || "",
      City: (reg as any).PostalInfo?.[0]?.Address?.City || "",
      SP: (reg as any).PostalInfo?.[0]?.Address?.SP || "",
      PC: (reg as any).PostalInfo?.[0]?.Address?.PC || "",
      CC: (reg as any).PostalInfo?.[0]?.Address?.CC || "",
      Status: statusValue,
      IANAStatus: ianaValue,
      Autorenew: (reg as any).Autorenew ?? undefined,
    });
  }, [reg, form]);

  const onSubmit = async (values: FormValues) => {
    const payload: any = {};

    if (values.Name) payload.Name = values.Name;
    if (values.NickName) payload.NickName = values.NickName;
    if (values.Email) payload.Email = values.Email;
    if (values.GurIDString && values.GurIDString.trim().length > 0) payload.GurID = Number(values.GurIDString);
    if (values.URL) payload.URL = values.URL;
    if (values.Voice) payload.Voice = values.Voice;
    if (values.Fax) payload.Fax = values.Fax;
    if (values.RdapBaseURL) payload.RdapBaseURL = values.RdapBaseURL;
    if (values.WhoisName || values.WhoisURL) {
      payload.WhoisInfo = {
        Name: values.WhoisName || undefined,
        URL: values.WhoisURL || undefined,
      };
    }
    // Status: backend may require presence; send current if unchanged
    const currentStatus = typeof (reg as any)?.Status === "string" ? String((reg as any).Status).trim().toLowerCase() : undefined;
    if (values.Status) {
      const status = String(values.Status).trim().toLowerCase();
      if (status === "ok" || status === "readonly" || status === "terminated") {
        payload.Status = status;
      }
    } else if (currentStatus && (currentStatus === "ok" || currentStatus === "readonly" || currentStatus === "terminated")) {
      payload.Status = currentStatus;
    }
    if (values.IANAStatus) payload.IANAStatus = values.IANAStatus;

    // Autorenew: include value; default to current if not provided
    if (typeof values.Autorenew === "boolean") {
      payload.Autorenew = values.Autorenew;
    } else if (typeof (reg as any)?.Autorenew === "boolean") {
      payload.Autorenew = (reg as any).Autorenew;
    }

    // Postal Info: include if at least CC+City or any address provided
    const hasAddress =
      values.City || values.CC || values.Street1 || values.Street2 || values.Street3 || values.SP || values.PC;
    if (hasAddress) {
      payload.PostalInfo = [
        {
          Type: values.PostalType || "int",
          Address: {
            Street1: values.Street1 || undefined,
            Street2: values.Street2 || undefined,
            Street3: values.Street3 || undefined,
            City: values.City || undefined,
            SP: values.SP || undefined,
            PC: values.PC || undefined,
            CC: values.CC || undefined,
          },
        },
      ];
    }

    try {
      await mutateAsync({ clid, data: payload });
      toast.success("Registrar updated successfully");
      posthog.capture('registrar_updated', {
        clid,
        status: payload.Status,
        iana_status: payload.IANAStatus,
      });
      router.push(`/registrars/${encodeURIComponent(clid)}`);
    } catch (error: any) {
      posthog.captureException(error);
      toast.error(error?.response?.data?.error || "Failed to update registrar");
    }
  };

  return (
    <DashboardLayout>
      <div className="max-w-3xl space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold">Edit Registrar</h1>
            <p className="text-sm text-muted-foreground mt-1">{clid}</p>
          </div>
          <Button variant="outline" onClick={() => router.push(`/registrars/${encodeURIComponent(clid)}`)}>Cancel</Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Registrar Information</CardTitle>
            <CardDescription>Update registrar details. Leave fields blank to keep current values.</CardDescription>
          </CardHeader>
          <CardContent>
            {error ? (
              <div className="text-red-600">Failed to load registrar.</div>
            ) : (
              <Form {...form}>
                <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <FormField control={form.control} name="Name" render={({ field }) => (
                      <FormItem>
                        <FormLabel>Name</FormLabel>
                        <FormControl><Input placeholder="Example Registrar, Inc." {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="NickName" render={({ field }) => (
                      <FormItem>
                        <FormLabel>Nickname</FormLabel>
                        <FormControl><Input placeholder="Brand or short name" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="GurIDString" render={({ field }) => (
                      <FormItem>
                        <FormLabel>IANA Registrar ID (GurID)</FormLabel>
                        <FormControl><Input placeholder="e.g. 468" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="Email" render={({ field }) => (
                      <FormItem>
                        <FormLabel>Email</FormLabel>
                        <FormControl><Input type="email" placeholder="contact@registrar.com" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="URL" render={({ field }) => (
                      <FormItem>
                        <FormLabel>Website URL</FormLabel>
                        <FormControl><Input placeholder="https://registrar.example" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="Voice" render={({ field }) => (
                      <FormItem>
                        <FormLabel>Phone</FormLabel>
                        <FormControl><Input placeholder="+1.1234567890" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="Fax" render={({ field }) => (
                      <FormItem>
                        <FormLabel>Fax</FormLabel>
                        <FormControl><Input placeholder="+1.1234567890" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="RdapBaseURL" render={({ field }) => (
                      <FormItem>
                        <FormLabel>RDAP Base URL</FormLabel>
                        <FormControl><Input placeholder="https://rdap.registrar.example" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="Status" render={({ field }) => (
                      <FormItem>
                        <FormLabel>Status</FormLabel>
                        <Select onValueChange={(v) => field.onChange(v)} value={field.value ?? undefined}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Unchanged" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value={RegistrarStatus.OK}>ok</SelectItem>
                            <SelectItem value={RegistrarStatus.Readonly}>readonly</SelectItem>
                            <SelectItem value={RegistrarStatus.Terminated}>terminated</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>Leave blank to keep the current status.</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <FormField control={form.control} name="IANAStatus" render={({ field }) => (
                      <FormItem>
                        <FormLabel>IANA Status</FormLabel>
                        <Select onValueChange={(v) => field.onChange(v)} value={field.value ?? undefined}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Unchanged" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value={IANARegistrarStatus.Accredited}>Accredited</SelectItem>
                            <SelectItem value={IANARegistrarStatus.Reserved}>Reserved</SelectItem>
                            <SelectItem value={IANARegistrarStatus.Terminated}>Terminated</SelectItem>
                            <SelectItem value={IANARegistrarStatus.Unknown}>Unknown</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>Leave blank to keep the current IANA status.</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )} />
                  </div>

                  <div className="space-y-4">
                    <div>
                      <h3 className="text-lg font-semibold">Postal Information</h3>
                      <p className="text-sm text-muted-foreground">Update address fields as needed; omit to leave unchanged.</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                      <FormField control={form.control} name="PostalType" render={({ field }) => (
                        <FormItem>
                          <FormLabel>Postal Type</FormLabel>
                          <Select onValueChange={field.onChange} value={field.value ?? "int"}>
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="Select type" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="loc">Local (loc)</SelectItem>
                              <SelectItem value="int">International (int)</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <FormField control={form.control} name="CC" render={({ field }) => (
                        <FormItem>
                          <FormLabel>Country Code (CC)</FormLabel>
                          <FormControl><Input placeholder="US" maxLength={2} {...field} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <FormField control={form.control} name="City" render={({ field }) => (
                        <FormItem>
                          <FormLabel>City</FormLabel>
                          <FormControl><Input placeholder="Austin" {...field} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <FormField control={form.control} name="SP" render={({ field }) => (
                        <FormItem>
                          <FormLabel>State/Province (SP)</FormLabel>
                          <FormControl><Input placeholder="TX" {...field} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <FormField control={form.control} name="PC" render={({ field }) => (
                        <FormItem>
                          <FormLabel>Postal Code (PC)</FormLabel>
                          <FormControl><Input placeholder="73301" {...field} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <FormField control={form.control} name="Street1" render={({ field }) => (
                        <FormItem>
                          <FormLabel>Street 1</FormLabel>
                          <FormControl><Input placeholder="1234 Main St" {...field} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <FormField control={form.control} name="Street2" render={({ field }) => (
                        <FormItem>
                          <FormLabel>Street 2</FormLabel>
                          <FormControl><Input placeholder="Suite 123" {...field} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <FormField control={form.control} name="Street3" render={({ field }) => (
                        <FormItem>
                          <FormLabel>Street 3</FormLabel>
                          <FormControl><Input placeholder="Attn: Registration" {...field} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />
                    </div>
                  </div>

                  {/* Settings row */}
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <FormField control={form.control} name="Autorenew" render={({ field }) => (
                      <FormItem className="flex flex-col justify-end">
                        <FormLabel>Autorenew</FormLabel>
                        <div className="flex items-center gap-3 py-2">
                          <Switch checked={!!field.value} onCheckedChange={field.onChange} />
                          <span className="text-sm text-muted-foreground">
                            When enabled, domains under this registrar will attempt to renew automatically. Success depends on each domain’s TLD configuration.
                          </span>
                        </div>
                        <FormMessage />
                      </FormItem>
                    )} />
                  </div>

                  <div className="flex gap-4">
                    <Button
                      type="button"
                      onClick={() => onSubmit(form.getValues())}
                      disabled={isPending}
                    >
                      {isPending ? "Saving..." : "Save changes"}
                    </Button>
                    <Button type="button" variant="outline" onClick={() => router.push(`/registrars/${encodeURIComponent(clid)}`)} disabled={isPending}>Cancel</Button>
                  </div>
                </form>
              </Form>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
