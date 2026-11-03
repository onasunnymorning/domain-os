/**
 * Lifecycle policies (ADR 0023): the named timing a TLD phase follows.
 * Mirrors pkg/domain/entities/lifecyclePolicy.go.
 */

/** As stored: a null setting is the policy's default, 0 is zero days. */
export interface LifecycleSettings {
  registrationGP: number | null;
  renewalGP: number | null;
  autorenewalGP: number | null;
  transferGP: number | null;
  transferLockPeriod: number | null;
  pendingTransferPeriod: number | null;
  redemptionGP: number | null;
  pendingdeleteGP: number | null;
  pendingRestorePeriod: number | null;
  restoreReportRequired: boolean | null;
  allowAutorenew: boolean | null;
  maxHorizon: number | null;
}

/** Every setting resolved: what the lifecycle code reads. */
export interface ResolvedLifecycle {
  registrationGP: number;
  renewalGP: number;
  autorenewalGP: number;
  transferGP: number;
  transferLockPeriod: number;
  pendingTransferPeriod: number;
  redemptionGP: number;
  pendingdeleteGP: number;
  pendingRestorePeriod: number;
  restoreReportRequired: boolean;
  allowAutorenew: boolean;
  maxHorizon: number;
}

/** A phase that names the policy: what a change to it reaches. */
export interface LifecyclePolicyUse {
  tld: string;
  phase: string;
}

export interface LifecyclePolicy {
  name: string;
  description?: string;
  settings: LifecycleSettings;
  resolved: ResolvedLifecycle;
  updatedAt: string;
  updatedBy?: string;
  usedBy?: LifecyclePolicyUse[];
}

export interface SaveLifecyclePolicyRequest {
  description?: string;
  settings: LifecycleSettings;
}
