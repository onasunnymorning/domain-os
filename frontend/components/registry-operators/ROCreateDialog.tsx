'use client';

import { useReducer, useRef, useEffect, useState, useCallback } from 'react';
import { useCreateRegistryOperator } from '@/lib/hooks/useRegistryOperators';
import { generateRyId } from '@/lib/utils/generateRyId';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter } from '@/components/ui/sheet';
import { TLDCreateDialog } from '@/components/tlds/TLDCreateDialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import {
  Building2,
  KeyRound,
  Mail,
  CheckCircle2,
  ArrowRight,
  Loader2,
  Sparkles,
  ListPlus,
  Globe,
  RotateCcw,
} from 'lucide-react';

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

type Step = 'name' | 'ryid' | 'email' | 'confirm' | 'submitting' | 'done';

interface WizardState {
  step: Step;
  name: string;
  ryid: string;
  ryidGenerated: string; // what we proposed
  email: string;
  url: string;
  voice: string;
  fax: string;
  error: string | null;
  createdRyId: string | null;
}

type WizardAction =
  | { type: 'SET_NAME'; name: string }
  | { type: 'SET_RYID'; ryid: string }
  | { type: 'SET_EMAIL'; email: string }
  | { type: 'SET_OPTIONAL'; url: string; voice: string; fax: string }
  | { type: 'SUBMIT' }
  | { type: 'SUBMIT_SUCCESS'; ryid: string }
  | { type: 'SUBMIT_ERROR'; error: string }
  | { type: 'RESET' };

const initialState: WizardState = {
  step: 'name',
  name: '',
  ryid: '',
  ryidGenerated: '',
  email: '',
  url: '',
  voice: '',
  fax: '',
  error: null,
  createdRyId: null,
};

function reducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'SET_NAME': {
      const generated = generateRyId(action.name);
      return {
        ...state,
        step: 'ryid',
        name: action.name,
        ryid: generated,
        ryidGenerated: generated,
        error: null,
      };
    }
    case 'SET_RYID':
      return { ...state, step: 'email', ryid: action.ryid, error: null };
    case 'SET_EMAIL':
      return { ...state, step: 'confirm', email: action.email, error: null };
    case 'SET_OPTIONAL':
      return { ...state, url: action.url, voice: action.voice, fax: action.fax };
    case 'SUBMIT':
      return { ...state, step: 'submitting', error: null };
    case 'SUBMIT_SUCCESS':
      return { ...state, step: 'done', createdRyId: action.ryid, error: null };
    case 'SUBMIT_ERROR':
      return { ...state, step: 'confirm', error: action.error };
    case 'RESET':
      return { ...initialState };
    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

function validateName(name: string): string | null {
  const trimmed = name.trim();
  if (trimmed.length < 3) return 'Name must be at least 3 characters';
  return null;
}

function validateRyId(ryid: string): string | null {
  const trimmed = ryid.trim();
  if (trimmed.length < 3) return 'RyID must be at least 3 characters';
  if (trimmed.length > 16) return 'RyID must not exceed 16 characters';
  if (!/^[\x20-\x7E]+$/.test(trimmed)) return 'RyID must contain only ASCII characters';
  if (trimmed !== ryid) return 'RyID cannot start or end with whitespace';
  return null;
}

function validateEmail(email: string): string | null {
  const trimmed = email.trim();
  if (!trimmed) return 'Email is required';
  // Simple email regex — server validates further
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)) return 'Invalid email address';
  return null;
}

// ---------------------------------------------------------------------------
// Chat bubble components
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

function StepInput({ 
  value, 
  onChange, 
  onSubmit, 
  placeholder, 
  type = 'text',
  error,
  autoFocus = true,
}: {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  placeholder: string;
  type?: string;
  error: string | null;
  autoFocus?: boolean;
}) {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (autoFocus) {
      // Small delay to let animation finish before focusing
      const timer = setTimeout(() => inputRef.current?.focus(), 150);
      return () => clearTimeout(timer);
    }
  }, [autoFocus]);

  return (
    <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-2">
      <div className="flex gap-2">
        <Input
          ref={inputRef}
          type={type}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              onSubmit();
            }
          }}
          placeholder={placeholder}
          className="flex-1"
        />
        <Button
          size="icon"
          onClick={onSubmit}
          className="shrink-0"
          aria-label="Next step"
        >
          <ArrowRight className="h-4 w-4" />
        </Button>
      </div>
      {error && (
        <p className="text-xs text-destructive animate-in fade-in duration-200">{error}</p>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

interface ROCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ROCreateDialog({ open, onOpenChange }: ROCreateDialogProps) {
  const [state, dispatch] = useReducer(reducer, initialState);
  const { mutate, isPending } = useCreateRegistryOperator();

  // TLD create dialog chaining
  const [tldCreateOpen, setTldCreateOpen] = useState(false);
  const [tldDefaultRyId, setTldDefaultRyId] = useState<string | undefined>();

  // Per-step input values (local to the current step, committed on Enter)
  const [inputValue, setInputValue] = useState('');
  const [inputError, setInputError] = useState<string | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);

  // Optional fields for the sheet
  const [optUrl, setOptUrl] = useState('');
  const [optVoice, setOptVoice] = useState('');
  const [optFax, setOptFax] = useState('');

  // Scroll container ref
  const scrollRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom on step change
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [state.step, inputError]);

  // Reset when dialog opens
  useEffect(() => {
    if (open) {
      dispatch({ type: 'RESET' });
      setInputValue('');
      setInputError(null);
      setSheetOpen(false);
      setOptUrl('');
      setOptVoice('');
      setOptFax('');
    }
  }, [open]);

  // Pre-fill input for ryid step
  useEffect(() => {
    if (state.step === 'ryid') {
      setInputValue(state.ryid);
      setInputError(null);
    } else if (state.step === 'email' || state.step === 'name') {
      setInputValue('');
      setInputError(null);
    }
  }, [state.step, state.ryid]);

  const handleSubmit = useCallback(() => {
    const data = {
      RyID: state.ryid,
      Name: state.name,
      Email: state.email,
      ...(state.url && { URL: state.url }),
      ...(state.voice && { Voice: state.voice }),
      ...(state.fax && { Fax: state.fax }),
    };

    dispatch({ type: 'SUBMIT' });

    mutate(data, {
      onSuccess: () => {
        dispatch({ type: 'SUBMIT_SUCCESS', ryid: state.ryid });
        toast.success(`Created registry operator ${state.ryid}`);
      },
      onError: (error: any) => {
        const message = error.response?.data?.error || error.message || 'Failed to create registry operator';
        dispatch({ type: 'SUBMIT_ERROR', error: message });
      },
    });
  }, [state, mutate]);

  const handleStepSubmit = useCallback(() => {
    const value = inputValue.trim();

    switch (state.step) {
      case 'name': {
        const err = validateName(value);
        if (err) { setInputError(err); return; }
        dispatch({ type: 'SET_NAME', name: value });
        break;
      }
      case 'ryid': {
        const err = validateRyId(value);
        if (err) { setInputError(err); return; }
        dispatch({ type: 'SET_RYID', ryid: value });
        break;
      }
      case 'email': {
        const err = validateEmail(value);
        if (err) { setInputError(err); return; }
        dispatch({ type: 'SET_EMAIL', email: value });
        break;
      }
    }
  }, [inputValue, state.step]);

  const handleSheetSubmit = useCallback(() => {
    dispatch({ type: 'SET_OPTIONAL', url: optUrl, voice: optVoice, fax: optFax });
    setSheetOpen(false);
    // Submit after setting optional fields — use a microtask to let reducer settle
    setTimeout(() => {
      // We need the latest state, so we'll trigger submit in a separate effect
    }, 0);
  }, [optUrl, optVoice, optFax]);

  // After sheet closes and optional fields are set, auto-submit
  const [pendingSheetSubmit, setPendingSheetSubmit] = useState(false);

  const handleSheetDone = useCallback(() => {
    dispatch({ type: 'SET_OPTIONAL', url: optUrl, voice: optVoice, fax: optFax });
    setSheetOpen(false);
    setPendingSheetSubmit(true);
  }, [optUrl, optVoice, optFax]);

  useEffect(() => {
    if (pendingSheetSubmit && !sheetOpen && state.step === 'confirm') {
      setPendingSheetSubmit(false);
      handleSubmit();
    }
  }, [pendingSheetSubmit, sheetOpen, state.step, handleSubmit]);

  const stepOrder: Step[] = ['name', 'ryid', 'email', 'confirm', 'submitting', 'done'];
  const currentIndex = stepOrder.indexOf(state.step);

  const pastStep = (s: Step) => stepOrder.indexOf(s) < currentIndex;

  return (
    <>
      <Dialog open={open} onOpenChange={(v) => {
        // Prevent closing during submission
        if (state.step === 'submitting') return;
        onOpenChange(v);
      }}>
        <DialogContent className="sm:max-w-md p-0 gap-0 overflow-hidden" showCloseButton={state.step !== 'submitting'}>
          {/* Header */}
          <DialogHeader className="px-6 pt-6 pb-4 border-b border-border/50">
            <DialogTitle className="flex items-center gap-2 text-base">
              <Building2 className="h-4 w-4 text-primary" />
              New Registry Operator
            </DialogTitle>
            <DialogDescription className="sr-only">
              Create a new registry operator through a guided conversation
            </DialogDescription>
          </DialogHeader>

          {/* Chat area */}
          <div
            ref={scrollRef}
            className="px-6 py-5 space-y-4 max-h-[400px] overflow-y-auto scroll-smooth"
          >
            {/* Step 1: Name */}
            <SystemBubble icon={Building2}>
              What&apos;s the company name?
            </SystemBubble>

            {pastStep('name') && (
              <UserBubble>{state.name}</UserBubble>
            )}

            {state.step === 'name' && (
              <StepInput
                value={inputValue}
                onChange={(v) => { setInputValue(v); setInputError(null); }}
                onSubmit={handleStepSubmit}
                placeholder="Acme Registry Inc."
                error={inputError}
              />
            )}

            {/* Step 2: RyID */}
            {(pastStep('ryid') || state.step === 'ryid') && (
              <SystemBubble icon={KeyRound}>
                <span>
                  How about{' '}
                  <Badge variant="secondary" className="font-mono text-xs">
                    {state.ryidGenerated}
                  </Badge>
                  {' '}as the RyID?
                </span>
                <p className="text-xs text-muted-foreground mt-1">
                  3–16 ASCII characters. Edit below to override.
                </p>
              </SystemBubble>
            )}

            {pastStep('ryid') && (
              <UserBubble>
                <span className="font-mono">{state.ryid}</span>
                {state.ryid !== state.ryidGenerated && (
                  <span className="text-xs text-muted-foreground ml-2">(custom)</span>
                )}
              </UserBubble>
            )}

            {state.step === 'ryid' && (
              <StepInput
                value={inputValue}
                onChange={(v) => { setInputValue(v); setInputError(null); }}
                onSubmit={handleStepSubmit}
                placeholder={state.ryidGenerated}
                error={inputError}
              />
            )}

            {/* Step 3: Email */}
            {(pastStep('email') || state.step === 'email') && (
              <SystemBubble icon={Mail}>
                Main contact email?
              </SystemBubble>
            )}

            {pastStep('email') && (
              <UserBubble>{state.email}</UserBubble>
            )}

            {state.step === 'email' && (
              <StepInput
                value={inputValue}
                onChange={(v) => { setInputValue(v); setInputError(null); }}
                onSubmit={handleStepSubmit}
                placeholder="contact@acme-registry.com"
                type="email"
                error={inputError}
              />
            )}

            {/* Step 4: Confirm */}
            {(state.step === 'confirm' || state.step === 'submitting' || state.step === 'done') && (
              <>
                <SystemBubble icon={ListPlus}>
                  <div className="space-y-3">
                    {/* Summary */}
                    <div className="rounded-lg border border-border/60 bg-muted/30 p-3 space-y-1.5">
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span className="font-medium text-foreground">{state.name}</span>
                      </div>
                      <div className="flex items-center gap-2 text-xs">
                        <Badge variant="outline" className="font-mono text-[11px]">{state.ryid}</Badge>
                        <span className="text-muted-foreground">·</span>
                        <span className="text-muted-foreground">{state.email}</span>
                      </div>
                      {(state.url || state.voice || state.fax) && (
                        <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground pt-1 border-t border-border/40 mt-1.5">
                          {state.url && <span>{state.url}</span>}
                          {state.voice && <span>☎ {state.voice}</span>}
                          {state.fax && <span>📠 {state.fax}</span>}
                        </div>
                      )}
                    </div>

                    <p>Want to add more details?</p>
                  </div>
                </SystemBubble>

                {/* Error from previous submit attempt */}
                {state.error && state.step === 'confirm' && (
                  <div className="animate-in fade-in duration-200 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    {state.error}
                  </div>
                )}
              </>
            )}

            {state.step === 'confirm' && (
              <div className="flex gap-2 animate-in fade-in slide-in-from-bottom-2 duration-300">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setSheetOpen(true)}
                  className="gap-1.5"
                >
                  <ListPlus className="h-3.5 w-3.5" />
                  Add URL, phone, fax
                </Button>
                <Button
                  size="sm"
                  onClick={handleSubmit}
                  className="gap-1.5"
                >
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  Looks good, create
                </Button>
              </div>
            )}

            {/* Step 5: Submitting */}
            {state.step === 'submitting' && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground animate-in fade-in duration-200">
                <Loader2 className="h-4 w-4 animate-spin text-primary" />
                Creating registry operator…
              </div>
            )}

            {/* Step 6: Done */}
            {state.step === 'done' && (
              <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-4">
                <div className="flex items-center gap-2 text-sm font-medium text-success">
                  <CheckCircle2 className="h-4 w-4" />
                  Created{' '}
                  <Badge variant="secondary" className="font-mono text-xs">
                    {state.createdRyId}
                  </Badge>
                </div>

                <SystemBubble icon={Sparkles}>
                  What&apos;s next?
                </SystemBubble>

                <div className="flex flex-wrap gap-2 animate-in fade-in slide-in-from-bottom-2 duration-300 delay-150">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setTldDefaultRyId(state.createdRyId!);
                      setTldCreateOpen(true);
                    }}
                    className="gap-1.5"
                  >
                    <Globe className="h-3.5 w-3.5" />
                    Add a TLD for {state.createdRyId}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      dispatch({ type: 'RESET' });
                      setInputValue('');
                      setInputError(null);
                    }}
                    className="gap-1.5"
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                    Create another
                  </Button>
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
              {['name', 'ryid', 'email', 'confirm'].map((s) => (
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

      {/* Optional fields sheet */}
      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Additional Details</SheetTitle>
            <SheetDescription>
              Optional fields for {state.name}
            </SheetDescription>
          </SheetHeader>

          <div className="space-y-4 px-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="ro-url">Website URL</Label>
              <Input
                id="ro-url"
                value={optUrl}
                onChange={(e) => setOptUrl(e.target.value)}
                placeholder="https://acme-registry.com"
                type="url"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ro-voice">Phone</Label>
              <Input
                id="ro-voice"
                value={optVoice}
                onChange={(e) => setOptVoice(e.target.value)}
                placeholder="+1.1234567890"
              />
              <p className="text-xs text-muted-foreground">E.164 format</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="ro-fax">Fax</Label>
              <Input
                id="ro-fax"
                value={optFax}
                onChange={(e) => setOptFax(e.target.value)}
                placeholder="+1.1234567890"
              />
              <p className="text-xs text-muted-foreground">E.164 format</p>
            </div>
          </div>

          <SheetFooter>
            <Button variant="outline" onClick={() => setSheetOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSheetDone} className="gap-1.5">
              <CheckCircle2 className="h-3.5 w-3.5" />
              Save & Create
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* TLD create dialog (chained from done step) */}
      <TLDCreateDialog
        open={tldCreateOpen}
        onOpenChange={(v) => {
          setTldCreateOpen(v);
          if (!v) setTldDefaultRyId(undefined);
        }}
        defaultRyId={tldDefaultRyId}
      />
    </>
  );
}
