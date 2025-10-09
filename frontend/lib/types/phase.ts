// Phase types based on backend entities
export type PhaseType = 'GA' | 'Launch';

export type PhaseStatus = 'past' | 'current' | 'future';

export interface PhasePolicy {
  minLabelLength?: number;
  maxLabelLength?: number;
  registrationGP?: number;
  renewalGP?: number;
  autoRenewalGP?: number;
  transferGP?: number;
  redemptionGP?: number;
  pendingdeleteGP?: number;
  transferLockPeriod?: number;
  maxHorizon?: number;
  allowAutorenew?: boolean;
  requiresValidation?: boolean;
  baseCurrency?: string;
}

export interface Price {
  id: number;
  currency: string;
  amount: number;
  createdAt: string;
  updatedAt: string;
}

export interface Fee {
  id: number;
  name: string;
  currency: string;
  amount: number;
  createdAt: string;
  updatedAt: string;
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
