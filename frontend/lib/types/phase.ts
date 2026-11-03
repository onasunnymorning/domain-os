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
