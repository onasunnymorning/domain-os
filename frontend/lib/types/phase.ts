// Phase types based on backend entities
export type PhaseType = 'GA' | 'Launch';

export type PhaseStatus = 'past' | 'current' | 'future';

export type ContactDataPolicyType = 'mandatory' | 'optional' | 'prohibited';

import type { LifecyclePolicy } from './lifecycle';

// The phase's commercial and launch settings. Lifecycle timing (grace
// periods, locks, redemption, the restore report, the horizon) is the
// phase's named lifecycle policy (ADR 0023): see Phase.lifecyclePolicy.
export interface PhasePolicy {
  minLabelLength?: number;
  maxLabelLength?: number;
  requiresValidation?: boolean;
  baseCurrency?: string;
  registrantContactDataPolicy?: ContactDataPolicyType;
  techContactDataPolicy?: ContactDataPolicyType;
  adminContactDataPolicy?: ContactDataPolicyType;
  billingContactDataPolicy?: ContactDataPolicyType;
}

// What kind of launch phase a phase is and how it registers (ADR 0024).
// Sunrise and claims phases are not yet served over EPP.
export type LaunchMode = 'open' | 'landrush' | 'custom' | 'sunrise' | 'claims';
export type RegistrationModel = 'first-come' | 'application';

export interface LaunchSettings {
  mode?: LaunchMode;
  model?: RegistrationModel;
  acceptSignedMarks: boolean;
  acceptCodeMarks: boolean;
  retainMarks: boolean;
}

export const LAUNCH_MODES: { value: LaunchMode; label: string; hint: string; servedOverEPP: boolean }[] = [
  { value: 'open', label: 'Open', hint: 'A general launch window before or beside GA', servedOverEPP: true },
  { value: 'landrush', label: 'Landrush', hint: 'Early access, usually at a premium', servedOverEPP: true },
  { value: 'custom', label: 'Custom', hint: 'A phase of your own; registrars name it', servedOverEPP: true },
  { value: 'sunrise', label: 'Sunrise', hint: 'Trademark holders first; not yet served over EPP', servedOverEPP: false },
  { value: 'claims', label: 'Claims', hint: 'A claims period; not yet served over EPP', servedOverEPP: false },
];

export const REGISTRATION_MODELS: { value: RegistrationModel; label: string; hint: string }[] = [
  { value: 'first-come', label: 'First come', hint: 'A create registers the domain at once' },
  { value: 'application', label: 'Applications', hint: 'A create files an application; you allocate each name' },
];

// A registrar's application for a name in an application phase (ADR 0024 §3),
// as the admin API returns it. The transfer code itself never leaves the
// registry: AuthInfoSet says whether one is set.
export type LaunchApplicationStatus = 'pendingValidation' | 'pendingAllocation' | 'allocated' | 'rejected' | 'invalid';

export interface LaunchApplication {
  ID: string;
  Name: string;
  TLDName: string;
  PhaseName: string;
  PhaseMode: LaunchMode;
  ClID: string;
  Status: LaunchApplicationStatus;
  RegistrantID: string;
  AdminID: string;
  TechID: string;
  BillingID: string;
  HostNames: string[] | null;
  Years: number;
  AuthInfoSet: boolean;
  Quote: { Currency: string; Amount: number; Class: string };
  Reason: string;
  DomainRoID: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export const APPLICATION_STATUSES: LaunchApplicationStatus[] = ['pendingValidation', 'pendingAllocation', 'allocated', 'rejected', 'invalid'];

export function isFinalApplication(status: LaunchApplicationStatus): boolean {
  return status === 'allocated' || status === 'rejected' || status === 'invalid';
}

export interface Price {
  currency: string;
  registrationAmount: number;
  renewalAmount: number;
  transferAmount: number;
  restoreAmount: number;
  phaseid: number;
}

export interface Fee {
  currency: string;
  name: string;
  amount: number;
  refundable: boolean;
}

export interface Phase {
  id: number;
  name: string;
  type: PhaseType;
  starts: string; // ISO date string
  ends?: string | null; // ISO date string or null for ongoing
  prices: Price[];
  fees: Fee[];
  premiumListName?: string | null;
  createdAt: string;
  updatedAt: string;
  tldName: string;
  policy: PhasePolicy;
  // The lifecycle policy the phase names, and that policy as the API resolved it.
  lifecyclePolicyName: string;
  lifecyclePolicy?: LifecyclePolicy;
  // The launch settings; the zero value (no mode) on a GA phase.
  launch?: LaunchSettings;
}

export interface CategorizedPhases {
  ga: {
    current: Phase | null;
    past: Phase[];
    future: Phase[];
  };
  launch: {
    current: Phase[];
    past: Phase[];
    future: Phase[];
  };
}
