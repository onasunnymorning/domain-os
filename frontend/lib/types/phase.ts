// Phase types based on backend entities
export type PhaseType = 'GA' | 'Launch';

export type PhaseStatus = 'past' | 'current' | 'future';

export type ContactDataPolicyType = 'mandatory' | 'optional' | 'prohibited';

export interface PhasePolicy {
  minLabelLength?: number;
  maxLabelLength?: number;
  registrationGP?: number;
  renewalGP?: number;
  autoRenewalGP?: number;
  // Grace period after a transfer (RFC 3915 transferPeriod).
  transferGP?: number;
  // Days the losing registrar has to answer a transfer request before the registry approves it.
  pendingTransferPeriod?: number;
  redemptionGP?: number;
  pendingdeleteGP?: number;
  // Days a registrar has to send the restore report after requesting a restore (RFC 3915).
  pendingRestorePeriod?: number;
  // Whether a restore waits for the restore report; unset is true. False restores on the request alone.
  restoreReportRequired?: boolean;
  // Days after a registration or a transfer in which the domain cannot be transferred; 0 is no lock.
  transferLockPeriod?: number;
  maxHorizon?: number;
  allowAutorenew?: boolean;
  requiresValidation?: boolean;
  baseCurrency?: string;
  registrantContactDataPolicy?: ContactDataPolicyType;
  techContactDataPolicy?: ContactDataPolicyType;
  adminContactDataPolicy?: ContactDataPolicyType;
  billingContactDataPolicy?: ContactDataPolicyType;
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
