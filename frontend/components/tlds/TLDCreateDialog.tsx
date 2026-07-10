'use client';

import { useReducer, useRef, useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useCreateTLD } from '@/lib/hooks/useTLDs';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';
import {
  Globe,
  Building2,
  CheckCircle2,
  ArrowRight,
  Loader2,
  Sparkles,
  Layers,
  Import,
  AlertTriangle,
  RotateCcw,
  Settings2,
} from 'lucide-react';

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

type Step = 'operator' | 'name' | 'defaults' | 'confirm' | 'submitting' | 'done';

interface WizardState {
  step: Step;
  ryid: string;
  ryidLabel: string; // "ACME - Acme Registry Inc." for display
  name: string;
  tldType: string | null;
  createOperatorRegistrars: boolean;
  allowEscrowImport: boolean;
  error: string | null;
  createdName: string | null;
}

type WizardAction =
  | { type: 'SET_OPERATOR'; ryid: string; label: string }
  | { type: 'SET_NAME'; name: string; tldType: string | null }
  | { type: 'SET_DEFAULTS'; createOperatorRegistrars: boolean; allowEscrowImport: boolean }
  | { type: 'SUBMIT' }
  | { type: 'SUBMIT_SUCCESS'; name: string }
  | { type: 'SUBMIT_ERROR'; error: string }
  | { type: 'RESET' };

const initialState: WizardState = {
  step: 'operator',
  ryid: '',
  ryidLabel: '',
  name: '',
  tldType: null,
  createOperatorRegistrars: true,
  allowEscrowImport: true,
  error: null,
  createdName: null,
};

function reducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'SET_OPERATOR':
      return { ...state, step: 'name', ryid: action.ryid, ryidLabel: action.label, error: null };
    case 'SET_NAME':
      return { ...state, step: 'defaults', name: action.name, tldType: action.tldType, error: null };
    case 'SET_DEFAULTS':
      return {
        ...state,
        step: 'confirm',
        createOperatorRegistrars: action.createOperatorRegistrars,
        allowEscrowImport: action.allowEscrowImport,
        error: null,
      };
    case 'SUBMIT':
      return { ...state, step: 'submitting', error: null };
    case 'SUBMIT_SUCCESS':
      return { ...state, step: 'done', createdName: action.name, error: null };
    case 'SUBMIT_ERROR':
      return { ...state, step: 'confirm', error: action.error };
    case 'RESET':
      return { ...initialState };
    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// TLD type detection
// ---------------------------------------------------------------------------

function detectTLDType(name: string): string | null {
  if (!name) return null;
  if (name.length === 2) return 'country-code';
  if (name.includes('.')) return 'second-level';
  return 'generic';
}

function tldTypeLabel(type: string | null): string {
  switch (type) {
    case 'generic': return 'gTLD';
    case 'country-code': return 'ccTLD';
    case 'second-level': return 'SLD';
    default: return '';
  }
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

function validateName(name: string): string | null {
  const trimmed = name.trim().toLowerCase();
  if (!trimmed) return 'TLD name is required';
  if (trimmed.length > 63) return 'TLD name must not exceed 63 characters';
  if (!/^[a-z0-9]([a-z0-9.-]{0,61}[a-z0-9])?$/i.test(trimmed)) return 'Invalid TLD name format';
  if (trimmed.startsWith('-') || trimmed.endsWith('-')) return 'TLD name cannot start or end with hyphen';
  return null;
}

// ---------------------------------------------------------------------------
// Chat bubble components (shared pattern with ROCreateDialog)
// ---------------------------------------------------------------------------

function SystemBubble({ children, icon: Icon, className }: {
  children: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
  className?: string;
}) {
  return (
    <div className={cn(
      'flex gap-3 items-start animate-in fade-in slide-in-from-bottom-2 duration-300',
      className,
    )}>
      <div className="shrink-0 mt-0.5 flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-primary">
        {Icon ? <Icon className="h-3.5 w-3.5" /> : <Sparkles className="h-3.5 w-3.5" />}
      </div>
      <div className="min-w-0 flex-1 text-sm text-foreground/80">
        {children}
      </div>
    </div>
  );
}

function UserBubble({ children, className }: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn(
      'flex justify-end animate-in fade-in slide-in-from-bottom-2 duration-200',
      className,
    )}>
      <div className="rounded-2xl rounded-br-sm bg-primary/10 border border-primary/20 px-4 py-2 text-sm font-medium text-foreground max-w-[80%]">
        {children}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

interface TLDCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Pre-select a registry operator (e.g., from the RO create dialog) */
  defaultRyId?: string;
}

export function TLDCreateDialog({ open, onOpenChange, defaultRyId }: TLDCreateDialogProps) {
  const [state, dispatch] = useReducer(reducer, initialState);
  const { mutate } = useCreateTLD();
  const router = useRouter();

  // Load operators for the select dropdown
  const { data: operatorsData, isLoading: isLoadingOperators } = useRegistryOperators({ pagesize: 100 });
  const operators = operatorsData?.Data ?? [];

  // Per-step local input
  const [nameInput, setNameInput] = useState('');
  const [nameError, setNameError] = useState<string | null>(null);
  const [selectedRyId, setSelectedRyId] = useState('');

  // Defaults toggle state
  const [createRegistrars, setCreateRegistrars] = useState(true);
  const [allowEscrow, setAllowEscrow] = useState(true);

  // Scroll container ref
  const scrollRef = useRef<HTMLDivElement>(null);

  // Auto-scroll on step change
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [state.step, nameError]);

  // Reset when dialog opens
  useEffect(() => {
    if (open) {
      dispatch({ type: 'RESET' });
      setNameInput('');
      setNameError(null);
      setCreateRegistrars(true);
      setAllowEscrow(true);

      // If a default RyID is provided, pre-select and auto-advance
      if (defaultRyId) {
        setSelectedRyId(defaultRyId);
      } else {
        setSelectedRyId('');
      }
    }
  }, [open, defaultRyId]);

  // Auto-advance when operator is pre-selected and list loads
  useEffect(() => {
    if (open && defaultRyId && operators.length > 0 && state.step === 'operator') {
      const op = operators.find((o) => o.RyID === defaultRyId);
      if (op) {
        dispatch({ type: 'SET_OPERATOR', ryid: op.RyID, label: `${op.RyID} — ${op.Name}` });
      }
    }
  }, [open, defaultRyId, operators, state.step]);

  const handleSelectOperator = useCallback(() => {
    if (!selectedRyId) return;
    const op = operators.find((o) => o.RyID === selectedRyId);
    const label = op ? `${op.RyID} — ${op.Name}` : selectedRyId;
    dispatch({ type: 'SET_OPERATOR', ryid: selectedRyId, label });
  }, [selectedRyId, operators]);

  const handleNameSubmit = useCallback(() => {
    const name = nameInput.trim().toLowerCase();
    const err = validateName(name);
    if (err) { setNameError(err); return; }
    dispatch({ type: 'SET_NAME', name, tldType: detectTLDType(name) });
  }, [nameInput]);

  const handleDefaultsConfirm = useCallback(() => {
    dispatch({ type: 'SET_DEFAULTS', createOperatorRegistrars: createRegistrars, allowEscrowImport: allowEscrow });
  }, [createRegistrars, allowEscrow]);

  const handleSubmit = useCallback(() => {
    dispatch({ type: 'SUBMIT' });

    mutate(
      {
        Name: state.name,
        RyID: state.ryid,
        CreateOperatorRegistrars: state.createOperatorRegistrars,
        AllowEscrowImport: state.allowEscrowImport,
      },
      {
        onSuccess: () => {
          dispatch({ type: 'SUBMIT_SUCCESS', name: state.name });
        },
        onError: (error: any) => {
          const message = error.response?.data?.error || error.message || 'Failed to create TLD';
          dispatch({ type: 'SUBMIT_ERROR', error: message });
        },
      },
    );
  }, [state, mutate]);

  // Immediately submit after defaults are confirmed
  useEffect(() => {
    if (state.step === 'confirm' && !state.error) {
      handleSubmit();
    }
  }, [state.step, state.error, handleSubmit]);

  const stepOrder: Step[] = ['operator', 'name', 'defaults', 'confirm', 'submitting', 'done'];
  const currentIndex = stepOrder.indexOf(state.step);
  const pastStep = (s: Step) => stepOrder.indexOf(s) < currentIndex;

  const nameRef = useRef<HTMLInputElement>(null);

  // Auto-focus name input when step changes to 'name'
  useEffect(() => {
    if (state.step === 'name') {
      const timer = setTimeout(() => nameRef.current?.focus(), 150);
      return () => clearTimeout(timer);
    }
  }, [state.step]);

  return (
    <Dialog open={open} onOpenChange={(v) => {
      if (state.step === 'submitting') return;
      onOpenChange(v);
    }}>
      <DialogContent className="sm:max-w-md p-0 gap-0 overflow-hidden" showCloseButton={state.step !== 'submitting'}>
        {/* Header */}
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-border/50">
          <DialogTitle className="flex items-center gap-2 text-base">
            <Globe className="h-4 w-4 text-primary" />
            New TLD
          </DialogTitle>
          <DialogDescription className="sr-only">
            Create a new top-level domain through a guided conversation
          </DialogDescription>
        </DialogHeader>

        {/* Chat area */}
        <div
          ref={scrollRef}
          className="px-6 py-5 space-y-4 max-h-[420px] overflow-y-auto scroll-smooth"
        >
          {/* Step 1: Registry Operator */}
          <SystemBubble icon={Building2}>
            Which registry operator will manage this TLD?
          </SystemBubble>

          {pastStep('operator') && (
            <UserBubble>{state.ryidLabel}</UserBubble>
          )}

          {state.step === 'operator' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-2">
              <div className="flex gap-2">
                <Select value={selectedRyId} onValueChange={setSelectedRyId}>
                  <SelectTrigger className="flex-1">
                    <SelectValue placeholder="Select operator" />
                  </SelectTrigger>
                  <SelectContent>
                    {isLoadingOperators ? (
                      <SelectItem value="loading" disabled>Loading…</SelectItem>
                    ) : operators.length === 0 ? (
                      <SelectItem value="none" disabled>No operators found</SelectItem>
                    ) : (
                      operators.map((op) => (
                        <SelectItem key={op.RyID} value={op.RyID}>
                          {op.RyID} — {op.Name}
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
                <Button
                  size="icon"
                  onClick={handleSelectOperator}
                  disabled={!selectedRyId}
                  aria-label="Next step"
                >
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {/* Step 2: TLD Name */}
          {(pastStep('name') || state.step === 'name') && (
            <SystemBubble icon={Globe}>
              What&apos;s the TLD name?
              <p className="text-xs text-muted-foreground mt-1">
                e.g., best, radio, co.uk — without the leading dot
              </p>
            </SystemBubble>
          )}

          {pastStep('name') && (
            <UserBubble>
              <span className="font-mono">.{state.name}</span>
              {state.tldType && (
                <Badge variant="outline" className="ml-2 text-[10px]">
                  {tldTypeLabel(state.tldType)}
                </Badge>
              )}
            </UserBubble>
          )}

          {state.step === 'name' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-2">
              <div className="flex gap-2">
                <Input
                  ref={nameRef}
                  value={nameInput}
                  onChange={(e) => { setNameInput(e.target.value); setNameError(null); }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') { e.preventDefault(); handleNameSubmit(); }
                  }}
                  placeholder="best"
                  className="flex-1 font-mono"
                />
                <Button
                  size="icon"
                  onClick={handleNameSubmit}
                  aria-label="Next step"
                >
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
              {nameError && (
                <p className="text-xs text-destructive animate-in fade-in duration-200">{nameError}</p>
              )}
              {nameInput && detectTLDType(nameInput.trim().toLowerCase()) && (
                <p className="text-xs text-muted-foreground">
                  Detected: <Badge variant="outline" className="text-[10px] ml-1">{tldTypeLabel(detectTLDType(nameInput.trim().toLowerCase()))}</Badge>
                </p>
              )}
            </div>
          )}

          {/* Step 3: Defaults */}
          {(pastStep('defaults') || state.step === 'defaults') && (
            <SystemBubble icon={Settings2}>
              <div className="space-y-3">
                <p>These defaults will be applied. Change if needed:</p>
                <div className="rounded-lg border border-border/60 bg-muted/30 p-3 space-y-3">
                  <div className="flex items-start gap-3">
                    <Checkbox
                      id="tld-registrars"
                      checked={state.step === 'defaults' ? createRegistrars : state.createOperatorRegistrars}
                      onCheckedChange={(v) => state.step === 'defaults' && setCreateRegistrars(!!v)}
                      disabled={state.step !== 'defaults'}
                    />
                    <div className="space-y-0.5">
                      <Label htmlFor="tld-registrars" className="text-xs font-medium cursor-pointer">
                        Create operator registrar accounts
                      </Label>
                      <p className="text-[11px] text-muted-foreground">
                        ICANN-reserved 9998/9999 accounts for registry-as-registrar transactions
                      </p>
                    </div>
                  </div>
                  <div className="flex items-start gap-3">
                    <Checkbox
                      id="tld-escrow"
                      checked={state.step === 'defaults' ? allowEscrow : state.allowEscrowImport}
                      onCheckedChange={(v) => state.step === 'defaults' && setAllowEscrow(!!v)}
                      disabled={state.step !== 'defaults'}
                    />
                    <div className="space-y-0.5">
                      <Label htmlFor="tld-escrow" className="text-xs font-medium cursor-pointer">
                        Allow escrow import
                      </Label>
                      <p className="text-[11px] text-muted-foreground">
                        Permit bulk domain data imports from escrow deposits
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </SystemBubble>
          )}

          {state.step === 'defaults' && (
            <div className="flex gap-2 animate-in fade-in slide-in-from-bottom-2 duration-300">
              <Button
                size="sm"
                onClick={handleDefaultsConfirm}
                className="gap-1.5"
              >
                <CheckCircle2 className="h-3.5 w-3.5" />
                Looks good, create .{nameInput.trim().toLowerCase() || state.name}
              </Button>
            </div>
          )}

          {/* Submitting */}
          {state.step === 'submitting' && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground animate-in fade-in duration-200">
              <Loader2 className="h-4 w-4 animate-spin text-primary" />
              Creating .{state.name}…
            </div>
          )}

          {/* Error on confirm (retry) */}
          {state.step === 'confirm' && state.error && (
            <>
              <div className="animate-in fade-in duration-200 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                {state.error}
              </div>
              <div className="flex gap-2">
                <Button size="sm" onClick={handleSubmit} className="gap-1.5">
                  <RotateCcw className="h-3.5 w-3.5" />
                  Retry
                </Button>
              </div>
            </>
          )}

          {/* Step 6: Done */}
          {state.step === 'done' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-4">
              <div className="flex items-center gap-2 text-sm font-medium text-success">
                <CheckCircle2 className="h-4 w-4" />
                Created{' '}
                <Badge variant="secondary" className="font-mono text-xs">
                  .{state.createdName}
                </Badge>
              </div>

              <SystemBubble icon={Sparkles}>
                What&apos;s next?
              </SystemBubble>

              <div className="space-y-3 animate-in fade-in slide-in-from-bottom-2 duration-300 delay-150">
                <div className="flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      onOpenChange(false);
                      router.push(`/tlds/${state.createdName}`);
                    }}
                    className="gap-1.5"
                  >
                    <Layers className="h-3.5 w-3.5" />
                    Add phases for .{state.createdName}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      onOpenChange(false);
                      router.push(`/tlds/${state.createdName}`);
                    }}
                    className="gap-1.5"
                  >
                    <Import className="h-3.5 w-3.5" />
                    Import data
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      dispatch({ type: 'RESET' });
                      setNameInput('');
                      setNameError(null);
                      setSelectedRyId('');
                      setCreateRegistrars(true);
                      setAllowEscrow(true);
                    }}
                    className="gap-1.5"
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                    Create another TLD
                  </Button>
                </div>

                {/* Warning about import vs phases */}
                <div className="flex gap-2 items-start rounded-lg border border-warning/30 bg-warning/5 px-3 py-2">
                  <AlertTriangle className="h-3.5 w-3.5 text-warning shrink-0 mt-0.5" />
                  <p className="text-[11px] text-foreground/70">
                    To import escrow data, the TLD must have <span className="font-medium text-warning">no active phases</span>.
                    Add phases only after the import is complete.
                  </p>
                </div>

                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onOpenChange(false)}
                  className="text-muted-foreground"
                >
                  Done
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* Progress indicator */}
        <div className="px-6 py-3 border-t border-border/50 bg-muted/20">
          <div className="flex gap-1.5">
            {['operator', 'name', 'defaults'].map((s) => (
              <div
                key={s}
                className={cn(
                  'h-1 flex-1 rounded-full transition-colors duration-300',
                  stepOrder.indexOf(s as Step) < currentIndex
                    ? 'bg-primary'
                    : stepOrder.indexOf(s as Step) === currentIndex
                      ? 'bg-primary/50'
                      : 'bg-muted',
                )}
              />
            ))}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
