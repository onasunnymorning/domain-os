"use client";

import React, { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useRouter } from "next/navigation";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage, FormDescription } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { toast } from "sonner";
import { useCreateDomain } from "@/lib/hooks/useDomains";
import { useRegistrars } from "@/lib/hooks/useRegistrars";
import { useTLDs } from "@/lib/hooks/useTLDs";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ChevronDown, ArrowLeft, PlusCircle } from "lucide-react";
import { DomainCreateRequest } from "@/lib/types/domain";
import { Checkbox } from "@/components/ui/checkbox";
import { generateAuthInfo } from "@/lib/utils/authinfo";
import posthog from "posthog-js";

const formSchema = z.object({
  Label: z
    .string()
    .min(1, "Domain label is required")
    .refine((s) => !s.includes("."), { message: "Label must not contain dots" }),
  TLD: z.string().min(1, "TLD is required"),
  ClID: z.string().min(1, "Registrar is required"),
  AuthInfo: z.string().min(1, "AuthInfo is required"),
  CreateDateLocal: z.string().min(1, "Create date is required"), // datetime-local
  ExpiryDateLocal: z.string().min(1, "Expiry date is required"), // datetime-local
  EnforcePhasePolicy: z.boolean().optional(),
  // Optional contact IDs
  RegistrantID: z.string().optional(),
  AdminID: z.string().optional(),
  TechID: z.string().optional(),
  BillingID: z.string().optional(),
});

type FormValues = z.infer<typeof formSchema>;

function toIsoFromLocal(local: string): string {
  // Convert yyyy-MM-ddTHH:mm to ISO 8601 in UTC
  // Interpret local as local time, then convert to UTC ISO string
  const d = new Date(local);
  return new Date(Date.UTC(
    d.getFullYear(),
    d.getMonth(),
    d.getDate(),
    d.getHours(),
    d.getMinutes(),
    0,
    0
  )).toISOString();
}

export default function CreateDomainPage() {
  const router = useRouter();
  const { mutateAsync, isPending } = useCreateDomain();

  // Registrar options for combobox
  const { data: registrarData } = useRegistrars({ pagesize: 200 });
  const registrarOptions = (registrarData?.Data ?? []).map((r: any) => ({
    value: r.ClID,
    label: r.Name ? `${r.Name} (${r.ClID})` : r.ClID,
  }));

  // TLD options for combobox
  const { data: tldData } = useTLDs({ pagesize: 200 });
  const tldOptions = (tldData?.Data ?? []).map((t: any) => ({
    value: t.Name,
    label: t.Name,
  }));

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Label: "",
      TLD: "",
      ClID: "",
      // Pre-generate a compliant AuthInfo to save a click
      AuthInfo: generateAuthInfo(),
      // Default dates: now for Create and +1 year for Expiry (both shown in the form)
      CreateDateLocal: new Date()
        .toISOString()
        .slice(0, 16), // yyyy-MM-ddTHH:mm
      // default to one year from now
      ExpiryDateLocal: new Date(Date.now() + 365 * 24 * 60 * 60 * 1000)
        .toISOString()
        .slice(0, 16), // yyyy-MM-ddTHH:mm
      EnforcePhasePolicy: false,
      RegistrantID: "",
      AdminID: "",
      TechID: "",
      BillingID: "",
    },
  });

  // Keep Expiry default aligned with Create date if the user hasn't manually changed Expiry.
  const createLocal = form.watch("CreateDateLocal");
  useEffect(() => {
    const expiryTouched = !!(form.formState.dirtyFields as any)?.ExpiryDateLocal;
    if (!expiryTouched && createLocal) {
      const d = new Date(createLocal);
      // Add one calendar year
      const next = new Date(
        d.getFullYear() + 1,
        d.getMonth(),
        d.getDate(),
        d.getHours(),
        d.getMinutes(),
        0,
        0
      );
      const val = next.toISOString().slice(0, 16);
      // avoid unnecessary set if equal
      if (form.getValues("ExpiryDateLocal") !== val) {
        form.setValue("ExpiryDateLocal", val, { shouldDirty: false, shouldValidate: true });
      }
    }
  }, [createLocal, form]);

  const onSubmit = async (values: FormValues) => {
    const fullName = `${values.Label.trim()}.${values.TLD.trim()}`;
    const payload: DomainCreateRequest = {
      Name: fullName,
      ClID: values.ClID,
      AuthInfo: values.AuthInfo,
      ExpiryDate: toIsoFromLocal(values.ExpiryDateLocal),
      // Send CreatedAt explicitly so backend persists our intended create time
      CreatedAt: toIsoFromLocal(values.CreateDateLocal),
      EnforcePhasePolicy: Boolean(values.EnforcePhasePolicy),
      RegistrantID: values.RegistrantID?.trim() || undefined,
      AdminID: values.AdminID?.trim() || undefined,
      TechID: values.TechID?.trim() || undefined,
      BillingID: values.BillingID?.trim() || undefined,
    };

    try {
      const created = await mutateAsync(payload);
      toast.success("Domain created successfully");
      const name = (created as any)?.Name || fullName;
      posthog.capture('domain_created', {
        domain_name: name,
        tld: values.TLD,
        registrar_clid: values.ClID,
        enforce_phase_policy: values.EnforcePhasePolicy,
      });
      router.push(`/domains/${encodeURIComponent(name)}`);
    } catch (error: any) {
      posthog.captureException(error);
      toast.error(error?.response?.data?.error || "Failed to create domain");
    }
  };

  // Combobox UI state
  const [openReg, setOpenReg] = useState(false);
  const [regSearch, setRegSearch] = useState("");
  const filteredRegs = registrarOptions.filter((o) =>
    o.label.toLowerCase().includes(regSearch.toLowerCase())
  );

  const [openTld, setOpenTld] = useState(false);
  const [tldSearch, setTldSearch] = useState("");
  const filteredTlds = tldOptions.filter((o) =>
    o.label.toLowerCase().includes(tldSearch.toLowerCase())
  );

  return (
    <DashboardLayout>
      <div className="max-w-3xl space-y-6">
        {/* Header */}
        <div className="flex items-center gap-4">
          <Link href="/domains">
            <Button variant="ghost" size="sm">
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back
            </Button>
          </Link>
        </div>

        <div>
          <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
            <PlusCircle className="h-7 w-7" />
            Create Domain
          </h1>
          <p className="text-muted-foreground">Create a domain as an admin with full control.</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Required</CardTitle>
            <CardDescription>These fields are required by the API</CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <FormField
                    control={form.control}
                    name="Label"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Domain Label *</FormLabel>
                        <FormControl>
                          <Input placeholder="example" {...field} />
                        </FormControl>
                        <FormDescription>The label part only, without the TLD</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="CreateDateLocal"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Create Date *</FormLabel>
                        <FormControl>
                          <Input type="datetime-local" {...field} />
                        </FormControl>
                        <FormDescription>
                          Local time; will be stored in UTC. Expiry defaults to Create Date + 1 year.
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="TLD"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>TLD *</FormLabel>
                        <Popover open={openTld} onOpenChange={setOpenTld}>
                          <PopoverTrigger asChild>
                            <Button variant="outline" role="combobox" className="w-full justify-between">
                              {field.value || "Select TLD"}
                              <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent className="w-80 p-2" align="start">
                            <Input
                              placeholder="Search TLD..."
                              value={tldSearch}
                              onChange={(e) => setTldSearch(e.target.value)}
                              className="mb-2"
                            />
                            <ScrollArea className="h-52">
                              <div className="space-y-1">
                                {filteredTlds.map((opt) => (
                                  <Button
                                    key={opt.value}
                                    type="button"
                                    variant={opt.value === field.value ? "secondary" : "ghost"}
                                    className="w-full justify-start"
                                    onClick={() => {
                                      field.onChange(opt.value);
                                      setOpenTld(false);
                                    }}
                                  >
                                    {opt.label}
                                  </Button>
                                ))}
                                {filteredTlds.length === 0 && (
                                  <div className="text-xs text-muted-foreground px-2 py-1">No results</div>
                                )}
                              </div>
                            </ScrollArea>
                          </PopoverContent>
                        </Popover>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="ClID"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Registrar (ClID) *</FormLabel>
                        <Popover open={openReg} onOpenChange={setOpenReg}>
                          <PopoverTrigger asChild>
                            <Button variant="outline" role="combobox" className="w-full justify-between">
                              {field.value
                                ? registrarOptions.find((o) => o.value === field.value)?.label
                                : "Select registrar"}
                              <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent className="w-80 p-2" align="start">
                            <Input
                              placeholder="Search registrar..."
                              value={regSearch}
                              onChange={(e) => setRegSearch(e.target.value)}
                              className="mb-2"
                            />
                            <ScrollArea className="h-52">
                              <div className="space-y-1">
                                {filteredRegs.map((opt) => (
                                  <Button
                                    key={opt.value}
                                    type="button"
                                    variant={opt.value === field.value ? "secondary" : "ghost"}
                                    className="w-full justify-start"
                                    onClick={() => {
                                      field.onChange(opt.value);
                                      setOpenReg(false);
                                    }}
                                  >
                                    {opt.label}
                                  </Button>
                                ))}
                                {filteredRegs.length === 0 && (
                                  <div className="text-xs text-muted-foreground px-2 py-1">No results</div>
                                )}
                              </div>
                            </ScrollArea>
                          </PopoverContent>
                        </Popover>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="AuthInfo"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>AuthInfo *</FormLabel>
                        <div className="flex gap-2">
                          <FormControl>
                            <Input placeholder="Secret transfer code" {...field} />
                          </FormControl>
                          <Button
                            type="button"
                            variant="secondary"
                            aria-label="Generate AuthInfo"
                            onClick={() => {
                              const v = generateAuthInfo();
                              form.setValue('AuthInfo', v, { shouldDirty: true, shouldValidate: true });
                            }}
                          >
                            Generate
                          </Button>
                        </div>
                        <FormDescription>
                          A strong code with upper/lowercase, a digit, and a special character. Pre-filled to save a click.
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="ExpiryDateLocal"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Expiry Date *</FormLabel>
                        <FormControl>
                          <Input type="datetime-local" {...field} />
                        </FormControl>
                        <FormDescription>
                          Defaults to Create Date + 1 year. Local time; will be stored in UTC.
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>

                {/* Optional contacts */}
                <Card>
                  <CardHeader>
                    <CardTitle>Contacts (optional)</CardTitle>
                    <CardDescription>Provide existing contact IDs if applicable</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                      <FormField
                        control={form.control}
                        name="RegistrantID"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>RegistrantID</FormLabel>
                            <FormControl>
                              <Input placeholder="contact-1" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="AdminID"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>AdminID</FormLabel>
                            <FormControl>
                              <Input placeholder="contact-2" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="TechID"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>TechID</FormLabel>
                            <FormControl>
                              <Input placeholder="contact-3" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="BillingID"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>BillingID</FormLabel>
                            <FormControl>
                              <Input placeholder="contact-4" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                  </CardContent>
                </Card>

                {/* Options */}
                <div className="space-y-2">
                  <FormField
                    control={form.control}
                    name="EnforcePhasePolicy"
                    render={({ field }) => (
                      <FormItem>
                        <div className="flex items-center gap-2">
                          <Checkbox id="enforce" checked={field.value ?? false} onCheckedChange={(v)=>field.onChange(Boolean(v))} />
                          <FormLabel htmlFor="enforce">Enforce current GA phase policy</FormLabel>
                        </div>
                        <FormDescription>
                          If enabled, contact IDs will be validated per the TLD's active GA phase policy.
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>

                <div className="flex items-center gap-2">
                  <Button type="submit" disabled={isPending}>Create Domain</Button>
                  <Button type="button" variant="outline" onClick={() => router.push('/domains')}>Cancel</Button>
                </div>
              </form>
            </Form>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
