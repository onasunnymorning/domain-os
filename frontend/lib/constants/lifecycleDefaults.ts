/**
 * The built-in gtld-standard policy's values, as the migrate job seeds them,
 * and the bounds the API holds every setting to. Source:
 * pkg/domain/entities/lifecyclePolicy.go. Keep them in step.
 */

import type { LifecycleSettings, ResolvedLifecycle } from "@/lib/types/lifecycle";

export const STANDARD_LIFECYCLE_POLICY = "gtld-standard";

export const STANDARD_LIFECYCLE: ResolvedLifecycle = {
  registrationGP: 5,
  renewalGP: 5,
  autorenewalGP: 45,
  transferGP: 5,
  transferLockPeriod: 60,
  pendingTransferPeriod: 5,
  redemptionGP: 30,
  pendingdeleteGP: 5,
  pendingRestorePeriod: 7,
  restoreReportRequired: true,
  allowAutorenew: true,
  maxHorizon: 10,
};

/** Every setting unset: a policy that follows the defaults. */
export const EMPTY_LIFECYCLE_SETTINGS: LifecycleSettings = {
  registrationGP: null,
  renewalGP: null,
  autorenewalGP: null,
  transferGP: null,
  transferLockPeriod: null,
  pendingTransferPeriod: null,
  redemptionGP: null,
  pendingdeleteGP: null,
  pendingRestorePeriod: null,
  restoreReportRequired: null,
  allowAutorenew: null,
  maxHorizon: null,
};

export type LifecycleDurationKey =
  | "registrationGP"
  | "renewalGP"
  | "autorenewalGP"
  | "transferGP"
  | "transferLockPeriod"
  | "pendingTransferPeriod"
  | "redemptionGP"
  | "pendingdeleteGP"
  | "pendingRestorePeriod";

/** The durations, in the order the lifecycle runs, with their upper bounds (days). */
export const LIFECYCLE_DURATIONS: { key: LifecycleDurationKey; label: string; help: string; max: number }[] = [
  { key: "registrationGP", label: "Add grace period", help: "Days after a create in which a delete earns a refund and skips redemption.", max: 365 },
  { key: "renewalGP", label: "Renew grace period", help: "Days after a renew in which a delete earns its refund.", max: 365 },
  { key: "autorenewalGP", label: "Auto-renew grace period", help: "Days after an auto-renew in which a delete earns its refund.", max: 365 },
  { key: "transferGP", label: "Transfer grace period", help: "Days after a transfer in which a delete earns its refund.", max: 365 },
  { key: "transferLockPeriod", label: "Transfer lock", help: "Days after a registration or a transfer in which the domain cannot be transferred. 0 is no lock.", max: 365 },
  { key: "pendingTransferPeriod", label: "Pending transfer", help: "Days the losing registrar has to answer a transfer request before the registry approves it.", max: 30 },
  { key: "redemptionGP", label: "Redemption period", help: "Days a deleted domain stays restorable.", max: 365 },
  { key: "pendingdeleteGP", label: "Pending delete", help: "Days after redemption until the domain is purged.", max: 90 },
  { key: "pendingRestorePeriod", label: "Restore report due", help: "Days a registrar has to send the restore report after requesting a restore.", max: 30 },
];

export const MAX_HORIZON_BOUNDS = { min: 1, max: 10 };

export const LIFECYCLE_POLICY_NAME_PATTERN = /^[a-z][a-z0-9-]{0,31}$/;

export function isStandardLifecyclePolicy(name: string): boolean {
  return name === STANDARD_LIFECYCLE_POLICY;
}

/** Fills every null setting with the standard value, as the API does. */
export function resolveLifecycle(settings: LifecycleSettings): ResolvedLifecycle {
  const out = { ...STANDARD_LIFECYCLE };
  for (const k of Object.keys(out) as (keyof ResolvedLifecycle)[]) {
    const v = settings[k];
    if (v !== null && v !== undefined) (out as Record<string, number | boolean>)[k] = v;
  }
  return out;
}
