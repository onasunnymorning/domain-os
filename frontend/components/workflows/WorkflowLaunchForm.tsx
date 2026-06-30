'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { Loader2 } from 'lucide-react';
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
import { launchWorkflow, type WorkflowMeta } from '@/lib/api/workflows';
import type { WorkflowRun } from '@/lib/stores/useWorkflowStore';
import { FileUpload } from './FileUpload';
import posthog from 'posthog-js';

interface WorkflowLaunchFormProps {
  workflow: WorkflowMeta | null;
  onClose: () => void;
  onLaunched: (run: WorkflowRun) => void;
}

// =============================================================================
// Per-workflow form bodies
// =============================================================================

function EscrowImportForm({
  params,
  onChange,
}: {
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
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
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
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
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
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
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
}) {
  switch (workflowKey) {
    case 'escrow-import':
      return <EscrowImportForm params={params} onChange={onChange} />;
    case 'tld-cleanup':
      return <TldCleanupForm params={params} onChange={onChange} />;
    case 'take-snapshot':
      return <TakeSnapshotForm params={params} onChange={onChange} />;
    default:
      return <ZeroParamConfirmation name={workflowName} />;
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
  const [params, setParams] = useState<Record<string, any>>({});
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
      const result = await launchWorkflow(workflow.key, params);

      const run: WorkflowRun = {
        workflowId: result.workflowId,
        runId: result.runId,
        type: workflow.key,
        displayName: workflow.name,
        status: 'RUNNING',
        temporalUrl: result.url,
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
        error?.response?.data?.message || error?.message || 'Failed to launch workflow';
      toast.error('Launch failed', { description: message });
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={workflow !== null} onOpenChange={(open) => !open && handleClose()}>
      <DialogContent>
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
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting && <Loader2 className="size-4 animate-spin" />}
            Launch Workflow
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
