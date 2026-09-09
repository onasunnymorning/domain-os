'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Check, ChevronsUpDown, Loader2 } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
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
import { launchWorkflow, type WorkflowMeta, type WorkflowStartResponse } from '@/lib/api/workflows';
import {
  listEscrowValidations,
  startEscrowSanitization,
  startEscrowValidation,
  type EscrowProfile,
} from '@/lib/api/escrow-runs';
import type { WorkflowRun } from '@/lib/stores/useWorkflowStore';
import { OperatorScopeSelect } from './OperatorScopeSelect';
import { FileUpload } from './FileUpload';
import posthog from 'posthog-js';

interface WorkflowLaunchFormProps {
  workflow: WorkflowMeta | null;
  onClose: () => void;
  onLaunched: (run: WorkflowRun) => void;
}

/**
 * Operator scope for a tenant-scoped launch.
 *
 * It travels in the form state like any other field but is sent as the
 * `X-Tenant-ID` header, never as a workflow parameter — hence the underscore.
 */
const SCOPE_KEY = '_tenantId';

type Params = Record<string, any>;

// =============================================================================
// Per-workflow form bodies
// =============================================================================

function EscrowImportForm({
  params,
  onChange,
}: {
  params: Params;
  onChange: (p: Params) => void;
}) {
  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="tld">TLD</Label>
        <Input
          id="tld"
          placeholder="e.g. com"
          value={params.tld ?? ''}
          onChange={(e) => onChange({ ...params, tld: e.target.value })}
        />
      </div>
      <div className="grid gap-2">
        <Label>Escrow Deposit File</Label>
        <FileUpload
          workflowType="escrow-import"
          tld={params.tld ?? ''}
          disabled={!params.tld}
          onUploaded={(key) => onChange({ ...params, objectKey: key })}
          onClear={() => onChange({ ...params, objectKey: undefined })}
        />
        {params.objectKey && (
          <p className="text-muted-foreground truncate text-xs">
            Key: {params.objectKey}
          </p>
        )}
      </div>
    </div>
  );
}

function TldCleanupForm({
  params,
  onChange,
}: {
  params: Params;
  onChange: (p: Params) => void;
}) {
  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="tld">TLD Name</Label>
        <Input
          id="tld"
          placeholder="e.g. com"
          value={params.tld ?? ''}
          onChange={(e) => onChange({ ...params, tld: e.target.value })}
        />
      </div>
      <div className="flex items-center gap-3">
        <Switch
          id="keepTLDAndPhases"
          checked={params.keepTLDAndPhases ?? false}
          onCheckedChange={(checked) =>
            onChange({ ...params, keepTLDAndPhases: checked })
          }
        />
        <Label htmlFor="keepTLDAndPhases">Keep TLD and Phases</Label>
      </div>
    </div>
  );
}


function TakeSnapshotForm({
  params,
  onChange,
}: {
  params: Params;
  onChange: (p: Params) => void;
}) {
  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="label">Label</Label>
        <Input
          id="label"
          placeholder="e.g. pre-migration, dev-seed, v2.1-release"
          value={params.label ?? ''}
          onChange={(e) => onChange({ ...params, label: e.target.value })}
        />
        <p className="text-muted-foreground text-xs">
          Short identifier used in the S3 key and workflow ID.
        </p>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="note">Note</Label>
        <textarea
          id="note"
          className="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex min-h-[80px] w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          placeholder="e.g. Snapshot with seed data for development"
          value={params.note ?? ''}
          onChange={(e) => onChange({ ...params, note: e.target.value })}
        />
        <p className="text-muted-foreground text-xs">
          Describe the intent of this snapshot. Preserved in the manifest.
        </p>
      </div>
    </div>
  );
}

function EscrowValidationForm({
  params,
  onChange,
}: {
  params: Params;
  onChange: (p: Params) => void;
}) {
  const profile: EscrowProfile = params.profile ?? 'ryde+sig';
  const signed = profile === 'ryde+sig';
  const tld = params.tld ?? '';

  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label>Registry Operator</Label>
        <OperatorScopeSelect
          value={params[SCOPE_KEY] ?? ''}
          onChange={(ryid) => onChange({ ...params, [SCOPE_KEY]: ryid })}
        />
        <p className="text-muted-foreground text-xs">
          The deposit is recorded in this operator&apos;s scope, and the TLD must belong to it.
        </p>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="tld">TLD</Label>
        <Input
          id="tld"
          placeholder="e.g. com"
          value={tld}
          onChange={(e) => onChange({ ...params, tld: e.target.value })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="profile">Deposit Profile</Label>
        <Select
          value={profile}
          onValueChange={(value) =>
            // A signature is rejected for an unsigned profile, so drop it on switch.
            onChange({ ...params, profile: value, signatureObjectKey: undefined })
          }
        >
          <SelectTrigger id="profile">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="ryde+sig">Signed &amp; encrypted (.ryde + .sig)</SelectItem>
            <SelectItem value="xml">Plaintext (.xml or .xml.gz)</SelectItem>
          </SelectContent>
        </Select>
        {!signed && (
          <p className="text-muted-foreground text-xs">
            A plaintext deposit is validated but never verified: it produces no DVPN/DVFN
            notification and cannot be used as an ICANN custody claim.
          </p>
        )}
      </div>

      <div className="grid gap-2">
        <Label>{signed ? 'Encrypted Deposit (.ryde)' : 'Deposit (.xml or .xml.gz)'}</Label>
        <FileUpload
          workflowType="escrow-validation"
          tld={tld}
          disabled={!tld}
          onUploaded={(key) => onChange({ ...params, artifactObjectKey: key })}
          onClear={() => onChange({ ...params, artifactObjectKey: undefined })}
        />
      </div>

      {signed && (
        <div className="grid gap-2">
          <Label>Detached Signature (.sig)</Label>
          <FileUpload
            workflowType="escrow-validation"
            tld={tld}
            disabled={!tld}
            onUploaded={(key) => onChange({ ...params, signatureObjectKey: key })}
            onClear={() => onChange({ ...params, signatureObjectKey: undefined })}
          />
        </div>
      )}

      <div className="grid gap-2">
        <Label htmlFor="intakeRef">Intake Reference</Label>
        <Input
          id="intakeRef"
          placeholder="Optional — e.g. a ticket or transfer id"
          value={params.intakeRef ?? ''}
          onChange={(e) => onChange({ ...params, intakeRef: e.target.value })}
        />
      </div>
    </div>
  );
}

function EscrowSanitizeForm({
  params,
  onChange,
}: {
  params: Params;
  onChange: (p: Params) => void;
}) {
  const [sourceOpen, setSourceOpen] = useState(false);
  const tenantId: string = params[SCOPE_KEY] ?? '';

  // Only an accepted run can be sanitized; the server enforces this too (409).
  const { data, isLoading } = useQuery({
    queryKey: ['escrow-validations', tenantId, 'PASS'],
    queryFn: () => listEscrowValidations(tenantId, { outcome: 'PASS', pagesize: 50 }),
    enabled: tenantId !== '',
  });
  const runs = data?.items ?? [];
  const selected = runs.find((r) => r.id === params.sourceValidationRunId);

  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label>Registry Operator</Label>
        <OperatorScopeSelect
          value={tenantId}
          onChange={(ryid) =>
            // Run ids are scope-local: a source picked under another operator
            // would 404, so clear the selection with the scope.
            onChange({ ...params, [SCOPE_KEY]: ryid, sourceValidationRunId: undefined })
          }
        />
      </div>

      <div className="grid gap-2">
        <Label>Source Validation Run</Label>
        <Popover open={sourceOpen} onOpenChange={setSourceOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="outline"
              role="combobox"
              aria-expanded={sourceOpen}
              disabled={!tenantId}
              className={cn(
                'w-full justify-between font-normal',
                !selected && 'text-muted-foreground'
              )}
            >
              {selected
                ? `${selected.tld} · ${new Date(selected.startedAt).toLocaleString()}`
                : tenantId
                  ? 'Select an accepted deposit…'
                  : 'Select a registry operator first'}
              <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
            <Command>
              <CommandInput placeholder="Search by TLD or run id…" />
              <CommandList>
                <CommandEmpty>
                  {isLoading ? 'Loading…' : 'No accepted validation runs in this scope.'}
                </CommandEmpty>
                <CommandGroup>
                  {runs.map((run) => (
                    <CommandItem
                      key={run.id}
                      value={`${run.tld} ${run.id}`}
                      onSelect={() => {
                        onChange({ ...params, sourceValidationRunId: run.id });
                        setSourceOpen(false);
                      }}
                    >
                      <Check
                        className={cn(
                          'mr-2 h-4 w-4 shrink-0',
                          params.sourceValidationRunId === run.id ? 'opacity-100' : 'opacity-0'
                        )}
                      />
                      <div className="min-w-0">
                        <div className="truncate text-sm">
                          {run.tld} · {run.profile}
                          {run.verified ? '' : ' · unverified'}
                        </div>
                        <div className="text-muted-foreground truncate font-mono text-[11px]">
                          {new Date(run.startedAt).toLocaleString()} · {run.id}
                        </div>
                      </div>
                    </CommandItem>
                  ))}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
        <p className="text-muted-foreground text-xs">
          The TLD is not a parameter: the derivative inherits it from the bound source deposit.
        </p>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="syntheticSuffix">Synthetic Suffix</Label>
        <Input
          id="syntheticSuffix"
          placeholder="Optional — defaults to the configured suffix"
          value={params.syntheticSuffix ?? ''}
          onChange={(e) => onChange({ ...params, syntheticSuffix: e.target.value })}
        />
        <p className="text-muted-foreground text-xs">
          Every FQDN in the derivative is rewritten onto this suffix. It must differ from the
          source TLD.
        </p>
      </div>
    </div>
  );
}

function ZeroParamConfirmation({ name }: { name: string }) {
  return (
    <p className="text-muted-foreground text-sm">
      Run <span className="text-foreground font-medium">{name}</span> now?
    </p>
  );
}

// =============================================================================
// Form body router
// =============================================================================

function FormBody({
  workflowKey,
  workflowName,
  params,
  onChange,
}: {
  workflowKey: string;
  workflowName: string;
  params: Params;
  onChange: (p: Params) => void;
}) {
  switch (workflowKey) {
    case 'escrow-import':
      return <EscrowImportForm params={params} onChange={onChange} />;
    case 'tld-cleanup':
      return <TldCleanupForm params={params} onChange={onChange} />;
    case 'take-snapshot':
      return <TakeSnapshotForm params={params} onChange={onChange} />;
    case 'escrow-validation':
      return <EscrowValidationForm params={params} onChange={onChange} />;
    case 'escrow-sanitize':
      return <EscrowSanitizeForm params={params} onChange={onChange} />;
    default:
      return <ZeroParamConfirmation name={workflowName} />;
  }
}

// =============================================================================
// Launch dispatch
// =============================================================================

/**
 * Returns true when every field the workflow requires is present.
 *
 * The server validates all of this again — this only keeps the button from
 * firing a request that is certain to 400.
 */
function canSubmit(workflowKey: string, params: Params): boolean {
  switch (workflowKey) {
    case 'escrow-validation':
      return Boolean(
        params[SCOPE_KEY] &&
          params.tld &&
          params.artifactObjectKey &&
          ((params.profile ?? 'ryde+sig') === 'xml' || params.signatureObjectKey)
      );
    case 'escrow-sanitize':
      return Boolean(params[SCOPE_KEY] && params.sourceValidationRunId);
    default:
      return true;
  }
}

/**
 * Starts the workflow.
 *
 * Tenant-scoped workflows go to their own endpoint rather than the generic
 * launcher: the scope is a header there, and the endpoint resolves the source
 * record in that scope before starting anything.
 */
async function launch(workflowKey: string, params: Params): Promise<WorkflowStartResponse> {
  const tenantId: string = params[SCOPE_KEY] ?? '';

  switch (workflowKey) {
    case 'escrow-validation': {
      const profile: EscrowProfile = params.profile ?? 'ryde+sig';
      return startEscrowValidation(tenantId, {
        tld: String(params.tld).trim(),
        profile,
        artifactObjectKey: params.artifactObjectKey,
        signatureObjectKey: profile === 'ryde+sig' ? params.signatureObjectKey : undefined,
        intakeRef: params.intakeRef?.trim() || undefined,
      });
    }
    case 'escrow-sanitize':
      return startEscrowSanitization(tenantId, {
        sourceValidationRunId: params.sourceValidationRunId,
        syntheticSuffix: params.syntheticSuffix?.trim() || undefined,
      });
    default:
      return launchWorkflow(workflowKey, params);
  }
}

// =============================================================================
// Main Dialog Component
// =============================================================================

export function WorkflowLaunchForm({
  workflow,
  onClose,
  onLaunched,
}: WorkflowLaunchFormProps) {
  const [params, setParams] = useState<Params>({});
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleClose = () => {
    setParams({});
    setIsSubmitting(false);
    onClose();
  };

  const handleSubmit = async () => {
    if (!workflow) return;

    setIsSubmitting(true);
    try {
      const result = await launch(workflow.key, params);

      const run: WorkflowRun = {
        workflowId: result.workflowId,
        runId: result.runId,
        type: workflow.key,
        displayName: workflow.name,
        status: 'RUNNING',
        temporalUrl: result.url ?? '',
        startedAt: new Date().toISOString(),
        params,
      };

      posthog.capture('workflow_launched', {
        workflow_key: workflow.key,
        workflow_name: workflow.name,
        workflow_id: result.workflowId,
      });
      onLaunched(run);
      toast.success(`Workflow "${workflow.name}" launched successfully`, {
        description: `ID: ${result.workflowId}`,
      });
      handleClose();
    } catch (error: any) {
      posthog.captureException(error);
      const message =
        error?.response?.data?.error ||
        error?.response?.data?.message ||
        error?.message ||
        'Failed to launch workflow';
      toast.error('Launch failed', { description: message });
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={workflow !== null} onOpenChange={(open) => !open && handleClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{workflow?.name ?? 'Launch Workflow'}</DialogTitle>
          <DialogDescription>
            {workflow?.description ?? 'Configure and launch a workflow'}
          </DialogDescription>
        </DialogHeader>

        {workflow && (
          <FormBody
            workflowKey={workflow.key}
            workflowName={workflow.name}
            params={params}
            onChange={setParams}
          />
        )}

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={isSubmitting || (workflow ? !canSubmit(workflow.key, params) : true)}
          >
            {isSubmitting && <Loader2 className="size-4 animate-spin" />}
            Launch Workflow
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
