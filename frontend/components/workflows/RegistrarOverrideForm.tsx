'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import {
  Plus,
  Check,
  ChevronsUpDown,
  Loader2,
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  X,
  StopCircle,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { getRegistrars, createRegistrar } from '@/lib/api/registrars';
import { signalWorkflow, terminateWorkflow } from '@/lib/api/workflows';
import type { RegistrarListItem } from '@/lib/types/registrar';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface UnmappedRegistrar {
  escrowId: string;
  name: string;
  gurId: number;
  domainCount: number;
  hostCount: number;
  contactCount: number;
}

interface RegistrarOverrideFormProps {
  workflowId: string;
  unmappedRegistrars: UnmappedRegistrar[];
  onSignalSent: () => void;
}

// ---------------------------------------------------------------------------
// ClID auto-generation (ported from Go)
// ---------------------------------------------------------------------------

function generateClID(gurId: number, name: string): string {
  let slug = name.split(',')[0];
  slug = slug.toLowerCase();
  slug = slug.replace(/[^\x00-\x7F]/g, ''); // remove non-ASCII
  slug = slug.replace(/\s+/g, '-');
  slug = slug.replace(/[^a-z0-9-]/g, ''); // remove non-alphanumeric except dashes
  slug = slug.replace(/\./g, '');
  slug = slug.replace(/^-+|-+$/g, ''); // trim dashes
  slug = `${gurId}-${slug}`;
  if (slug.length > 16) slug = slug.substring(0, 16);
  slug = slug.replace(/^-+|-+$/g, ''); // trim dashes again
  return slug;
}

// ---------------------------------------------------------------------------
// Registrar Search Combobox
// ---------------------------------------------------------------------------

function RegistrarCombobox({
  value,
  onChange,
}: {
  value: string | null;
  onChange: (clid: string, name: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [results, setResults] = useState<RegistrarListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    if (!open) return;

    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      setLoading(true);
      try {
        const baseParams = { status_equals: 'ok' as const, pagesize: 10 };
        const queries: Promise<{ Data: RegistrarListItem[] }>[] = [];

        if (search) {
          // Search by name and ClID in parallel
          queries.push(getRegistrars({ ...baseParams, name_like: search }));
          queries.push(getRegistrars({ ...baseParams, clid_like: search }));
          // If numeric, also try exact GurID match
          const asNum = Number(search);
          if (!isNaN(asNum) && asNum > 0) {
            queries.push(getRegistrars({ ...baseParams, gurid_equals: asNum }));
          }
        } else {
          queries.push(getRegistrars(baseParams));
        }

        const responses = await Promise.all(queries);
        // Merge and deduplicate by ClID
        const seen = new Set<string>();
        const merged: RegistrarListItem[] = [];
        for (const res of responses) {
          for (const r of res.Data || []) {
            if (!seen.has(r.ClID)) {
              seen.add(r.ClID);
              merged.push(r);
            }
          }
        }
        setResults(merged);
      } catch {
        setResults([]);
      } finally {
        setLoading(false);
      }
    }, 250);

    return () => clearTimeout(debounceRef.current);
  }, [search, open]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="h-7 w-full justify-between text-xs font-normal px-2"
        >
          <span className="truncate">
            {value || 'Search registrar…'}
          </span>
          <ChevronsUpDown className="ml-1 size-3 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[320px] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder="Search by name, ClID, or GurID…"
            value={search}
            onValueChange={setSearch}
          />
          <CommandList>
            {loading && (
              <div className="flex items-center justify-center py-4">
                <Loader2 className="size-4 animate-spin text-muted-foreground" />
              </div>
            )}
            {!loading && results.length === 0 && (
              <CommandEmpty>No registrars found.</CommandEmpty>
            )}
            {!loading && (
              <CommandGroup>
                {results.map((r) => (
                  <CommandItem
                    key={r.ClID}
                    value={r.ClID}
                    onSelect={() => {
                      onChange(r.ClID, r.Name);
                      setOpen(false);
                    }}
                    className="text-xs"
                  >
                    <Check
                      className={cn(
                        'mr-1 size-3',
                        value === r.ClID ? 'opacity-100' : 'opacity-0'
                      )}
                    />
                    <span className="truncate">
                      {r.Name} ({r.ClID})
                      <span className="text-muted-foreground"> — GurID: {r.GurID}</span>
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

// ---------------------------------------------------------------------------
// Inline Create Form
// ---------------------------------------------------------------------------

interface CreateFormState {
  clid: string;
  name: string;
  gurId: number;
  email: string;
}

function InlineCreateForm({
  registrar,
  onCreated,
  onCancel,
}: {
  registrar: UnmappedRegistrar;
  onCreated: (clid: string) => void;
  onCancel: () => void;
}) {
  const [form, setForm] = useState<CreateFormState>({
    clid: generateClID(registrar.gurId, registrar.name),
    name: registrar.name,
    gurId: registrar.gurId,
    email: '',
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.email) {
      setError('Email is required');
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await createRegistrar({
        ClID: form.clid,
        Name: form.name,
        Email: form.email,
        GurID: form.gurId,
        Status: 'ok' as any,
        IANAStatus: 'Unknown' as any,
        PostalInfo: [
          {
            Type: 'int',
            Address: { Street: [''], City: 'TBD', CountryCode: 'PE' },
          },
        ],
      });
      toast.success('Registrar created', {
        description: `${form.clid} is now available as an override target`,
      });
      onCreated(form.clid);
    } catch (err: any) {
      const msg =
        err?.response?.data?.error ||
        err?.response?.data?.message ||
        err?.message ||
        'Failed to create registrar';
      setError(msg);
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr>
      <td colSpan={8} className="px-2 py-2 bg-muted/30">
        <form onSubmit={handleSubmit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div className="space-y-1">
              <Label className="text-[10px] text-muted-foreground">ClID</Label>
              <Input
                value={form.clid}
                onChange={(e) => setForm((f) => ({ ...f, clid: e.target.value }))}
                className="h-7 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label className="text-[10px] text-muted-foreground">Name</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                className="h-7 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label className="text-[10px] text-muted-foreground">GurID</Label>
              <Input
                type="number"
                value={form.gurId}
                onChange={(e) =>
                  setForm((f) => ({ ...f, gurId: parseInt(e.target.value, 10) || 0 }))
                }
                className="h-7 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label className="text-[10px] text-muted-foreground">Email *</Label>
              <Input
                type="email"
                value={form.email}
                onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
                placeholder="operator@example.com"
                className="h-7 text-xs"
              />
            </div>
          </div>
          {error && (
            <p className="text-xs text-red-600 dark:text-red-400">{error}</p>
          )}
          <div className="flex items-center gap-2">
            <Button type="submit" size="sm" disabled={saving} className="h-7 text-xs gap-1">
              {saving && <Loader2 className="size-3 animate-spin" />}
              Create Registrar
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onCancel}
              disabled={saving}
              className="h-7 text-xs"
            >
              Cancel
            </Button>
          </div>
        </form>
      </td>
    </tr>
  );
}

// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------

export function RegistrarOverrideForm({
  workflowId,
  unmappedRegistrars,
  onSignalSent,
}: RegistrarOverrideFormProps) {
  const [overrides, setOverrides] = useState<Record<string, string>>({});
  const [expandedCreate, setExpandedCreate] = useState<string | null>(null);
  const [sending, setSending] = useState<'apply' | 'skip' | 'stop' | null>(null);
  const [emptyExpanded, setEmptyExpanded] = useState(false);

  // Partition: registrars with objects vs empty ones
  const withObjects = unmappedRegistrars.filter(
    (r) => r.domainCount > 0 || r.hostCount > 0
  );
  const emptyRegistrars = unmappedRegistrars.filter(
    (r) => r.domainCount === 0 && r.hostCount === 0
  );

  const overrideCount = Object.keys(overrides).length;

  const setOverride = useCallback((escrowName: string, clid: string) => {
    setOverrides((prev) => ({ ...prev, [escrowName]: clid }));
  }, []);

  const removeOverride = useCallback((escrowName: string) => {
    setOverrides((prev) => {
      const next = { ...prev };
      delete next[escrowName];
      return next;
    });
  }, []);

  const handleApply = async () => {
    setSending('apply');
    try {
      await signalWorkflow(workflowId, 'ProvideRegistrarOverrides', overrides);
      toast.success('Registrar overrides sent', {
        description: `${overrideCount} override(s) applied — workflow continuing`,
      });
      onSignalSent();
    } catch (err: any) {
      const msg =
        err?.response?.data?.error ||
        err?.response?.data?.message ||
        err?.message ||
        'Failed to send signal';
      toast.error('Signal failed', { description: msg });
    } finally {
      setSending(null);
    }
  };

  const handleSkip = async () => {
    setSending('skip');
    try {
      await signalWorkflow(workflowId, 'SkipRegistrarOverrides', true);
      toast.success('Overrides skipped', {
        description: 'Workflow continuing without registrar overrides',
      });
      onSignalSent();
    } catch (err: any) {
      const msg =
        err?.response?.data?.error ||
        err?.response?.data?.message ||
        err?.message ||
        'Failed to send signal';
      toast.error('Signal failed', { description: msg });
    } finally {
      setSending(null);
    }
  };

  const handleStop = async () => {
    setSending('stop');
    try {
      await terminateWorkflow(workflowId, 'stopped by operator at registrar override step');
      toast.success('Import stopped', {
        description: 'The workflow has been terminated. You can re-run it when ready.',
      });
      onSignalSent();
    } catch (err: any) {
      const msg =
        err?.response?.data?.error ||
        err?.response?.data?.message ||
        err?.message ||
        'Failed to terminate workflow';
      toast.error('Terminate failed', { description: msg });
    } finally {
      setSending(null);
    }
  };

  if (withObjects.length === 0 && emptyRegistrars.length === 0) {
    return null;
  }

  return (
    <div className="space-y-4">
      {/* Main override table */}
      {withObjects.length > 0 && (
        <div className="overflow-x-auto rounded border">
          <table className="w-full text-xs">
            <thead>
              <tr className="bg-muted/50 text-muted-foreground">
                <th className="text-left px-2 py-1.5 font-medium">Escrow ID</th>
                <th className="text-left px-2 py-1.5 font-medium">Name</th>
                <th className="text-right px-2 py-1.5 font-medium">GurID</th>
                <th className="text-right px-2 py-1.5 font-medium">Domains</th>
                <th className="text-right px-2 py-1.5 font-medium">Hosts</th>
                <th className="text-right px-2 py-1.5 font-medium">Contacts</th>
                <th className="text-left px-2 py-1.5 font-medium min-w-[180px]">
                  Override Target
                </th>
                <th className="px-2 py-1.5 font-medium w-[80px]" />
              </tr>
            </thead>
            <tbody className="divide-y">
              {withObjects.map((r) => {
                const key = r.name;
                const currentOverride = overrides[key];
                const isCreating = expandedCreate === key;

                return (
                  <>
                    <tr
                      key={r.escrowId}
                      className={cn(
                        'hover:bg-muted/20 transition-colors',
                        currentOverride && 'bg-green-500/5'
                      )}
                    >
                      <td className="px-2 py-1.5 font-mono">{r.escrowId}</td>
                      <td className="px-2 py-1.5 truncate max-w-[140px]" title={r.name}>
                        {r.name}
                      </td>
                      <td className="px-2 py-1.5 text-right tabular-nums">{r.gurId}</td>
                      <td className="px-2 py-1.5 text-right tabular-nums">
                        {r.domainCount.toLocaleString()}
                      </td>
                      <td className="px-2 py-1.5 text-right tabular-nums">
                        {r.hostCount.toLocaleString()}
                      </td>
                      <td className="px-2 py-1.5 text-right tabular-nums">
                        {r.contactCount.toLocaleString()}
                      </td>
                      <td className="px-2 py-1.5">
                        <div className="flex items-center gap-1">
                          <div className="flex-1">
                            <RegistrarCombobox
                              value={currentOverride || null}
                              onChange={(clid) => {
                                setOverride(key, clid);
                                if (isCreating) setExpandedCreate(null);
                              }}
                            />
                          </div>
                          {currentOverride && (
                            <button
                              type="button"
                              onClick={() => removeOverride(key)}
                              className="text-muted-foreground hover:text-foreground transition-colors p-0.5"
                              title="Clear override"
                            >
                              <X className="size-3" />
                            </button>
                          )}
                        </div>
                      </td>
                      <td className="px-2 py-1.5">
                        {!currentOverride && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 text-[10px] gap-0.5 px-1.5"
                            onClick={() =>
                              setExpandedCreate(isCreating ? null : key)
                            }
                          >
                            <Plus className="size-3" />
                            Create
                          </Button>
                        )}
                      </td>
                    </tr>
                    {isCreating && (
                      <InlineCreateForm
                        key={`create-${r.escrowId}`}
                        registrar={r}
                        onCreated={(clid) => {
                          setOverride(key, clid);
                          setExpandedCreate(null);
                        }}
                        onCancel={() => setExpandedCreate(null)}
                      />
                    )}
                  </>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Empty registrars — collapsible */}
      {emptyRegistrars.length > 0 && (
        <div>
          <button
            type="button"
            onClick={() => setEmptyExpanded((v) => !v)}
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            {emptyExpanded ? (
              <ChevronDown className="size-3" />
            ) : (
              <ChevronRight className="size-3" />
            )}
            {emptyRegistrars.length} empty registrar{emptyRegistrars.length !== 1 ? 's' : ''}{' '}
            (no domains/hosts)
          </button>
          {emptyExpanded && (
            <div className="mt-2 overflow-x-auto rounded border">
              <table className="w-full text-xs">
                <thead>
                  <tr className="bg-muted/50 text-muted-foreground">
                    <th className="text-left px-2 py-1.5 font-medium">Escrow ID</th>
                    <th className="text-left px-2 py-1.5 font-medium">Name</th>
                    <th className="text-right px-2 py-1.5 font-medium">GurID</th>
                    <th className="text-right px-2 py-1.5 font-medium">Contacts</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {emptyRegistrars.map((r) => (
                    <tr key={r.escrowId} className="hover:bg-muted/20">
                      <td className="px-2 py-1.5 font-mono">{r.escrowId}</td>
                      <td className="px-2 py-1.5 truncate max-w-[140px]" title={r.name}>
                        {r.name}
                      </td>
                      <td className="px-2 py-1.5 text-right tabular-nums">{r.gurId}</td>
                      <td className="px-2 py-1.5 text-right tabular-nums">
                        {r.contactCount.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Action buttons */}
      <div className="flex items-center gap-2 pt-2 border-t">
        <TooltipProvider>
          <Button
            size="sm"
            disabled={overrideCount === 0 || sending !== null}
            onClick={handleApply}
            className="gap-1.5"
          >
            {sending === 'apply' ? (
              <Loader2 className="size-3 animate-spin" />
            ) : (
              <Check className="size-3" />
            )}
            Apply {overrideCount > 0 ? overrideCount : ''} Override{overrideCount !== 1 ? 's' : ''} &amp; Continue
          </Button>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                size="sm"
                variant="outline"
                disabled={sending !== null}
                onClick={handleSkip}
                className="gap-1.5"
              >
                {sending === 'skip' ? (
                  <Loader2 className="size-3 animate-spin" />
                ) : (
                  <AlertTriangle className="size-3 text-amber-500" />
                )}
                Skip &amp; Continue
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom" className="max-w-[260px] text-xs">
              Domains and hosts belonging to unmapped registrars will be skipped
              during import. You can re-run with overrides later.
            </TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                size="sm"
                variant="ghost"
                disabled={sending !== null}
                onClick={handleStop}
                className="gap-1.5 text-muted-foreground"
              >
                {sending === 'stop' ? (
                  <Loader2 className="size-3 animate-spin" />
                ) : (
                  <StopCircle className="size-3" />
                )}
                Stop Here for Now
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom" className="max-w-[260px] text-xs">
              Terminate the import workflow. You can re-run it from scratch when
              you&apos;re ready.
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
    </div>
  );
}
