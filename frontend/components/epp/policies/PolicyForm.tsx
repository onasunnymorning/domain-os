"use client";

import { useEffect, useState } from "react";
import { useForm, type Path, type UseFormReturn } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { ChevronDown, ChevronRight, Info } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { POLICY_NAME_PATTERN, RATE_CLASSES, tripleRates } from "@/lib/constants/eppPolicyDefaults";
import type { EPPPolicyLimits } from "@/lib/types/epp";

const limit = (what: string) =>
  z.number({ message: `${what} must be a number` }).int(`${what} must be a whole number`).min(1, `${what} must be at least 1`);
const rate = (cls: string) =>
  z.object({
    per_second: limit(`${cls} per second`),
    burst: limit(`${cls} burst`),
  });
const rates = () => z.object({ check: rate("Check"), info: rate("Info"), transform: rate("Transform"), poll: rate("Poll") });
const budget = (who: string) =>
  z.object({
    failures: limit(`${who} failures`),
    ratio_percent: z
      .number({ message: "The ratio must be a number" })
      .int("The ratio must be a whole number")
      .min(1, "The ratio is 1 to 100")
      .max(100, "The ratio is 1 to 100"),
  });

export const policySchema = z
  .object({
    name: z.string().regex(POLICY_NAME_PATTERN, "1 to 32 lower-case letters, digits and hyphens, starting with a letter"),
    description: z.string().optional().or(z.literal("")),
    limits: z.object({
      sessions_per_registrar: limit("Sessions per registrar"),
      sessions_per_login: limit("Sessions per login"),
      sessions_per_family: limit("Sessions per family"),
      check_names: limit("Names per check"),
      rates: rates(),
      family_rates: rates(),
      creates_per_day: z.number({ message: "Creates per day must be a number" }).int("Creates per day must be a whole number").min(0, "0 means no cap"),
      failed_creates: budget("Registrar"),
      family_failed_creates: budget("Family"),
    }),
  })
  // A full check could never pass if the bucket cannot hold it.
  .refine((v) => v.limits.rates.check.burst >= v.limits.check_names, {
    message: "The check burst must be at least the names per check, or a full check could never pass",
    path: ["limits", "rates", "check", "burst"],
  })
  .refine((v) => v.limits.family_rates.check.burst >= v.limits.check_names, {
    message: "The family check burst must be at least the names per check",
    path: ["limits", "family_rates", "check", "burst"],
  });

export type PolicyFormValues = z.infer<typeof policySchema>;

export interface PolicyFormProps {
  /** Existing policy: name is locked. New: name is editable. */
  mode: "create" | "edit";
  initial: { name: string; description?: string; limits: EPPPolicyLimits };
  builtIn?: boolean;
  /** The built-in starting values, when this is a built-in policy. */
  resetValues?: EPPPolicyLimits;
  saving?: boolean;
  onSave: (values: { name: string; description: string; limits: EPPPolicyLimits }) => Promise<void>;
  onDelete?: () => void;
}

function Section({ title, description, children }: { title: string; description: string; children: React.ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription className="max-w-prose">{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">{children}</CardContent>
    </Card>
  );
}

function NumberField({
  form,
  name,
  label,
  description,
  min = 1,
}: {
  form: UseFormReturn<PolicyFormValues>;
  name: Path<PolicyFormValues>;
  label: string;
  description: string;
  min?: number;
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input
              type="number"
              min={min}
              className="max-w-[140px]"
              name={field.name}
              ref={field.ref}
              onBlur={field.onBlur}
              value={Number.isNaN(field.value as number) ? "" : (field.value as number)}
              onChange={(e) => field.onChange(e.target.value === "" ? NaN : Number(e.target.value))}
            />
          </FormControl>
          <FormDescription>{description}</FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  );
}

function RatesGrid({
  form,
  base,
}: {
  form: UseFormReturn<PolicyFormValues>;
  base: "limits.rates" | "limits.family_rates";
}) {
  return (
    <TooltipProvider delayDuration={150}>
      <div className="grid grid-cols-[minmax(110px,1fr)_140px_140px] items-start gap-x-4 gap-y-3">
        <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Class</div>
        <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Per second</div>
        <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Burst</div>
        {RATE_CLASSES.map((c) => (
          <div key={c.key} className="contents">
            <div className="flex items-center gap-1 pt-2 text-sm font-medium">
              {c.label}
              <Tooltip>
                <TooltipTrigger asChild>
                  <button type="button" aria-label={`What ${c.label} meters`} className="text-muted-foreground">
                    <Info className="h-3.5 w-3.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent className="max-w-xs">
                  <p>{c.commands}</p>
                  {c.note && <p className="mt-1 font-medium">{c.note}</p>}
                </TooltipContent>
              </Tooltip>
            </div>
            {(["per_second", "burst"] as const).map((k) => (
              <FormField
                key={k}
                control={form.control}
                name={`${base}.${c.key}.${k}` as Path<PolicyFormValues>}
                render={({ field }) => (
                  <FormItem>
                    <FormControl>
                      <Input
                        type="number"
                        min={1}
                        aria-label={`${c.label} ${k === "per_second" ? "per second" : "burst"}`}
                        name={field.name}
                        ref={field.ref}
                        onBlur={field.onBlur}
                        value={Number.isNaN(field.value as number) ? "" : (field.value as number)}
                        onChange={(e) => field.onChange(e.target.value === "" ? NaN : Number(e.target.value))}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
          </div>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">
        A bucket refills at <em>per second</em> tokens a second and holds at most <em>burst</em>, so a quiet registrar
        can send a burst at once and then its rate. Over the rate a command waits up to a second, then is refused.
      </p>
    </TooltipProvider>
  );
}

/**
 * One form for a policy, built-in or custom. Every field says its rule and
 * its default in words; the cross-field rule (check burst at least the names
 * per check) is shown on the burst field.
 */
export function PolicyForm({ mode, initial, builtIn = false, resetValues, saving = false, onSave, onDelete }: PolicyFormProps) {
  const form = useForm<PolicyFormValues>({
    resolver: zodResolver(policySchema),
    defaultValues: { name: initial.name, description: initial.description ?? "", limits: initial.limits },
  });
  const [limitCreates, setLimitCreates] = useState(initial.limits.creates_per_day > 0);
  const [showFamily, setShowFamily] = useState(false);

  // Family rates are collapsed by default; an error there must be seen.
  const familyErrors = form.formState.errors.limits?.family_rates;
  useEffect(() => {
    if (familyErrors) setShowFamily(true);
  }, [familyErrors]);

  const submit = form.handleSubmit(
    async (v) => {
      const limits: EPPPolicyLimits = { ...v.limits, creates_per_day: limitCreates ? v.limits.creates_per_day : 0 };
      await onSave({ name: v.name, description: v.description ?? "", limits });
    },
    (errors) => {
      if (errors.limits?.family_rates) setShowFamily(true);
    },
  );

  // Off means 0, whatever the hidden field held (a cleared field is NaN).
  const setLimitCreatesTo = (on: boolean) => {
    setLimitCreates(on);
    if (!on) form.setValue("limits.creates_per_day", 0, { shouldValidate: true });
  };

  const setFamilyToTriple = () => {
    const r = form.getValues("limits.rates");
    form.setValue("limits.family_rates", tripleRates(r), { shouldDirty: true });
    setShowFamily(true);
  };

  const resetToBuiltIn = () => {
    if (!resetValues) return;
    form.setValue("limits", resetValues, { shouldDirty: true });
    setLimitCreates(resetValues.creates_per_day > 0);
  };

  return (
    <Form {...form}>
      <form onSubmit={submit} className="space-y-6">
        <Section
          title="Name and description"
          description={
            mode === "create"
              ? "The name is how registrars are assigned to this policy and cannot change later."
              : builtIn
                ? "A built-in policy: its limits can change, it cannot be deleted or renamed."
                : "The name cannot change; the description is for operators."
          }
        >
          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Name</FormLabel>
                <FormControl>
                  <Input {...field} className="max-w-xs font-mono" disabled={mode === "edit"} placeholder="gold" />
                </FormControl>
                <FormDescription>1 to 32 lower-case letters, digits and hyphens, starting with a letter.</FormDescription>
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
                  <Input {...field} placeholder="Large registrars with their own client systems" />
                </FormControl>
                <FormDescription>One line, shown wherever the policy is named.</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </Section>

        <Section
          title="Sessions"
          description="How many sessions may be open at once. A login can have its own cap on the registrar's EPP tab, which replaces the per-login number for that login."
        >
          <div className="grid gap-4 sm:grid-cols-3">
            <NumberField form={form} name="limits.sessions_per_registrar" label="Per registrar" description="Across every login and node. Default 10." />
            <NumberField form={form} name="limits.sessions_per_login" label="Per login" description="One login's sessions. Default 10." />
            <NumberField form={form} name="limits.sessions_per_family" label="Per family" description="Used only when this policy is a family's. Default 40." />
          </div>
        </Section>

        <Section title="Checks" description="How many names one check command may carry. The check burst below must be at least this, or a full check could never pass.">
          <NumberField form={form} name="limits.check_names" label="Names per check" description="Default 50. Over it the check is refused without a lookup." />
        </Section>

        <Section title="Command rates" description="How fast a registrar may send commands, by class. Check is counted per name, so batch size does not change capacity.">
          <RatesGrid form={form} base="limits.rates" />
        </Section>

        <Card>
          <CardHeader>
            <button
              type="button"
              className="flex w-full items-center gap-2 text-left"
              onClick={() => setShowFamily((s) => !s)}
              aria-expanded={showFamily}
            >
              {showFamily ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              <CardTitle>Family rates</CardTitle>
            </button>
            <CardDescription className="max-w-prose">
              Used only when this policy is assigned to a registrar family: the family&apos;s members share these buckets
              on top of their own. The built-in starting value is three times the rates above.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Button type="button" variant="outline" size="sm" onClick={setFamilyToTriple}>
              Set to 3× the rates above
            </Button>
            {showFamily && <RatesGrid form={form} base="limits.family_rates" />}
          </CardContent>
        </Card>

        <Section title="Creates per day" description="A cap on successful domain creates per registrar per UTC day. Refused or failed creates do not count.">
          <label className="flex items-center gap-3 text-sm">
            <Switch checked={limitCreates} onCheckedChange={setLimitCreatesTo} aria-label="Limit successful domain creates per day" />
            Limit successful domain creates per day
          </label>
          {limitCreates ? (
            <NumberField form={form} name="limits.creates_per_day" label="Creates per day" description="Over it a create is refused with 2308. Probation starts at 100." />
          ) : (
            <p className="text-sm text-muted-foreground">No cap. The default policy starts this way.</p>
          )}
        </Section>

        <Section
          title="Failed-create budget"
          description="Catches drop-catching. A registrar is over budget when, in 5 minutes, at least this many creates were refused because the name is taken, and they are at least this share of all its creates. Over budget, creates are slowed, then refused, then the login is suspended for 15 minutes."
        >
          <div className="grid gap-6 sm:grid-cols-2">
            <div className="space-y-4">
              <div className="text-sm font-medium">Registrar</div>
              <NumberField form={form} name="limits.failed_creates.failures" label="Failures" description="Default 100 (probation 50)." />
              <NumberField form={form} name="limits.failed_creates.ratio_percent" label="Share of creates, %" description="1 to 100. Default 90." />
            </div>
            <div className="space-y-4">
              <div className="text-sm font-medium">Family (when this policy is a family&apos;s)</div>
              <NumberField form={form} name="limits.family_failed_creates.failures" label="Failures" description="Default 300 (probation 150)." />
              <NumberField form={form} name="limits.family_failed_creates.ratio_percent" label="Share of creates, %" description="1 to 100. Default 90." />
            </div>
          </div>
        </Section>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex gap-2">
            {builtIn && resetValues && (
              <Button type="button" variant="outline" onClick={resetToBuiltIn}>
                Reset to built-in starting values
              </Button>
            )}
            {mode === "edit" && !builtIn && onDelete && (
              <Button type="button" variant="destructive" onClick={onDelete}>
                Delete policy
              </Button>
            )}
          </div>
          <Button type="submit" disabled={saving}>
            {saving ? "Saving…" : mode === "create" ? "Create policy" : "Save"}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Changes reach every EPP node within about 30 seconds and apply to the next command and the next login.
          Sessions already open stay open under a lowered session cap.
        </p>
      </form>
    </Form>
  );
}
