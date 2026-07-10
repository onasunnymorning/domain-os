'use client';

import { useReducer, useRef, useEffect, useState, useCallback } from 'react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';
import {
  Radio,
  Globe,
  ArrowRight,
  Loader2,
  Sparkles,
  CheckCircle2,
  ChevronsUpDown,
  Check,
  RotateCcw,
  Antenna,
  Server,
  CalendarClock,
} from 'lucide-react';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { launchWorkflow, type WorkflowMeta } from '@/lib/api/workflows';
import { createSlaving } from '@/lib/api/zone-slavings';
import type { WorkflowRun } from '@/lib/stores/useWorkflowStore';
import posthog from 'posthog-js';

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

type Step = 'operator' | 'zone' | 'masters' | 'slaves' | 'mode' | 'submitting' | 'done';

interface WizardState {
  step: Step;
  tenantId: string;
  tenantLabel: string;
  zone: string;
  masterNS: string[];
  slaveNS: string[];
  schedule: boolean;
  intervalMinutes: number;
  error: string | null;
  resultWorkflowId: string | null;
  resultRunId: string | null;
  resultUrl: string | null;
}

type WizardAction =
  | { type: 'SET_OPERATOR'; tenantId: string; label: string }
  | { type: 'SET_ZONE'; zone: string }
  | { type: 'SET_MASTERS'; masterNS: string[] }
  | { type: 'SET_SLAVES'; slaveNS: string[] }
  | { type: 'SET_MODE'; schedule: boolean; intervalMinutes: number }
  | { type: 'SUBMIT' }
  | { type: 'SUBMIT_SUCCESS'; workflowId: string; runId: string; url: string }
  | { type: 'SUBMIT_ERROR'; error: string }
  | { type: 'RESET' };

const initialState: WizardState = {
  step: 'operator',
  tenantId: '',
  tenantLabel: '',
  zone: '',
  masterNS: [],
  slaveNS: [],
  schedule: false,
  intervalMinutes: 5,
  error: null,
  resultWorkflowId: null,
  resultRunId: null,
  resultUrl: null,
};

function reducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'SET_OPERATOR':
      return { ...state, step: 'zone', tenantId: action.tenantId, tenantLabel: action.label, error: null };
    case 'SET_ZONE':
      return { ...state, step: 'masters', zone: action.zone, error: null };
    case 'SET_MASTERS':
      return { ...state, step: 'slaves', masterNS: action.masterNS, error: null };
    case 'SET_SLAVES':
      return { ...state, step: 'mode', slaveNS: action.slaveNS, error: null };
    case 'SET_MODE':
      return { ...state, step: 'submitting', schedule: action.schedule, intervalMinutes: action.intervalMinutes, error: null };
    case 'SUBMIT':
      return { ...state, step: 'submitting', error: null };
    case 'SUBMIT_SUCCESS':
      return { ...state, step: 'done', resultWorkflowId: action.workflowId, resultRunId: action.runId, resultUrl: action.url };
    case 'SUBMIT_ERROR':
      return { ...state, step: 'mode', error: action.error };
    case 'RESET':
      return initialState;
    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// Chat bubble components (shared pattern with TLDCreateDialog)
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
// NS parsing helper
// ---------------------------------------------------------------------------

function parseNSList(raw: string): string[] {
  return raw
    .split(/[,\s]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

interface SerialDriftDialogProps {
  workflow: WorkflowMeta | null;
  onClose: () => void;
  onLaunched: (run: WorkflowRun) => void;
}

export function SerialDriftDialog({ workflow, onClose, onLaunched }: SerialDriftDialogProps) {
  const [state, dispatch] = useReducer(reducer, initialState);

  const { data: operatorsData, isLoading: isLoadingOperators } = useRegistryOperators({ pagesize: 100 });
  const operators = operatorsData?.Data ?? [];

  // Per-step local input
  const [zoneInput, setZoneInput] = useState('');
  const [mastersInput, setMastersInput] = useState('');
  const [slavesInput, setSlavesInput] = useState('');
  const [scheduleToggle, setScheduleToggle] = useState(false);
  const [intervalMinutes, setIntervalMinutes] = useState('5');
  const [operatorOpen, setOperatorOpen] = useState(false);
  const [selectedTenantId, setSelectedTenantId] = useState('');

  // Refs for auto-focus
  const zoneRef = useRef<HTMLInputElement>(null);
  const mastersRef = useRef<HTMLInputElement>(null);
  const slavesRef = useRef<HTMLInputElement>(null);

  // Scroll container ref
  const scrollRef = useRef<HTMLDivElement>(null);

  // Auto-scroll on step change
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [state.step, state.error]);

  // Reset when dialog opens
  useEffect(() => {
    if (workflow) {
      dispatch({ type: 'RESET' });
      setZoneInput('');
      setMastersInput('');
      setSlavesInput('');
      setScheduleToggle(false);
      setIntervalMinutes('5');
      setSelectedTenantId('');
    }
  }, [workflow]);

  // Auto-focus on step transitions
  useEffect(() => {
    const timer = setTimeout(() => {
      if (state.step === 'zone') zoneRef.current?.focus();
      if (state.step === 'masters') mastersRef.current?.focus();
      if (state.step === 'slaves') slavesRef.current?.focus();
    }, 150);
    return () => clearTimeout(timer);
  }, [state.step]);

  const handleSelectOperator = useCallback(() => {
    if (!selectedTenantId) return;
    const op = operators.find((o) => o.RyID === selectedTenantId);
    const label = op ? `${op.RyID} — ${op.Name}` : selectedTenantId;
    dispatch({ type: 'SET_OPERATOR', tenantId: selectedTenantId, label });
  }, [selectedTenantId, operators]);

  const handleZoneSubmit = useCallback(() => {
    const zone = zoneInput.trim().toLowerCase();
    if (!zone) return;
    dispatch({ type: 'SET_ZONE', zone });
  }, [zoneInput]);

  const handleMastersSubmit = useCallback(() => {
    const ns = parseNSList(mastersInput);
    if (ns.length === 0) return;
    dispatch({ type: 'SET_MASTERS', masterNS: ns });
  }, [mastersInput]);

  const handleSlavesSubmit = useCallback(() => {
    const ns = parseNSList(slavesInput);
    if (ns.length === 0) return;
    dispatch({ type: 'SET_SLAVES', slaveNS: ns });
  }, [slavesInput]);

  const handleModeSubmit = useCallback(async () => {
    const interval = parseInt(intervalMinutes, 10) || 5;
    dispatch({ type: 'SET_MODE', schedule: scheduleToggle, intervalMinutes: interval });

    try {
      if (scheduleToggle) {
        // Create persistent slaving monitor + Temporal schedule
        const record = await createSlaving(state.tenantId, {
          zone: state.zone,
          masterNS: state.masterNS,
          slaveNS: state.slaveNS,
          checkIntervalSeconds: interval * 60,
        });

        posthog.capture('serial_drift_schedule_created', {
          tenant_id: state.tenantId,
          zone: state.zone,
          interval_minutes: interval,
        });

        // Use the record ID as the workflow ID for the run tracker
        dispatch({
          type: 'SUBMIT_SUCCESS',
          workflowId: `zone-slaving-${record.id}`,
          runId: record.id,
          url: '',
        });

        onLaunched({
          workflowId: `zone-slaving-${record.id}`,
          runId: record.id,
          type: 'serial-drift',
          displayName: `Serial Drift — ${state.zone}`,
          status: 'RUNNING',
          temporalUrl: '',
          startedAt: new Date().toISOString(),
          params: { zone: state.zone, schedule: true, intervalMinutes: interval },
        });
      } else {
        // One-shot: launch workflow directly
        const result = await launchWorkflow('serial-drift', {
          tenantId: state.tenantId,
          zone: state.zone,
          masterNS: state.masterNS,
          slaveNS: state.slaveNS,
        });

        posthog.capture('serial_drift_launched', {
          tenant_id: state.tenantId,
          zone: state.zone,
          workflow_id: result.workflowId,
        });

        dispatch({
          type: 'SUBMIT_SUCCESS',
          workflowId: result.workflowId,
          runId: result.runId,
          url: result.url,
        });

        onLaunched({
          workflowId: result.workflowId,
          runId: result.runId,
          type: 'serial-drift',
          displayName: `Serial Drift — ${state.zone}`,
          status: 'RUNNING',
          temporalUrl: result.url,
          startedAt: new Date().toISOString(),
          params: { zone: state.zone, schedule: false },
        });
      }

      toast.success(
        scheduleToggle
          ? `Schedule created for ${state.zone}`
          : `Serial drift check launched for ${state.zone}`,
      );
    } catch (error: any) {
      posthog.captureException(error);
      const message =
        error?.response?.data?.error || error?.message || 'Failed to launch serial drift check';
      dispatch({ type: 'SUBMIT_ERROR', error: message });
      toast.error('Launch failed', { description: message });
    }
  }, [scheduleToggle, intervalMinutes, state, onLaunched]);

  // Submit when we enter the submitting step (triggered by SET_MODE)
  // The actual submission is handled inline in handleModeSubmit.

  const stepOrder: Step[] = ['operator', 'zone', 'masters', 'slaves', 'mode', 'submitting', 'done'];
  const currentIndex = stepOrder.indexOf(state.step);
  const pastStep = (s: Step) => stepOrder.indexOf(s) < currentIndex;

  const handleClose = () => {
    if (state.step === 'submitting') return;
    onClose();
  };

  return (
    <Dialog open={workflow !== null} onOpenChange={(v) => {
      if (state.step === 'submitting') return;
      if (!v) handleClose();
    }}>
      <DialogContent className="sm:max-w-md p-0 gap-0 overflow-hidden" showCloseButton={state.step !== 'submitting'}>
        {/* Header */}
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-border/50">
          <DialogTitle className="flex items-center gap-2 text-base">
            <Radio className="h-4 w-4 text-primary" />
            Check Zone Slaving
          </DialogTitle>
          <DialogDescription className="sr-only">
            Configure and launch a serial drift check for zone slaving confidence
          </DialogDescription>
        </DialogHeader>

        {/* Chat area */}
        <div
          ref={scrollRef}
          className="px-6 py-5 space-y-4 max-h-[420px] overflow-y-auto scroll-smooth"
        >
          {/* Step 1: Registry Operator */}
          <SystemBubble icon={Globe}>
            Which registry operator owns this zone?
          </SystemBubble>

          {pastStep('operator') && (
            <UserBubble>{state.tenantLabel}</UserBubble>
          )}

          {state.step === 'operator' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-2">
              <div className="flex gap-2">
                <Popover open={operatorOpen} onOpenChange={setOperatorOpen}>
                  <PopoverTrigger asChild>
                    <Button
                      variant="outline"
                      role="combobox"
                      aria-expanded={operatorOpen}
                      className={cn(
                        'flex-1 justify-between font-normal',
                        !selectedTenantId && 'text-muted-foreground'
                      )}
                    >
                      {selectedTenantId
                        ? (() => {
                            const op = operators.find((o) => o.RyID === selectedTenantId);
                            return op ? `${op.RyID} — ${op.Name}` : selectedTenantId;
                          })()
                        : 'Search operators…'}
                      <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
                    <Command>
                      <CommandInput placeholder="Search by RyID or name…" />
                      <CommandList>
                        <CommandEmpty>
                          {isLoadingOperators ? 'Loading…' : 'No operators found.'}
                        </CommandEmpty>
                        <CommandGroup>
                          {operators.map((op) => (
                            <CommandItem
                              key={op.RyID}
                              value={`${op.RyID} ${op.Name}`}
                              onSelect={() => {
                                setSelectedTenantId(op.RyID);
                                setOperatorOpen(false);
                              }}
                            >
                              <Check
                                className={cn(
                                  'mr-2 h-4 w-4',
                                  selectedTenantId === op.RyID ? 'opacity-100' : 'opacity-0'
                                )}
                              />
                              <span className="font-medium">{op.RyID}</span>
                              <span className="text-muted-foreground ml-2">— {op.Name}</span>
                            </CommandItem>
                          ))}
                        </CommandGroup>
                      </CommandList>
                    </Command>
                  </PopoverContent>
                </Popover>
                <Button
                  size="icon"
                  onClick={handleSelectOperator}
                  disabled={!selectedTenantId}
                  aria-label="Next step"
                >
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {/* Step 2: Zone */}
          {(pastStep('zone') || state.step === 'zone') && (
            <SystemBubble icon={Antenna}>
              What zone are you slaving?
            </SystemBubble>
          )}

          {pastStep('zone') && (
            <UserBubble>
              <span className="font-mono">{state.zone}</span>
            </UserBubble>
          )}

          {state.step === 'zone' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-2">
              <div className="flex gap-2">
                <Input
                  ref={zoneRef}
                  value={zoneInput}
                  onChange={(e) => setZoneInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') { e.preventDefault(); handleZoneSubmit(); }
                  }}
                  placeholder="example.com"
                  className="flex-1 font-mono"
                />
                <Button
                  size="icon"
                  onClick={handleZoneSubmit}
                  disabled={!zoneInput.trim()}
                  aria-label="Next step"
                >
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {/* Step 3: Master NS */}
          {(pastStep('masters') || state.step === 'masters') && (
            <SystemBubble icon={Server}>
              Master nameservers?
              <p className="text-xs text-muted-foreground mt-1">
                Comma-separated, e.g. ns1.master.example, ns2.master.example
              </p>
            </SystemBubble>
          )}

          {pastStep('masters') && (
            <UserBubble>
              <div className="flex flex-wrap gap-1">
                {state.masterNS.map((ns) => (
                  <Badge key={ns} variant="secondary" className="font-mono text-[11px]">{ns}</Badge>
                ))}
              </div>
            </UserBubble>
          )}

          {state.step === 'masters' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-2">
              <div className="flex gap-2">
                <Input
                  ref={mastersRef}
                  value={mastersInput}
                  onChange={(e) => setMastersInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') { e.preventDefault(); handleMastersSubmit(); }
                  }}
                  placeholder="ns1.master.example, ns2.master.example"
                  className="flex-1 font-mono text-xs"
                />
                <Button
                  size="icon"
                  onClick={handleMastersSubmit}
                  disabled={parseNSList(mastersInput).length === 0}
                  aria-label="Next step"
                >
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {/* Step 4: Slave NS */}
          {(pastStep('slaves') || state.step === 'slaves') && (
            <SystemBubble icon={Server}>
              Slave nameservers?
            </SystemBubble>
          )}

          {pastStep('slaves') && (
            <UserBubble>
              <div className="flex flex-wrap gap-1">
                {state.slaveNS.map((ns) => (
                  <Badge key={ns} variant="secondary" className="font-mono text-[11px]">{ns}</Badge>
                ))}
              </div>
            </UserBubble>
          )}

          {state.step === 'slaves' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-2">
              <div className="flex gap-2">
                <Input
                  ref={slavesRef}
                  value={slavesInput}
                  onChange={(e) => setSlavesInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') { e.preventDefault(); handleSlavesSubmit(); }
                  }}
                  placeholder="ns1.slave.example, ns2.slave.example"
                  className="flex-1 font-mono text-xs"
                />
                <Button
                  size="icon"
                  onClick={handleSlavesSubmit}
                  disabled={parseNSList(slavesInput).length === 0}
                  aria-label="Next step"
                >
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {/* Step 5: Mode — one-shot vs schedule */}
          {(pastStep('mode') || state.step === 'mode') && (
            <SystemBubble icon={CalendarClock}>
              <div className="space-y-3">
                <p>How should this run?</p>
                <div className="rounded-lg border border-border/60 bg-muted/30 p-3 space-y-3">
                  <div className="flex items-center justify-between gap-3">
                    <div className="space-y-0.5">
                      <Label htmlFor="schedule-toggle" className="text-xs font-medium cursor-pointer">
                        Set up recurring schedule
                      </Label>
                      <p className="text-[11px] text-muted-foreground">
                        Saves config and monitors until you stop it
                      </p>
                    </div>
                    <Switch
                      id="schedule-toggle"
                      checked={state.step === 'mode' ? scheduleToggle : state.schedule}
                      onCheckedChange={(v) => state.step === 'mode' && setScheduleToggle(!!v)}
                      disabled={state.step !== 'mode'}
                    />
                  </div>
                  {((state.step === 'mode' && scheduleToggle) || (pastStep('mode') && state.schedule)) && (
                    <div className="animate-in fade-in duration-200">
                      <Label htmlFor="interval" className="text-xs text-muted-foreground">
                        Check every
                      </Label>
                      <Select
                        value={state.step === 'mode' ? intervalMinutes : String(state.intervalMinutes)}
                        onValueChange={(v) => state.step === 'mode' && setIntervalMinutes(v)}
                        disabled={state.step !== 'mode'}
                      >
                        <SelectTrigger className="w-full mt-1">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="1">1 minute</SelectItem>
                          <SelectItem value="5">5 minutes</SelectItem>
                          <SelectItem value="10">10 minutes</SelectItem>
                          <SelectItem value="30">30 minutes</SelectItem>
                          <SelectItem value="60">1 hour</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                </div>
              </div>
            </SystemBubble>
          )}

          {state.step === 'mode' && (
            <div className="flex gap-2 animate-in fade-in slide-in-from-bottom-2 duration-300">
              <Button
                size="sm"
                onClick={handleModeSubmit}
                className="gap-1.5"
              >
                <CheckCircle2 className="h-3.5 w-3.5" />
                {scheduleToggle ? `Start monitoring ${state.zone}` : `Check ${state.zone} now`}
              </Button>
            </div>
          )}

          {/* Error on mode step (retry) */}
          {state.step === 'mode' && state.error && (
            <>
              <div className="animate-in fade-in duration-200 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                {state.error}
              </div>
              <div className="flex gap-2">
                <Button size="sm" onClick={handleModeSubmit} className="gap-1.5">
                  <RotateCcw className="h-3.5 w-3.5" />
                  Retry
                </Button>
              </div>
            </>
          )}

          {/* Submitting */}
          {state.step === 'submitting' && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground animate-in fade-in duration-200">
              <Loader2 className="h-4 w-4 animate-spin text-primary" />
              {state.schedule ? 'Creating schedule…' : 'Launching drift check…'}
            </div>
          )}

          {/* Done */}
          {state.step === 'done' && (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-4">
              <div className="flex items-center gap-2 text-sm font-medium text-success">
                <CheckCircle2 className="h-4 w-4" />
                {state.schedule ? 'Schedule created' : 'Drift check launched'}
                {' '}for{' '}
                <Badge variant="secondary" className="font-mono text-xs">
                  {state.zone}
                </Badge>
              </div>

              {state.schedule && (
                <SystemBubble icon={Sparkles}>
                  <p>
                    The system will check every <span className="font-medium">{state.intervalMinutes} min</span> until you stop it.
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">
                    To stop monitoring, use <span className="font-mono">PATCH /zone-slavings/:id</span> with action &quot;abandon&quot;.
                  </p>
                </SystemBubble>
              )}

              {!state.schedule && state.resultUrl && (
                <SystemBubble icon={Sparkles}>
                  <a
                    href={state.resultUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary underline underline-offset-2 hover:text-primary/80"
                  >
                    View in Temporal →
                  </a>
                </SystemBubble>
              )}

              <div className="space-y-3 animate-in fade-in slide-in-from-bottom-2 duration-300 delay-150">
                <div className="flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      dispatch({ type: 'RESET' });
                      setZoneInput('');
                      setMastersInput('');
                      setSlavesInput('');
                      setScheduleToggle(false);
                      setIntervalMinutes('5');
                      setSelectedTenantId('');
                    }}
                    className="gap-1.5"
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                    Check another zone
                  </Button>
                </div>

                <Button
                  size="sm"
                  variant="ghost"
                  onClick={handleClose}
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
            {(['operator', 'zone', 'masters', 'slaves', 'mode'] as Step[]).map((s) => (
              <div
                key={s}
                className={cn(
                  'h-1 flex-1 rounded-full transition-colors duration-300',
                  stepOrder.indexOf(s) < currentIndex
                    ? 'bg-primary'
                    : stepOrder.indexOf(s) === currentIndex
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
