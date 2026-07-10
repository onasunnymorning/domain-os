'use client';

import { useReducer, useRef, useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useCreatePhase, useAddPrice, useAddFee } from '@/lib/hooks/usePhases';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { SystemBubble, UserBubble, ConversationProgress, StepInput } from '@/components/shared/ChatBubbles';
import { cn } from '@/lib/utils';
import {
  Layers,
  CalendarIcon,
  Import,
  CheckCircle2,
  Loader2,
  ArrowRight,
  DollarSign,
  Tag,
  RotateCcw,
  ExternalLink,
  Zap,
  Clock,
  AlertTriangle,
  Sparkles,
  Shield,
  ScrollText,
  Settings2,
} from 'lucide-react';
import { format } from 'date-fns';

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

type Step =
  | 'import-check'
  | 'type'
  | 'name'
  | 'start-date'
  | 'end-date'
  | 'pricing-offer'
  | 'pricing'
  | 'fee-offer'
  | 'fee-entry'
  | 'validation'
  | 'data-policy'
  | 'lifecycle'
  | 'review'
  | 'submitting'
  | 'done';

interface PriceEntry {
  currency: string;
  registrationAmount: number;
  renewalAmount: number;
  transferAmount: number;
  restoreAmount: number;
}

interface FeeEntry {
  name: string;
  currency: string;
  amount: number;
  refundable: boolean;
}

type ContactPolicyPreset = 'thick' | 'thin' | 'custom';
type ContactPolicyValue = 'mandatory' | 'optional' | 'prohibited';

interface PolicyState {
  requiresValidation: boolean;
  contactPreset: ContactPolicyPreset;
  registrantPolicy: ContactPolicyValue;
  techPolicy: ContactPolicyValue;
  adminPolicy: ContactPolicyValue;
  billingPolicy: ContactPolicyValue;
  useDefaultLifecycle: boolean;
  registrationGP: number;
  renewalGP: number;
  autoRenewalGP: number;
  transferGP: number;
  redemptionGP: number;
  pendingDeleteGP: number;
  transferLockPeriod: number;
  maxHorizon: number;
  allowAutoRenew: boolean;
}

const DEFAULT_POLICY: PolicyState = {
  requiresValidation: false,
  contactPreset: 'thick',
  registrantPolicy: 'mandatory',
  techPolicy: 'mandatory',
  adminPolicy: 'optional',
  billingPolicy: 'optional',
  useDefaultLifecycle: true,
  registrationGP: 5,
  renewalGP: 5,
  autoRenewalGP: 45,
  transferGP: 5,
  redemptionGP: 30,
  pendingDeleteGP: 5,
  transferLockPeriod: 60,
  maxHorizon: 10,
  allowAutoRenew: true,
};

interface WizardState {
  step: Step;
  phaseType: 'GA' | 'Launch' | null;
  phaseName: string;
  starts: Date | null;
  startIsNow: boolean;
  startsTime: { hours: string; minutes: string };
  ends: Date | null;
  endsTime: { hours: string; minutes: string };
  openEnded: boolean;
  prices: PriceEntry[];
  fees: FeeEntry[];
  policy: PolicyState;
  error: string | null;
  pricingWarning: string | null;
  createdPhaseName: string | null;
}

type WizardAction =
  | { type: 'CONTINUE_IMPORT' }
  | { type: 'SET_PHASE_TYPE'; phaseType: 'GA' | 'Launch' }
  | { type: 'SET_NAME'; name: string }
  | { type: 'SET_START_DATE'; date: Date; hours: string; minutes: string; isNow?: boolean }
  | { type: 'SET_END_DATE'; date: Date | null; hours: string; minutes: string; openEnded: boolean }
  | { type: 'OFFER_PRICING' }
  | { type: 'ADD_PRICE'; price: PriceEntry }
  | { type: 'SKIP_PRICING' }
  | { type: 'CONTINUE_TO_FEES' }
  | { type: 'OFFER_FEE' }
  | { type: 'ADD_FEE'; fee: FeeEntry }
  | { type: 'SKIP_FEES' }
  | { type: 'CONTINUE_TO_VALIDATION' }
  | { type: 'SET_VALIDATION'; requires: boolean }
  | { type: 'SET_DATA_POLICY'; preset: ContactPolicyPreset; registrant: ContactPolicyValue; tech: ContactPolicyValue; admin: ContactPolicyValue; billing: ContactPolicyValue }
  | { type: 'SET_LIFECYCLE'; policy: Partial<PolicyState> }
  | { type: 'SUBMIT' }
  | { type: 'SUBMIT_SUCCESS'; name: string; pricingWarning?: string }
  | { type: 'SUBMIT_ERROR'; error: string }
  | { type: 'RESET' };

const initialState: WizardState = {
  step: 'import-check',
  phaseType: null,
  phaseName: '',
  starts: null,
  startIsNow: false,
  startsTime: { hours: '00', minutes: '00' },
  ends: null,
  endsTime: { hours: '00', minutes: '00' },
  openEnded: true,
  prices: [],
  fees: [],
  policy: { ...DEFAULT_POLICY },
  error: null,
  pricingWarning: null,
  createdPhaseName: null,
};

function reducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'CONTINUE_IMPORT':
      return { ...state, step: 'type', error: null };
    case 'SET_PHASE_TYPE':
      return { ...state, step: 'name', phaseType: action.phaseType, error: null };
    case 'SET_NAME':
      return { ...state, step: 'start-date', phaseName: action.name, error: null };
    case 'SET_START_DATE':
      return { ...state, step: 'end-date', starts: action.date, startIsNow: !!action.isNow, startsTime: { hours: action.hours, minutes: action.minutes }, error: null };
    case 'SET_END_DATE':
      return { ...state, step: 'pricing-offer', ends: action.date, endsTime: { hours: action.hours, minutes: action.minutes }, openEnded: action.openEnded, error: null };
    case 'OFFER_PRICING':
      return { ...state, step: 'pricing', error: null };
    case 'ADD_PRICE':
      return { ...state, prices: [...state.prices, action.price] };
    case 'SKIP_PRICING':
      return { ...state, step: 'validation', error: null };
    case 'CONTINUE_TO_FEES':
      return { ...state, step: 'fee-offer', error: null };
    case 'OFFER_FEE':
      return { ...state, step: 'fee-entry', error: null };
    case 'ADD_FEE':
      return { ...state, fees: [...state.fees, action.fee] };
    case 'SKIP_FEES':
      return { ...state, step: 'validation', error: null };
    case 'CONTINUE_TO_VALIDATION':
      return { ...state, step: 'validation', error: null };
    case 'SET_VALIDATION':
      return { ...state, step: 'data-policy', policy: { ...state.policy, requiresValidation: action.requires }, error: null };
    case 'SET_DATA_POLICY':
      return { ...state, step: 'lifecycle', policy: { ...state.policy, contactPreset: action.preset, registrantPolicy: action.registrant, techPolicy: action.tech, adminPolicy: action.admin, billingPolicy: action.billing }, error: null };
    case 'SET_LIFECYCLE':
      return { ...state, step: 'review', policy: { ...state.policy, ...action.policy }, error: null };
    case 'SUBMIT':
      return { ...state, step: 'submitting', error: null };
    case 'SUBMIT_SUCCESS':
      return { ...state, step: 'done', createdPhaseName: action.name, pricingWarning: action.pricingWarning || null, error: null };
    case 'SUBMIT_ERROR':
      return { ...state, step: 'review', error: action.error };
    case 'RESET':
      return { ...initialState };
    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function buildUTCDate(date: Date, hours: string, minutes: string): Date {
  const d = new Date(date);
  d.setUTCHours(parseInt(hours, 10), parseInt(minutes, 10), 0, 0);
  return d;
}

function formatCurrency(amount: number, currency: string): string {
  // Convert from smallest unit (cents) to major unit
  const major = amount / 100;
  return new Intl.NumberFormat('en-US', { style: 'currency', currency, minimumFractionDigits: 2 }).format(major);
}

/** Convert a human-friendly decimal string (e.g. "25.00") to smallest currency unit (cents). */
function toSmallestUnit(value: string): number {
  const parsed = parseFloat(value);
  if (isNaN(parsed)) return NaN;
  return Math.round(parsed * 100);
}

/** Format a currency symbol for display (e.g. "USD" → "$", "EUR" → "€"). */
function currencySymbol(currency: string): string {
  try {
    // Extract just the symbol from a zero-formatted string
    return new Intl.NumberFormat('en-US', { style: 'currency', currency, currencyDisplay: 'narrowSymbol' })
      .formatToParts(0)
      .find(p => p.type === 'currency')?.value ?? currency;
  } catch {
    return currency;
  }
}

// Step keys for progress bar (user-visible steps only)
const PROGRESS_STEPS = ['type', 'name', 'start-date', 'end-date', 'pricing-offer', 'validation', 'review'];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface PhaseCreateConversationProps {
  tldName: string;
  open: boolean;
  onClose: () => void;
  existingPhases?: Array<{
    name: string;
    type: 'GA' | 'Launch';
    starts: string;
    ends?: string | null;
  }>;
}

export function PhaseCreateConversation({ tldName, open, onClose, existingPhases = [] }: PhaseCreateConversationProps) {
  const router = useRouter();
  const [state, dispatch] = useReducer(reducer, initialState);
  const { mutate: createPhase } = useCreatePhase(tldName);

  // Scroll container ref
  const scrollRef = useRef<HTMLDivElement>(null);

  // Per-step local input
  const [nameInput, setNameInput] = useState('');
  const [nameError, setNameError] = useState<string | null>(null);

  // Date inputs
  const [startDate, setStartDate] = useState<Date | undefined>();
  const [startHours, setStartHours] = useState('00');
  const [startMinutes, setStartMinutes] = useState('00');
  const [endDate, setEndDate] = useState<Date | undefined>();
  const [endHours, setEndHours] = useState('00');
  const [endMinutes, setEndMinutes] = useState('00');

  // Pricing inputs
  const [priceCurrency, setPriceCurrency] = useState('USD');
  const [priceReg, setPriceReg] = useState('');
  const [priceRenew, setPriceRenew] = useState('');
  const [priceTransfer, setPriceTransfer] = useState('');
  const [priceRestore, setPriceRestore] = useState('');
  const [priceError, setPriceError] = useState<string | null>(null);

  // Fee inputs
  const [feeName, setFeeName] = useState('');
  const [feeCurrency, setFeeCurrency] = useState('USD');
  const [feeAmount, setFeeAmount] = useState('');
  const [feeRefundable, setFeeRefundable] = useState(false);
  const [feeError, setFeeError] = useState<string | null>(null);

  // Policy inputs (custom data policy)
  const [showCustomPolicy, setShowCustomPolicy] = useState(false);
  const [customContactPolicy, setCustomContactPolicy] = useState({
    customRegistrant: 'mandatory' as ContactPolicyValue,
    customTech: 'mandatory' as ContactPolicyValue,
    customAdmin: 'optional' as ContactPolicyValue,
    customBilling: 'optional' as ContactPolicyValue,
  });

  // Lifecycle inputs (custom lifecycle)
  const [showCustomLifecycle, setShowCustomLifecycle] = useState(false);
  const [customLifecycle, setCustomLifecycle] = useState({
    registrationGP: DEFAULT_POLICY.registrationGP,
    renewalGP: DEFAULT_POLICY.renewalGP,
    autoRenewalGP: DEFAULT_POLICY.autoRenewalGP,
    transferGP: DEFAULT_POLICY.transferGP,
    redemptionGP: DEFAULT_POLICY.redemptionGP,
    pendingDeleteGP: DEFAULT_POLICY.pendingDeleteGP,
    transferLockPeriod: DEFAULT_POLICY.transferLockPeriod,
    maxHorizon: DEFAULT_POLICY.maxHorizon,
    allowAutoRenew: DEFAULT_POLICY.allowAutoRenew,
  });

  // Overlap/continuity info
  const [overlapWarning, setOverlapWarning] = useState<string | null>(null);
  const [continuityInfo, setContinuityInfo] = useState<string | null>(null);

  // Auto-scroll on step changes
  useEffect(() => {
    if (scrollRef.current) {
      setTimeout(() => {
        scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
      }, 100);
    }
  }, [state.step, state.prices.length, state.fees.length, nameError, priceError, feeError]);

  // Reset on open
  useEffect(() => {
    if (open) {
      dispatch({ type: 'RESET' });
      setNameInput('');
      setNameError(null);
      setStartDate(undefined);
      setStartHours('00');
      setStartMinutes('00');
      setEndDate(undefined);
      setEndHours('00');
      setEndMinutes('00');
      setPriceCurrency('USD');
      setPriceReg('');
      setPriceRenew('');
      setPriceTransfer('');
      setPriceRestore('');
      setPriceError(null);
      setFeeName('');
      setFeeCurrency('USD');
      setFeeAmount('');
      setFeeRefundable(false);
      setFeeError(null);
      setOverlapWarning(null);
      setContinuityInfo(null);
      setShowCustomPolicy(false);
      setCustomContactPolicy({
        customRegistrant: 'mandatory',
        customTech: 'mandatory',
        customAdmin: 'optional',
        customBilling: 'optional',
      });
      setShowCustomLifecycle(false);
      setCustomLifecycle({
        registrationGP: DEFAULT_POLICY.registrationGP,
        renewalGP: DEFAULT_POLICY.renewalGP,
        autoRenewalGP: DEFAULT_POLICY.autoRenewalGP,
        transferGP: DEFAULT_POLICY.transferGP,
        redemptionGP: DEFAULT_POLICY.redemptionGP,
        pendingDeleteGP: DEFAULT_POLICY.pendingDeleteGP,
        transferLockPeriod: DEFAULT_POLICY.transferLockPeriod,
        maxHorizon: DEFAULT_POLICY.maxHorizon,
        allowAutoRenew: DEFAULT_POLICY.allowAutoRenew,
      });
    }
  }, [open]);

  // Auto-populate start date from previous phase of same type
  useEffect(() => {
    if (state.phaseType && existingPhases.length > 0) {
      const sameType = existingPhases
        .filter(p => p.type === state.phaseType && p.ends)
        .sort((a, b) => new Date(b.ends!).getTime() - new Date(a.ends!).getTime());

      if (sameType.length > 0 && sameType[0].ends) {
        const endDate = new Date(sameType[0].ends);
        setStartDate(endDate);
        setStartHours(endDate.getUTCHours().toString().padStart(2, '0'));
        setStartMinutes(endDate.getUTCMinutes().toString().padStart(2, '0'));
      }
    }
  }, [state.phaseType, existingPhases]);

  // Pre-fill a proposed phase name for GA phases (ga-YYYY-MM)
  useEffect(() => {
    if (state.step === 'name' && state.phaseType === 'GA' && !nameInput) {
      const now = new Date();
      const proposed = `ga-${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
      setNameInput(proposed);
    }
  }, [state.step, state.phaseType]); // intentionally omit nameInput to avoid overwriting user edits

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  const handleNameSubmit = useCallback(() => {
    const name = nameInput.trim();
    if (!name) { setNameError('Phase name is required'); return; }
    if (name.length < 3) { setNameError('Must be at least 3 characters'); return; }
    if (name.length > 16) { setNameError('Must be 16 characters or less'); return; }
    // eslint-disable-next-line no-control-regex
    if (!/^[\x00-\x7F]+$/.test(name)) { setNameError('Only ASCII characters allowed'); return; }
    if (existingPhases.some(p => p.name.toLowerCase() === name.toLowerCase())) {
      setNameError('A phase with this name already exists');
      return;
    }
    setNameError(null);
    dispatch({ type: 'SET_NAME', name });
  }, [nameInput, existingPhases]);

  const handleStartDateSubmit = useCallback(() => {
    if (!startDate) return;
    const d = buildUTCDate(startDate, startHours, startMinutes);

    // Check GA overlap
    if (state.phaseType === 'GA') {
      const newStart = d;
      const gaPhases = existingPhases.filter(p => p.type === 'GA');

      // Check continuity
      const continuous = gaPhases.find(p => {
        if (!p.ends) return false;
        return newStart.getTime() === new Date(p.ends).getTime();
      });
      setContinuityInfo(continuous ? `Continuous with "${continuous.name}"` : null);
    }

    dispatch({ type: 'SET_START_DATE', date: d, hours: startHours, minutes: startMinutes });
  }, [startDate, startHours, startMinutes, state.phaseType, existingPhases]);

  const handleEndDateSubmit = useCallback((openEnded: boolean) => {
    if (openEnded) {
      dispatch({ type: 'SET_END_DATE', date: null, hours: '00', minutes: '00', openEnded: true });
      return;
    }
    if (!endDate) return;
    const d = buildUTCDate(endDate, endHours, endMinutes);
    if (state.starts && d <= state.starts) return; // end must be after start

    // Check GA overlap with end date
    if (state.phaseType === 'GA') {
      const newStart = state.starts!;
      const newEnd = d;
      const overlap = existingPhases.filter(p => p.type === 'GA').find(p => {
        const pStart = new Date(p.starts);
        const pEnd = p.ends ? new Date(p.ends) : null;
        if (!pEnd) return newStart < pStart ? false : true; // ongoing existing
        return newStart < pEnd && pStart < newEnd;
      });
      if (overlap) {
        setOverlapWarning(`Would overlap with GA phase "${overlap.name}"`);
        return;
      }
    }

    dispatch({ type: 'SET_END_DATE', date: d, hours: endHours, minutes: endMinutes, openEnded: false });
  }, [endDate, endHours, endMinutes, state.starts, state.phaseType, existingPhases]);

  const handleAddPrice = useCallback(() => {
    const reg = toSmallestUnit(priceReg);
    const renew = toSmallestUnit(priceRenew);
    const transfer = toSmallestUnit(priceTransfer);
    const restore = toSmallestUnit(priceRestore);

    if (!priceCurrency.trim()) { setPriceError('Currency is required'); return; }
    if (isNaN(reg) || isNaN(renew) || isNaN(transfer) || isNaN(restore)) {
      setPriceError('All amounts are required');
      return;
    }
    if (reg < 0 || renew < 0 || transfer < 0 || restore < 0) {
      setPriceError('Amounts must be positive');
      return;
    }
    if (state.prices.some(p => p.currency.toUpperCase() === priceCurrency.toUpperCase())) {
      setPriceError(`Price for ${priceCurrency.toUpperCase()} already added`);
      return;
    }

    setPriceError(null);
    dispatch({
      type: 'ADD_PRICE',
      price: {
        currency: priceCurrency.toUpperCase(),
        registrationAmount: reg,
        renewalAmount: renew,
        transferAmount: transfer,
        restoreAmount: restore,
      },
    });

    // Reset inputs for potential next currency
    setPriceCurrency('');
    setPriceReg('');
    setPriceRenew('');
    setPriceTransfer('');
    setPriceRestore('');
  }, [priceCurrency, priceReg, priceRenew, priceTransfer, priceRestore, state.prices]);

  const handleAddFee = useCallback(() => {
    const amount = toSmallestUnit(feeAmount);
    if (!feeName.trim()) { setFeeError('Fee name is required'); return; }
    if (!feeCurrency.trim()) { setFeeError('Currency is required'); return; }
    if (isNaN(amount) || amount < 0) { setFeeError('Enter a valid amount'); return; }
    if (state.fees.some(f => f.name.toLowerCase() === feeName.toLowerCase() && f.currency.toUpperCase() === feeCurrency.toUpperCase())) {
      setFeeError(`Fee "${feeName}" in ${feeCurrency.toUpperCase()} already added`);
      return;
    }

    setFeeError(null);
    dispatch({
      type: 'ADD_FEE',
      fee: {
        name: feeName.trim(),
        currency: feeCurrency.toUpperCase(),
        amount,
        refundable: feeRefundable,
      },
    });

    // Reset for next fee
    setFeeName('');
    setFeeAmount('');
    setFeeRefundable(false);
  }, [feeName, feeCurrency, feeAmount, feeRefundable, state.fees]);

  const handleSubmit = useCallback(() => {
    dispatch({ type: 'SUBMIT' });

    // If user chose "Now", compute a fresh timestamp with a 30s buffer
    const startsISO = state.startIsNow
      ? new Date(Date.now() + 30_000).toISOString()
      : state.starts!.toISOString();
    const endsISO = state.ends ? state.ends.toISOString() : undefined;

    createPhase(
      { name: state.phaseName, type: state.phaseType!, starts: startsISO, ends: endsISO },
      {
        onSuccess: async () => {
          const warnings: string[] = [];

          // Add prices
          try {
            for (const price of state.prices) {
              console.log('[PhaseWizard] Adding price:', { tldName, phaseName: state.phaseName, price });
              await phasesApiAddPrice(tldName, state.phaseName, price);
            }
          } catch (err: any) {
            const msg = err?.response?.data?.error || err?.message || 'Unknown error';
            const status = err?.response?.status;
            warnings.push(`Pricing failed (${status || 'network'}): ${msg}`);
            console.error('Failed to add prices:', { error: err, response: err?.response?.data, status });
          }

          // Add fees
          try {
            for (const fee of state.fees) {
              await phasesApiAddFee(tldName, state.phaseName, fee);
            }
          } catch (err: any) {
            const msg = err?.response?.data?.error || err?.message || 'Unknown error';
            warnings.push(`Fees failed: ${msg}`);
            console.error('Failed to add fees:', err);
          }

          // Update policy if non-default
          try {
            const p = state.policy;
            const policyPayload: Record<string, any> = {};
            if (p.requiresValidation) policyPayload.requiresValidation = true;
            if (p.registrantPolicy !== 'mandatory') policyPayload.registrantContactDataPolicy = p.registrantPolicy;
            if (p.techPolicy !== 'mandatory') policyPayload.techContactDataPolicy = p.techPolicy;
            if (p.adminPolicy !== 'optional') policyPayload.adminContactDataPolicy = p.adminPolicy;
            if (p.billingPolicy !== 'optional') policyPayload.billingContactDataPolicy = p.billingPolicy;
            if (!p.useDefaultLifecycle) {
              policyPayload.registrationGP = p.registrationGP;
              policyPayload.renewalGP = p.renewalGP;
              policyPayload.autorenewalGP = p.autoRenewalGP;
              policyPayload.transferGP = p.transferGP;
              policyPayload.redemptionGP = p.redemptionGP;
              policyPayload.pendingdeleteGP = p.pendingDeleteGP;
              policyPayload.transferLockPeriod = p.transferLockPeriod;
              policyPayload.maxHorizon = p.maxHorizon;
              policyPayload.allowAutorenew = p.allowAutoRenew;
            }

            if (Object.keys(policyPayload).length > 0) {
              await phasesApiUpdatePolicy(tldName, state.phaseName, policyPayload);
            }
          } catch (err: any) {
            const msg = err?.response?.data?.error || err?.message || 'Unknown error';
            warnings.push(`Policy update failed: ${msg}`);
            console.error('Failed to update policy:', err);
          }

          dispatch({
            type: 'SUBMIT_SUCCESS',
            name: state.phaseName,
            pricingWarning: warnings.length > 0 ? warnings.join('. ') : undefined,
          });
        },
        onError: (error: any) => {
          const message = error.response?.data?.error || error.message || 'Failed to create phase';
          dispatch({ type: 'SUBMIT_ERROR', error: message });
        },
      },
    );
  }, [state, createPhase, tldName]);

  // Direct API calls for post-creation pricing/fees/policy (not via hooks since phaseName is dynamic)
  const phasesApiAddPrice = async (tld: string, phase: string, price: PriceEntry) => {
    const { phasesApi } = await import('@/lib/api/phases');
    return phasesApi.addPrice(tld, phase, price);
  };

  const phasesApiAddFee = async (tld: string, phase: string, fee: FeeEntry) => {
    const { phasesApi } = await import('@/lib/api/phases');
    return phasesApi.addFee(tld, phase, fee);
  };

  const phasesApiUpdatePolicy = async (tld: string, phase: string, policy: Record<string, any>) => {
    const { phasesApi } = await import('@/lib/api/phases');
    return phasesApi.updatePolicy(tld, phase, policy as any);
  };

  // ---------------------------------------------------------------------------
  // Progress
  // ---------------------------------------------------------------------------

  const stepOrder: Step[] = ['import-check', 'type', 'name', 'start-date', 'end-date', 'pricing-offer', 'pricing', 'fee-offer', 'fee-entry', 'validation', 'data-policy', 'lifecycle', 'review', 'submitting', 'done'];
  const currentIndex = stepOrder.indexOf(state.step);
  const pastStep = (s: Step) => stepOrder.indexOf(s) < currentIndex;
  const progressIndex = PROGRESS_STEPS.indexOf(
    PROGRESS_STEPS.find(s => stepOrder.indexOf(s as Step) >= currentIndex) || 'review'
  );

  // Suggested start date from previous phase
  const suggestedStart = (() => {
    if (!state.phaseType || existingPhases.length === 0) return null;
    const sameType = existingPhases
      .filter(p => p.type === state.phaseType && p.ends)
      .sort((a, b) => new Date(b.ends!).getTime() - new Date(a.ends!).getTime());
    return sameType.length > 0 && sameType[0].ends ? new Date(sameType[0].ends) : null;
  })();

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <Dialog open={open} onOpenChange={(v) => {
      if (state.step === 'submitting') return;
      if (!v) onClose();
    }}>
      <DialogContent className="sm:max-w-md p-0 gap-0 overflow-hidden" showCloseButton={state.step !== 'submitting'}>
        {/* Header */}
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-border/50">
          <DialogTitle className="flex items-center gap-2 text-base">
            <Layers className="h-4 w-4 text-primary" />
            New Phase for .{tldName}
          </DialogTitle>
          <DialogDescription className="sr-only">
            Create a new phase through a guided conversation
          </DialogDescription>
        </DialogHeader>

        {/* Chat area */}
        <div
          ref={scrollRef}
          className="px-6 py-5 space-y-4 max-h-[480px] overflow-y-auto scroll-smooth"
        >
          {/* ── Step: Import Check ────────────────────────────────── */}
          <SystemBubble icon={Import}>
            <p>Before creating a phase, ensure any <strong>data import</strong> is complete.</p>
            <p className="mt-1 text-muted-foreground text-xs">
              Escrow imports require no active phases. If you need to import existing registry data, do that first.
            </p>
          </SystemBubble>

          {state.step === 'import-check' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 flex flex-col gap-2 items-start ml-10">
              <Button
                size="sm"
                variant="outline"
                className="gap-1.5 text-muted-foreground"
                onClick={() => {
                  // Navigate to workflows page for import
                  router.push(`/tlds/${tldName}?tab=phases`);
                  onClose();
                  // Open workflow launch from elsewhere — use the workflow shortcuts pattern
                  window.open('/workflows?launch=escrow-import', '_blank');
                }}
              >
                <ExternalLink className="h-3.5 w-3.5" />
                Run Import Workflow
              </Button>
              <Button
                size="sm"
                onClick={() => dispatch({ type: 'CONTINUE_IMPORT' })}
                className="gap-1.5"
              >
                <ArrowRight className="h-3.5 w-3.5" />
                Continue — I&apos;m ready
              </Button>
            </div>
          )}

          {pastStep('import-check') && (
            <UserBubble>Ready to create phase</UserBubble>
          )}

          {/* ── Step: Phase Type ──────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('type') && (
            <SystemBubble icon={Layers}>
              What type of phase is this?
            </SystemBubble>
          )}

          {state.step === 'type' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 grid grid-cols-2 gap-3">
              <button
                onClick={() => dispatch({ type: 'SET_PHASE_TYPE', phaseType: 'GA' })}
                className="text-left rounded-lg border-2 border-border/60 hover:border-primary/50 hover:bg-primary/5 p-3 transition-all duration-200 group"
              >
                <div className="text-sm font-semibold text-orange-600 dark:text-orange-400 group-hover:text-primary">GA Phase</div>
                <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                  General Availability — only one active at a time, continuous timeline, no overlaps.
                </p>
              </button>
              <button
                onClick={() => dispatch({ type: 'SET_PHASE_TYPE', phaseType: 'Launch' })}
                className="text-left rounded-lg border-2 border-border/60 hover:border-primary/50 hover:bg-primary/5 p-3 transition-all duration-200 group"
              >
                <div className="text-sm font-semibold text-muted-foreground group-hover:text-primary">Launch Phase</div>
                <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                  Sunrise, Landrush, or promotions. Multiple can run simultaneously.
                </p>
              </button>
            </div>
          )}

          {pastStep('type') && state.phaseType && (
            <UserBubble>
              <Badge variant={state.phaseType === 'GA' ? 'default' : 'secondary'} className="text-xs">
                {state.phaseType === 'GA' ? 'General Availability' : 'Launch Phase'}
              </Badge>
            </UserBubble>
          )}

          {/* ── Step: Name ────────────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('name') && (
            <SystemBubble>
              Give this phase a name. <span className="text-muted-foreground">3–16 characters, ASCII only.</span>
            </SystemBubble>
          )}

          {state.step === 'name' && (
            <div className="ml-10">
              <StepInput
                value={nameInput}
                onChange={setNameInput}
                onSubmit={handleNameSubmit}
                placeholder="e.g., ga-2025, sunrise-1"
                maxLength={16}
                error={nameError}
              />
            </div>
          )}

          {pastStep('name') && (
            <UserBubble>{state.phaseName}</UserBubble>
          )}

          {/* ── Step: Start Date ──────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('start-date') && (
            <SystemBubble icon={CalendarIcon}>
              {suggestedStart ? (
                <>
                  The previous {state.phaseType} phase ends on{' '}
                  <strong>{format(suggestedStart, 'PPP')} {format(suggestedStart, 'HH:mm')} UTC</strong>.
                  <br />
                  <span className="text-muted-foreground">Start right where it left off, or pick a different date.</span>
                </>
              ) : (
                'When should this phase begin?'
              )}
            </SystemBubble>
          )}

          {state.step === 'start-date' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 space-y-3">
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  className="gap-1.5"
                  onClick={() => {
                    const now = new Date();
                    dispatch({
                      type: 'SET_START_DATE',
                      date: now,
                      hours: now.getUTCHours().toString().padStart(2, '0'),
                      minutes: now.getUTCMinutes().toString().padStart(2, '0'),
                      isNow: true,
                    });
                  }}
                >
                  <Zap className="h-3.5 w-3.5" />
                  Now
                </Button>
                {suggestedStart && (
                  <Button
                    size="sm"
                    variant="outline"
                    className="gap-1.5"
                    onClick={() => {
                      const d = suggestedStart;
                      dispatch({
                        type: 'SET_START_DATE',
                        date: d,
                        hours: d.getUTCHours().toString().padStart(2, '0'),
                        minutes: d.getUTCMinutes().toString().padStart(2, '0'),
                      });
                    }}
                  >
                    <CalendarIcon className="h-3.5 w-3.5" />
                    Use {format(suggestedStart, 'PPP HH:mm')} UTC
                  </Button>
                )}
              </div>
              <div className="space-y-2">
                <Popover>
                  <PopoverTrigger asChild>
                    <Button variant="outline" size="sm" className={cn('w-full justify-start text-left font-normal', !startDate && 'text-muted-foreground')}>
                      <CalendarIcon className="mr-2 h-3.5 w-3.5" />
                      {startDate ? format(startDate, 'PPP') : 'Pick a date'}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-0" align="start">
                    <Calendar mode="single" selected={startDate} onSelect={setStartDate} />
                  </PopoverContent>
                </Popover>
                <div className="flex items-center gap-2">
                  <Clock className="h-3.5 w-3.5 text-muted-foreground" />
                  <Input
                    type="text"
                    placeholder="HH"
                    value={startHours}
                    onChange={(e) => setStartHours(e.target.value.replace(/\D/g, '').slice(0, 2))}
                    className="w-14 text-center text-sm"
                  />
                  <span className="text-muted-foreground">:</span>
                  <Input
                    type="text"
                    placeholder="MM"
                    value={startMinutes}
                    onChange={(e) => setStartMinutes(e.target.value.replace(/\D/g, '').slice(0, 2))}
                    className="w-14 text-center text-sm"
                  />
                  <span className="text-xs text-muted-foreground">UTC</span>
                </div>
                <Button size="sm" onClick={handleStartDateSubmit} disabled={!startDate} className="gap-1.5">
                  <ArrowRight className="h-3.5 w-3.5" /> Confirm start
                </Button>
              </div>
            </div>
          )}

          {pastStep('start-date') && state.starts && (
            <UserBubble>{state.startIsNow ? 'Now' : `Starts ${format(state.starts, 'PPP HH:mm')} UTC`}</UserBubble>
          )}

          {continuityInfo && pastStep('start-date') && (
            <div className="ml-10 flex items-center gap-2 text-xs text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="h-3 w-3" />
              {continuityInfo}
            </div>
          )}

          {/* ── Step: End Date ────────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('end-date') && (
            <SystemBubble icon={CalendarIcon}>
              When should this phase end? <span className="text-muted-foreground">Many {state.phaseType === 'GA' ? 'GA' : 'launch'} phases run indefinitely.</span>
            </SystemBubble>
          )}

          {state.step === 'end-date' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 space-y-3">
              <Button
                size="sm"
                onClick={() => handleEndDateSubmit(true)}
                className="gap-1.5"
              >
                <ArrowRight className="h-3.5 w-3.5" /> Leave open-ended
              </Button>
              <div className="space-y-2">
                <p className="text-[11px] text-muted-foreground">Or set a specific end date:</p>
                <Popover>
                  <PopoverTrigger asChild>
                    <Button variant="outline" size="sm" className={cn('w-full justify-start text-left font-normal', !endDate && 'text-muted-foreground')}>
                      <CalendarIcon className="mr-2 h-3.5 w-3.5" />
                      {endDate ? format(endDate, 'PPP') : 'Pick end date'}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-0" align="start">
                    <Calendar mode="single" selected={endDate} onSelect={setEndDate} />
                  </PopoverContent>
                </Popover>
                <div className="flex items-center gap-2">
                  <Clock className="h-3.5 w-3.5 text-muted-foreground" />
                  <Input
                    type="text"
                    placeholder="HH"
                    value={endHours}
                    onChange={(e) => setEndHours(e.target.value.replace(/\D/g, '').slice(0, 2))}
                    className="w-14 text-center text-sm"
                  />
                  <span className="text-muted-foreground">:</span>
                  <Input
                    type="text"
                    placeholder="MM"
                    value={endMinutes}
                    onChange={(e) => setEndMinutes(e.target.value.replace(/\D/g, '').slice(0, 2))}
                    className="w-14 text-center text-sm"
                  />
                  <span className="text-xs text-muted-foreground">UTC</span>
                </div>
                {overlapWarning && (
                  <div className="flex items-center gap-2 text-xs text-destructive">
                    <AlertTriangle className="h-3 w-3" />
                    {overlapWarning}
                  </div>
                )}
                <Button size="sm" variant="outline" onClick={() => handleEndDateSubmit(false)} disabled={!endDate} className="gap-1.5">
                  <ArrowRight className="h-3.5 w-3.5" /> Set end date
                </Button>
              </div>
            </div>
          )}

          {pastStep('end-date') && (
            <UserBubble>{state.openEnded ? 'Open-ended (ongoing)' : `Ends ${format(state.ends!, 'PPP HH:mm')} UTC`}</UserBubble>
          )}

          {/* ── Step: Pricing Offer ──────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('pricing-offer') && (
            <SystemBubble icon={DollarSign}>
              Would you like to add pricing now? <span className="text-muted-foreground">You can always configure this later.</span>
            </SystemBubble>
          )}

          {state.step === 'pricing-offer' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 flex gap-2 ml-10">
              <Button size="sm" onClick={() => dispatch({ type: 'OFFER_PRICING' })} className="gap-1.5">
                <DollarSign className="h-3.5 w-3.5" /> Add pricing
              </Button>
              <Button size="sm" variant="outline" onClick={() => dispatch({ type: 'SKIP_PRICING' })}>
                Skip for now
              </Button>
            </div>
          )}

          {/* ── Step: Pricing Entry ──────────────────────────────── */}
          {state.step === 'pricing' && (
            <>
              {state.prices.length > 0 && (
                <>
                  {state.prices.map((p, i) => (
                    <UserBubble key={i}>
                      <span className="font-mono text-xs">{p.currency}</span> — Reg: {formatCurrency(p.registrationAmount, p.currency)}, Renew: {formatCurrency(p.renewalAmount, p.currency)}
                    </UserBubble>
                  ))}
                  <SystemBubble icon={DollarSign}>
                    Price added! Add another currency or continue to fees.
                  </SystemBubble>
                </>
              )}

              {state.prices.length === 0 && (
                <SystemBubble icon={DollarSign}>
                  Set the standard prices.
                </SystemBubble>
              )}

              <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 space-y-3">
                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1">
                    <Label className="text-[11px]">Currency</Label>
                    <Input value={priceCurrency} onChange={(e) => setPriceCurrency(e.target.value.toUpperCase())} placeholder="USD" className="text-sm font-mono" maxLength={3} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[11px]">Registration</Label>
                    <div className="relative">
                      <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{currencySymbol(priceCurrency || 'USD')}</span>
                      <Input inputMode="decimal" value={priceReg} onChange={(e) => setPriceReg(e.target.value)} placeholder="10.00" className="text-sm pl-7" />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[11px]">Renewal</Label>
                    <div className="relative">
                      <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{currencySymbol(priceCurrency || 'USD')}</span>
                      <Input inputMode="decimal" value={priceRenew} onChange={(e) => setPriceRenew(e.target.value)} placeholder="10.00" className="text-sm pl-7" />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[11px]">Transfer</Label>
                    <div className="relative">
                      <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{currencySymbol(priceCurrency || 'USD')}</span>
                      <Input inputMode="decimal" value={priceTransfer} onChange={(e) => setPriceTransfer(e.target.value)} placeholder="10.00" className="text-sm pl-7" />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[11px]">Restore</Label>
                    <div className="relative">
                      <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{currencySymbol(priceCurrency || 'USD')}</span>
                      <Input inputMode="decimal" value={priceRestore} onChange={(e) => setPriceRestore(e.target.value)} placeholder="50.00" className="text-sm pl-7" />
                    </div>
                  </div>
                </div>
                {priceError && <p className="text-xs text-destructive">{priceError}</p>}
                <div className="flex gap-2">
                  <Button size="sm" onClick={handleAddPrice} className="gap-1.5">
                    <DollarSign className="h-3.5 w-3.5" />
                    {state.prices.length > 0 ? 'Add another currency' : 'Add price'}
                  </Button>
                  {state.prices.length > 0 && (
                    <Button size="sm" variant="outline" onClick={() => dispatch({ type: 'CONTINUE_TO_FEES' })} className="gap-1.5">
                      <ArrowRight className="h-3.5 w-3.5" /> Continue
                    </Button>
                  )}
                </div>
              </div>
            </>
          )}

          {pastStep('pricing') && state.prices.length > 0 && !(state.step === 'pricing') && (
            <UserBubble>
              {state.prices.length} price{state.prices.length !== 1 ? 's' : ''} configured ({state.prices.map(p => p.currency).join(', ')})
            </UserBubble>
          )}

          {/* ── Step: Fee Offer ───────────────────────────────────── */}
          {state.step === 'fee-offer' && (
            <>
              <SystemBubble icon={Tag}>
                Would you like to add any fees? <span className="text-muted-foreground">e.g., application fee, premium surcharge.</span>
              </SystemBubble>
              <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 flex gap-2 ml-10">
                <Button size="sm" onClick={() => dispatch({ type: 'OFFER_FEE' })} className="gap-1.5">
                  <Tag className="h-3.5 w-3.5" /> Add a fee
                </Button>
                <Button size="sm" variant="outline" onClick={() => dispatch({ type: 'SKIP_FEES' })}>
                  Skip
                </Button>
              </div>
            </>
          )}

          {/* ── Step: Fee Entry ───────────────────────────────────── */}
          {state.step === 'fee-entry' && (
            <>
              {state.fees.length > 0 && (
                <>
                  {state.fees.map((f, i) => (
                    <UserBubble key={i}>
                      <span className="font-mono text-xs">{f.name}</span> — {formatCurrency(f.amount, f.currency)} {f.currency} {f.refundable ? '(refundable)' : ''}
                    </UserBubble>
                  ))}
                  <SystemBubble icon={Tag}>
                    Fee added! Add another or continue.
                  </SystemBubble>
                </>
              )}

              {state.fees.length === 0 && (
                <SystemBubble icon={Tag}>
                  Configure the fee.
                </SystemBubble>
              )}

              <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 space-y-3">
                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1">
                    <Label className="text-[11px]">Fee Name</Label>
                    <Input value={feeName} onChange={(e) => setFeeName(e.target.value)} placeholder="application" className="text-sm" />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[11px]">Currency</Label>
                    <Input value={feeCurrency} onChange={(e) => setFeeCurrency(e.target.value.toUpperCase())} placeholder="USD" className="text-sm font-mono" maxLength={3} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-[11px]">Amount</Label>
                    <div className="relative">
                      <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{currencySymbol(feeCurrency || 'USD')}</span>
                      <Input inputMode="decimal" value={feeAmount} onChange={(e) => setFeeAmount(e.target.value)} placeholder="50.00" className="text-sm pl-7" />
                    </div>
                  </div>
                  <div className="flex items-end gap-2 pb-1">
                    <Switch id="refundable" checked={feeRefundable} onCheckedChange={setFeeRefundable} />
                    <Label htmlFor="refundable" className="text-[11px]">Refundable</Label>
                  </div>
                </div>
                {feeError && <p className="text-xs text-destructive">{feeError}</p>}
                <div className="flex gap-2">
                  <Button size="sm" onClick={handleAddFee} className="gap-1.5">
                    <Tag className="h-3.5 w-3.5" />
                    {state.fees.length > 0 ? 'Add another fee' : 'Add fee'}
                  </Button>
                  {state.fees.length > 0 && (
                    <Button size="sm" variant="outline" onClick={() => dispatch({ type: 'CONTINUE_TO_VALIDATION' })} className="gap-1.5">
                      <ArrowRight className="h-3.5 w-3.5" /> Continue
                    </Button>
                  )}
                </div>
              </div>
            </>
          )}

          {/* ── Step: Validation ────────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('validation') && (
            <SystemBubble icon={Shield}>
              Require validation for registrations? <span className="text-muted-foreground">When enabled, new domains enter <code className="text-[11px] bg-muted px-1 rounded">pendingCreate</code> status until manually approved.</span>
            </SystemBubble>
          )}

          {state.step === 'validation' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 flex gap-2 ml-10">
              <Button size="sm" variant="outline" onClick={() => dispatch({ type: 'SET_VALIDATION', requires: false })} className="gap-1.5">
                No — auto-approve
              </Button>
              <Button size="sm" variant="outline" onClick={() => dispatch({ type: 'SET_VALIDATION', requires: true })} className="gap-1.5">
                <Shield className="h-3.5 w-3.5" />
                Yes — require validation
              </Button>
            </div>
          )}

          {pastStep('validation') && (
            <UserBubble>
              {state.policy.requiresValidation ? 'Validation required' : 'No validation (auto-approve)'}
            </UserBubble>
          )}

          {/* ── Step: Data Policy ────────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('data-policy') && (
            <SystemBubble icon={ScrollText}>
              Contact data policy — which contact types should be collected?
            </SystemBubble>
          )}

          {state.step === 'data-policy' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 space-y-3">
              <div className="grid grid-cols-1 gap-2">
                <button
                  onClick={() => dispatch({ type: 'SET_DATA_POLICY', preset: 'thick', registrant: 'mandatory', tech: 'mandatory', admin: 'optional', billing: 'optional' })}
                  className="text-left rounded-lg border-2 border-primary/30 bg-primary/5 hover:border-primary/50 p-3 transition-all duration-200 group"
                >
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-semibold group-hover:text-primary">Thick (recommended)</div>
                    <Badge variant="secondary" className="text-[10px]">2025 gTLD RDP</Badge>
                  </div>
                  <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                    Registrant &amp; Tech required, Admin &amp; Billing optional.
                  </p>
                </button>
                <button
                  onClick={() => dispatch({ type: 'SET_DATA_POLICY', preset: 'thin', registrant: 'mandatory', tech: 'prohibited', admin: 'prohibited', billing: 'prohibited' })}
                  className="text-left rounded-lg border-2 border-border/60 hover:border-primary/50 hover:bg-primary/5 p-3 transition-all duration-200 group"
                >
                  <div className="text-sm font-semibold text-muted-foreground group-hover:text-primary">Thin</div>
                  <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                    Registrant required only. Tech, Admin &amp; Billing rejected.
                  </p>
                </button>
                <button
                  onClick={() => setShowCustomPolicy(true)}
                  className="text-left rounded-lg border-2 border-border/60 hover:border-primary/50 hover:bg-primary/5 p-3 transition-all duration-200 group"
                >
                  <div className="text-sm font-semibold text-muted-foreground group-hover:text-primary">Custom</div>
                  <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                    Set each contact type individually.
                  </p>
                </button>
              </div>

              {showCustomPolicy && (
                <div className="animate-in fade-in slide-in-from-bottom-2 duration-200 rounded-lg border bg-muted/30 p-3 space-y-2.5">
                  {(['registrant', 'tech', 'admin', 'billing'] as const).map((role) => (
                    <div key={role} className="flex items-center justify-between">
                      <Label className="text-xs capitalize">{role}</Label>
                      <div className="flex gap-1">
                        {(['mandatory', 'optional', 'prohibited'] as const).map((val) => (
                          <button
                            key={val}
                            onClick={() => {
                              const key = `custom${role.charAt(0).toUpperCase() + role.slice(1)}` as keyof typeof customContactPolicy;
                              setCustomContactPolicy(prev => ({ ...prev, [key]: val }));
                            }}
                            className={cn(
                              'px-2 py-0.5 rounded text-[10px] font-medium border transition-colors',
                              customContactPolicy[`custom${role.charAt(0).toUpperCase() + role.slice(1)}` as keyof typeof customContactPolicy] === val
                                ? 'border-primary bg-primary/10 text-primary'
                                : 'border-border text-muted-foreground hover:border-primary/30'
                            )}
                          >
                            {val}
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                  <Button
                    size="sm"
                    className="w-full mt-2 gap-1.5"
                    onClick={() => {
                      dispatch({
                        type: 'SET_DATA_POLICY',
                        preset: 'custom',
                        registrant: customContactPolicy.customRegistrant,
                        tech: customContactPolicy.customTech,
                        admin: customContactPolicy.customAdmin,
                        billing: customContactPolicy.customBilling,
                      });
                      setShowCustomPolicy(false);
                    }}
                  >
                    <ArrowRight className="h-3.5 w-3.5" /> Confirm policy
                  </Button>
                </div>
              )}
            </div>
          )}

          {pastStep('data-policy') && (
            <UserBubble>
              {state.policy.contactPreset === 'thick' ? 'Thick data policy' :
               state.policy.contactPreset === 'thin' ? 'Thin data policy' :
               `Custom: Reg=${state.policy.registrantPolicy}, Tech=${state.policy.techPolicy}, Admin=${state.policy.adminPolicy}, Billing=${state.policy.billingPolicy}`}
            </UserBubble>
          )}

          {/* ── Step: Lifecycle ──────────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('lifecycle') && (
            <SystemBubble icon={Settings2}>
              Domain lifecycle — use the standard gTLD defaults?
              <span className="text-muted-foreground block mt-1 text-[11px] leading-relaxed">
                5-day add/renew/transfer grace, 45-day auto-renew, 30-day redemption, 5-day pending delete, 60-day transfer lock, 10-year max horizon.
              </span>
            </SystemBubble>
          )}

          {state.step === 'lifecycle' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 space-y-3">
              <div className="flex gap-2">
                <Button size="sm" onClick={() => dispatch({ type: 'SET_LIFECYCLE', policy: { useDefaultLifecycle: true } })} className="gap-1.5">
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  Use defaults
                </Button>
                <Button size="sm" variant="outline" onClick={() => setShowCustomLifecycle(true)} className="gap-1.5">
                  <Settings2 className="h-3.5 w-3.5" />
                  Customize
                </Button>
              </div>

              {showCustomLifecycle && (
                <div className="animate-in fade-in slide-in-from-bottom-2 duration-200 rounded-lg border bg-muted/30 p-3 space-y-2.5">
                  <div className="grid grid-cols-2 gap-2">
                    {([
                      { label: 'Add GP', key: 'registrationGP' as const, unit: 'days' },
                      { label: 'Renew GP', key: 'renewalGP' as const, unit: 'days' },
                      { label: 'Auto-renew GP', key: 'autoRenewalGP' as const, unit: 'days' },
                      { label: 'Transfer GP', key: 'transferGP' as const, unit: 'days' },
                      { label: 'Redemption GP', key: 'redemptionGP' as const, unit: 'days' },
                      { label: 'Pending delete', key: 'pendingDeleteGP' as const, unit: 'days' },
                      { label: 'Transfer lock', key: 'transferLockPeriod' as const, unit: 'days' },
                      { label: 'Max horizon', key: 'maxHorizon' as const, unit: 'years' },
                    ]).map(({ label, key, unit }) => (
                      <div key={key} className="space-y-0.5">
                        <Label className="text-[10px] text-muted-foreground">{label} ({unit})</Label>
                        <Input
                          type="number"
                          value={customLifecycle[key]}
                          onChange={(e) => setCustomLifecycle(prev => ({ ...prev, [key]: parseInt(e.target.value, 10) || 0 }))}
                          className="text-sm h-8"
                        />
                      </div>
                    ))}
                  </div>
                  <div className="flex items-center gap-2 pt-1">
                    <Switch
                      id="allowAutoRenew"
                      checked={customLifecycle.allowAutoRenew}
                      onCheckedChange={(v) => setCustomLifecycle(prev => ({ ...prev, allowAutoRenew: v }))}
                    />
                    <Label htmlFor="allowAutoRenew" className="text-xs">Allow auto-renewal</Label>
                  </div>
                  <Button
                    size="sm"
                    className="w-full mt-1 gap-1.5"
                    onClick={() => {
                      dispatch({ type: 'SET_LIFECYCLE', policy: { ...customLifecycle, useDefaultLifecycle: false } });
                      setShowCustomLifecycle(false);
                    }}
                  >
                    <ArrowRight className="h-3.5 w-3.5" /> Confirm lifecycle
                  </Button>
                </div>
              )}
            </div>
          )}

          {pastStep('lifecycle') && (
            <UserBubble>
              {state.policy.useDefaultLifecycle ? 'Standard gTLD lifecycle' : 'Custom lifecycle configured'}
            </UserBubble>
          )}

          {/* ── Step: Review ──────────────────────────────────────── */}
          {currentIndex >= stepOrder.indexOf('review') && state.step !== 'submitting' && state.step !== 'done' && (
            <>
              <SystemBubble icon={CheckCircle2}>
                Here&apos;s what we&apos;ll create:
              </SystemBubble>

              <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10">
                <div className="rounded-lg border bg-muted/30 p-4 space-y-3 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">TLD</span>
                    <span className="font-mono font-medium">.{tldName}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Phase</span>
                    <span className="font-medium">{state.phaseName}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Type</span>
                    <Badge variant={state.phaseType === 'GA' ? 'default' : 'secondary'} className="text-xs">
                      {state.phaseType}
                    </Badge>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Starts</span>
                    <span className="text-xs">{state.startIsNow ? 'Now (on save)' : state.starts && `${format(state.starts, 'PPP HH:mm')} UTC`}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Ends</span>
                    <span className="text-xs">{state.openEnded ? 'Open-ended' : state.ends && `${format(state.ends, 'PPP HH:mm')} UTC`}</span>
                  </div>
                  {state.prices.length > 0 && (
                    <div className="border-t pt-2 space-y-1">
                      <span className="text-muted-foreground text-xs font-medium">Pricing</span>
                      {state.prices.map((p, i) => (
                        <div key={i} className="flex items-center justify-between text-xs">
                          <span className="font-mono">{p.currency}</span>
                          <span>Reg {formatCurrency(p.registrationAmount, p.currency)} · Renew {formatCurrency(p.renewalAmount, p.currency)}</span>
                        </div>
                      ))}
                    </div>
                  )}
                  {state.fees.length > 0 && (
                    <div className="border-t pt-2 space-y-1">
                      <span className="text-muted-foreground text-xs font-medium">Fees</span>
                      {state.fees.map((f, i) => (
                        <div key={i} className="flex items-center justify-between text-xs">
                          <span>{f.name}</span>
                          <span>{formatCurrency(f.amount, f.currency)} {f.currency} {f.refundable ? '(refundable)' : ''}</span>
                        </div>
                      ))}
                    </div>
                  )}
                  <div className="border-t pt-2 space-y-1">
                    <span className="text-muted-foreground text-xs font-medium">Policy</span>
                    <div className="flex items-center justify-between text-xs">
                      <span>Validation</span>
                      <span>{state.policy.requiresValidation ? 'Required' : 'Auto-approve'}</span>
                    </div>
                    <div className="flex items-center justify-between text-xs">
                      <span>Contact data</span>
                      <span className="capitalize">{state.policy.contactPreset}</span>
                    </div>
                    <div className="flex items-center justify-between text-xs">
                      <span>Lifecycle</span>
                      <span>{state.policy.useDefaultLifecycle ? 'Standard gTLD' : 'Custom'}</span>
                    </div>
                  </div>
                </div>

                {state.error && (
                  <div className="mt-2 flex items-center gap-2 text-xs text-destructive">
                    <AlertTriangle className="h-3 w-3 shrink-0" />
                    {state.error}
                  </div>
                )}

                <Button
                  className="mt-3 w-full gap-1.5"
                  onClick={handleSubmit}
                >
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  Create Phase
                </Button>
              </div>
            </>
          )}

          {/* ── Step: Submitting ──────────────────────────────────── */}
          {state.step === 'submitting' && (
            <SystemBubble>
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                Creating phase…
              </div>
            </SystemBubble>
          )}

          {/* ── Step: Done ────────────────────────────────────────── */}
          {state.step === 'done' && (
            <>
              <SystemBubble icon={CheckCircle2}>
                <p className="font-medium text-foreground">Phase &quot;{state.createdPhaseName}&quot; created!</p>
                <p className="text-muted-foreground mt-0.5">What&apos;s next?</p>
              </SystemBubble>

              {state.pricingWarning && (
                <div className="ml-10 flex items-start gap-2 text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/30 rounded-lg p-2.5 border border-amber-200 dark:border-amber-800">
                  <AlertTriangle className="h-3.5 w-3.5 shrink-0 mt-0.5" />
                  <div>
                    <p className="font-medium">Some settings could not be saved:</p>
                    <p className="mt-0.5 opacity-80">{state.pricingWarning}</p>
                    <p className="mt-1 opacity-60">You can configure these from the phase detail view.</p>
                  </div>
                </div>
              )}

              <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 ml-10 flex flex-col gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  className="gap-1.5 justify-start"
                  onClick={() => {
                    router.push(`/tlds/${tldName}?tab=phases&phase=${state.createdPhaseName}`);
                    onClose();
                  }}
                >
                  <Layers className="h-3.5 w-3.5" />
                  View phase
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="gap-1.5 justify-start"
                  onClick={() => {
                    dispatch({ type: 'RESET' });
                    setNameInput('');
                    setNameError(null);
                  }}
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                  Create another phase
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={onClose}
                  className="text-muted-foreground justify-start"
                >
                  Done
                </Button>
              </div>
            </>
          )}
        </div>

        {/* Progress indicator */}
        <ConversationProgress
          steps={PROGRESS_STEPS}
          currentIndex={progressIndex}
        />
      </DialogContent>
    </Dialog>
  );
}
