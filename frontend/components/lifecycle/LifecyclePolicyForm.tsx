"use client";

import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  LIFECYCLE_DURATIONS,
  LIFECYCLE_POLICY_NAME_PATTERN,
  MAX_HORIZON_BOUNDS,
  STANDARD_LIFECYCLE,
  resolveLifecycle,
} from "@/lib/constants/lifecycleDefaults";
import type { LifecyclePolicyUse, LifecycleSettings } from "@/lib/types/lifecycle";

/** A duration field: empty is "the standard value" (null), otherwise 0 to its bound. */
const duration = (label: string, max: number) =>
  z
    .number({ message: `${label} must be a number` })
    .int(`${label} must be a whole number of days`)
    .min(0, `${label} must be 0 to ${max}`)
    .max(max, `${label} must be 0 to ${max}`)
    .nullable();

const settingsSchema = z.object({
  registrationGP: duration("Add grace period", 365),
  renewalGP: duration("Renew grace period", 365),
  autorenewalGP: duration("Auto-renew grace period", 365),
  transferGP: duration("Transfer grace period", 365),
  transferLockPeriod: duration("Transfer lock", 365),
  pendingTransferPeriod: duration("Pending transfer", 30),
  redemptionGP: duration("Redemption period", 365),
  pendingdeleteGP: duration("Pending delete", 90),
  pendingRestorePeriod: duration("Restore report due", 30),
  restoreReportRequired: z.boolean().nullable(),
  allowAutorenew: z.boolean().nullable(),
  maxHorizon: z
    .number({ message: "The horizon must be a number" })
    .int("The horizon must be whole years")
    .min(MAX_HORIZON_BOUNDS.min, `The horizon is ${MAX_HORIZON_BOUNDS.min} to ${MAX_HORIZON_BOUNDS.max} years`)
    .max(MAX_HORIZON_BOUNDS.max, `The horizon is ${MAX_HORIZON_BOUNDS.min} to ${MAX_HORIZON_BOUNDS.max} years`)
    .nullable(),
});

export const lifecyclePolicySchema = z
  .object({
    name: z.string().regex(LIFECYCLE_POLICY_NAME_PATTERN, "1 to 32 lower-case letters, digits and hyphens, starting with a letter"),
    description: z.string().optional().or(z.literal("")),
    settings: settingsSchema,
  })
  // A report cannot be sent within a zero-day window (the API refuses it too).
  .refine((v) => !(resolveLifecycle(v.settings).restoreReportRequired && resolveLifecycle(v.settings).pendingRestorePeriod === 0), {
    message: "A restore report needs a window to be sent in: at least 1 day, or turn the report off",
    path: ["settings", "pendingRestorePeriod"],
  });

export type LifecyclePolicyFormValues = z.infer<typeof lifecyclePolicySchema>;

export interface LifecyclePolicyFormProps {
  mode: "create" | "edit";
  initial: { name: string; description?: string; settings: LifecycleSettings };
  /** The phases that name the policy: what a save reaches. */
  usedBy?: LifecyclePolicyUse[];
  builtIn?: boolean;
  saving?: boolean;
  onSave: (values: { name: string; description: string; settings: LifecycleSettings }) => Promise<void>;
  onDelete?: () => void;
}

/** An empty field is null (the standard value); anything else is parsed as a number. */
function parseOptionalInt(raw: string): number | null {
  if (raw.trim() === "") return null;
  const n = Number(raw);
  return Number.isNaN(n) ? NaN : n;
}

export function LifecyclePolicyForm({ mode, initial, usedBy = [], builtIn = false, saving = false, onSave, onDelete }: LifecyclePolicyFormProps) {
  const form = useForm<LifecyclePolicyFormValues>({
    resolver: zodResolver(lifecyclePolicySchema),
    defaultValues: { name: initial.name, description: initial.description ?? "", settings: initial.settings },
    mode: "onSubmit",
  });
  const settings = form.watch("settings");
  const resolved = resolveLifecycle(settings);
  const tlds = new Set(usedBy.map((u) => u.tld));

  const submit = form.handleSubmit((v) => onSave({ name: v.name, description: v.description ?? "", settings: v.settings }));

  return (
    <Form {...form}>
      {/* noValidate: the bounds are zod's, with a message on the field; the browser's own tooltip would pre-empt it. */}
      <form onSubmit={submit} className="space-y-6" noValidate>
        {usedBy.length > 0 && (
          <div className="flex items-start gap-3 rounded-md border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200" role="status">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <div className="space-y-1">
              <div className="font-medium">
                A save reaches {usedBy.length} phase{usedBy.length === 1 ? "" : "s"} across {tlds.size} TLD{tlds.size === 1 ? "" : "s"} at once.
              </div>
              <div className="flex flex-wrap gap-x-3 gap-y-1 font-mono text-xs">
                {usedBy.map((u) => (
                  <Link key={`${u.tld}/${u.phase}`} href={`/tlds/${encodeURIComponent(u.tld)}`} className="hover:underline">
                    {u.tld}/{u.phase}
                  </Link>
                ))}
              </div>
              <div className="text-xs">
                A TLD that should not follow the change needs a policy of its own, named on its phase, before this one is saved.
              </div>
            </div>
          </div>
        )}

        <Card>
          <CardHeader>
            <CardTitle>Policy</CardTitle>
            <CardDescription className="max-w-prose">A name phases refer to, and a line on what it is for.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Name</FormLabel>
                  <FormControl>
                    <Input {...field} disabled={mode === "edit"} className="font-mono" placeholder="short-redemption" />
                  </FormControl>
                  <FormDescription>Lower-case letters, digits and hyphens.</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Description</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder="What it is for" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Durations</CardTitle>
            <CardDescription className="max-w-prose">
              Whole days. Leave a field empty to follow the standard value shown beside it; enter 0 for zero days (no grace period, no lock).
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {LIFECYCLE_DURATIONS.map((d) => (
              <FormField
                key={d.key}
                control={form.control}
                name={`settings.${d.key}`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel htmlFor={`settings.${d.key}`}>{d.label}</FormLabel>
                    <FormControl>
                      <Input
                        id={`settings.${d.key}`}
                        type="number"
                        inputMode="numeric"
                        placeholder={`${STANDARD_LIFECYCLE[d.key]} (standard)`}
                        value={field.value ?? ""}
                        onChange={(e) => field.onChange(parseOptionalInt(e.target.value))}
                        onBlur={field.onBlur}
                        name={field.name}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {d.help} In force: <span className="font-mono">{resolved[d.key]}d</span>.
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Switches and horizon</CardTitle>
            <CardDescription className="max-w-prose">Whether a restore waits for the report, whether the TLD auto-renews, and how far ahead an expiry may reach.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-6 sm:grid-cols-3">
            <FormField
              control={form.control}
              name="settings.restoreReportRequired"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel htmlFor="settings.restoreReportRequired">Restore report required</FormLabel>
                  <div className="flex items-center gap-2">
                    <FormControl>
                      <Switch id="settings.restoreReportRequired" checked={field.value ?? STANDARD_LIFECYCLE.restoreReportRequired} onCheckedChange={(v) => field.onChange(v)} aria-label="Restore report required" />
                    </FormControl>
                    <span className="text-sm">{(field.value ?? STANDARD_LIFECYCLE.restoreReportRequired) ? "Required" : "Not required"}</span>
                  </div>
                  <FormDescription>Off, a restore completes on the request alone.</FormDescription>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="settings.allowAutorenew"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel htmlFor="settings.allowAutorenew">Auto-renew allowed</FormLabel>
                  <div className="flex items-center gap-2">
                    <FormControl>
                      <Switch id="settings.allowAutorenew" checked={field.value ?? STANDARD_LIFECYCLE.allowAutorenew} onCheckedChange={(v) => field.onChange(v)} aria-label="Auto-renew allowed" />
                    </FormControl>
                    <span className="text-sm">{(field.value ?? STANDARD_LIFECYCLE.allowAutorenew) ? "Allowed" : "Not allowed"}</span>
                  </div>
                  <FormDescription>Off, every expired domain goes to pending delete, whatever its registrar opted into.</FormDescription>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="settings.maxHorizon"
              render={({ field }) => (
                <FormItem>
                  <FormLabel htmlFor="settings.maxHorizon">Max horizon (years)</FormLabel>
                  <FormControl>
                    <Input
                      id="settings.maxHorizon"
                      type="number"
                      inputMode="numeric"
                      placeholder={`${STANDARD_LIFECYCLE.maxHorizon} (standard)`}
                      value={field.value ?? ""}
                      onChange={(e) => field.onChange(parseOptionalInt(e.target.value))}
                      onBlur={field.onBlur}
                      name={field.name}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    The furthest expiry a registration, renewal or transfer may reach. In force: <span className="font-mono">{resolved.maxHorizon}y</span>.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardContent>
        </Card>

        <div className="flex items-center justify-between gap-3">
          <div>
            {mode === "edit" && onDelete && !builtIn && (
              <Button type="button" variant="outline" onClick={onDelete} disabled={saving}>
                Delete
              </Button>
            )}
          </div>
          <Button type="submit" disabled={saving}>
            {saving ? "Saving…" : mode === "create" ? "Create policy" : "Save"}
          </Button>
        </div>
      </form>
    </Form>
  );
}
