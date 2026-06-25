'use client';

import { useEffect, useState, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Download, AlertCircle, AlertTriangle, CheckCircle2, Loader2, XCircle } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { getWorkflowResult, getStorageDownloadURL, signalWorkflow } from '@/lib/api/workflows';
import { QAReportViewer } from './QAReportViewer';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';

interface WorkflowResultProps {
  workflowId: string;
  workflowType: string;
  status: string;
  signalName?: string;
  onStateLoaded?: (state: any) => void;
}

// Known S3 artifact fields per workflow type — used to identify downloadable files in results
const ARTIFACT_FIELDS: Record<string, Record<string, string>> = {
  'escrow-staging': {
    qaReportKey: 'QA Report',
    stagedDbKey: 'Staged Database',
    dbKey: 'Raw Database',
  },
  'escrow-import': {
    qaReportKey: 'QA Report',
    stagedDbKey: 'Staged Database',
    dbKey: 'Raw Database',
    verificationReportKey: 'Verification Report',
    QAReportKey: 'QA Report',
    StagedDBKey: 'Staged Database',
    DBKey: 'Raw Database',
  },
  'tld-cleanup': {
    ManifestKey: 'Cleanup Manifest',
    BackupKey: 'Backup Archive',
    manifestKey: 'Cleanup Manifest',
    backupKey: 'Backup Archive',
  },
  'take-snapshot': {
    snapshotKey: 'Snapshot (JSONL)',
    manifestKey: 'Manifest',
    SnapshotKey: 'Snapshot (JSONL)',
    ManifestKey: 'Manifest',
  },
};

function isArtifactField(workflowType: string, field: string): string | null {
  return ARTIFACT_FIELDS[workflowType]?.[field] ?? null;
}

// Pretty labels for known result fields
const FIELD_LABELS: Record<string, string> = {
  tld: 'TLD',
  objectKey: 'Source File',
  startedAt: 'Started',
  completedAt: 'Completed',
  runPrefix: 'Run Prefix',
  qaPassed: 'QA Status',
  ingestionRunId: 'Ingestion Run',
  notes: 'Notes',
  artifacts: 'Artifacts',
  Success: 'Success',
  Counts: 'Counts',
  ManifestKey: 'Manifest',
  BackupKey: 'Backup',
  DeletedCount: 'Deleted Count',
  manifestKey: 'Manifest',
  backupKey: 'Backup',
  ingestedCounts: 'Ingested Counts',
  IngestedCounts: 'Ingested Counts',
  failures: 'Failures',
  createdItems: 'Created Registrars',
  updatedItems: 'Updated Registrars',
  totalIana: 'Total IANA',
  totalExisting: 'Total Existing',
  totalProcessed: 'Total Processed',
  // Take Snapshot
  snapshotKey: 'Snapshot File',
  SnapshotKey: 'Snapshot File',
  tableCounts: 'Table Counts',
  totalRows: 'Total Rows',
  // Seed from Snapshot
  insertedCounts: 'Inserted Counts',
  skippedCounts: 'Skipped Counts',
  totalInserted: 'Total Inserted',
  totalSkipped: 'Total Skipped',
};

function ArtifactDownloadButton({ label, s3Key }: { label: string; s3Key: string }) {
  const [downloading, setDownloading] = useState(false);

  const handleDownload = async () => {
    setDownloading(true);
    try {
      const { url } = await getStorageDownloadURL(s3Key);
      window.open(url, '_blank');
    } catch {
      // Fallback: show error
    } finally {
      setDownloading(false);
    }
  };

  const filename = s3Key.split('/').pop() || s3Key;

  return (
    <button
      type="button"
      onClick={handleDownload}
      disabled={downloading}
      className={cn(
        'flex items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors',
        'hover:bg-muted/50 active:bg-muted',
        downloading && 'opacity-50'
      )}
    >
      <Download className="text-muted-foreground size-4 shrink-0" />
      <div className="min-w-0 flex-1">
        <div className="font-medium">{label}</div>
        <div className="text-muted-foreground truncate text-xs">{filename}</div>
      </div>
    </button>
  );
}

function CountsTable({ counts }: { counts: Record<string, number> }) {
  // Group by entity type: contacts_total, contacts_inserted → Contacts: { total: X, inserted: Y }
  const groups: Record<string, Record<string, number>> = {};
  for (const [key, value] of Object.entries(counts)) {
    const parts = key.split('_');
    if (parts.length >= 2) {
      const entity = parts[0].charAt(0).toUpperCase() + parts[0].slice(1);
      const metric = parts.slice(1).join('_');
      if (!groups[entity]) groups[entity] = {};
      groups[entity][metric] = value;
    } else {
      if (!groups['Other']) groups['Other'] = {};
      groups['Other'][key] = value;
    }
  }

  return (
    <div className="overflow-hidden rounded-md border">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-muted/50">
            <th className="px-3 py-1.5 text-left font-medium">Entity</th>
            <th className="px-3 py-1.5 text-right font-medium">Total</th>
            <th className="px-3 py-1.5 text-right font-medium">Inserted</th>
            <th className="px-3 py-1.5 text-right font-medium">Updated</th>
          </tr>
        </thead>
        <tbody>
          {Object.entries(groups).map(([entity, metrics]) => (
            <tr key={entity} className="border-t">
              <td className="px-3 py-1.5 font-medium">{entity}</td>
              <td className="text-muted-foreground px-3 py-1.5 text-right tabular-nums">
                {metrics.total?.toLocaleString() ?? '—'}
              </td>
              <td className="px-3 py-1.5 text-right tabular-nums text-green-600 dark:text-green-400">
                {metrics.inserted?.toLocaleString() ?? '—'}
              </td>
              <td className="text-muted-foreground px-3 py-1.5 text-right tabular-nums">
                {metrics.updated?.toLocaleString() ?? '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ArtifactsMap({ artifacts }: { artifacts: Record<string, string> }) {
  return (
    <div className="space-y-1">
      {Object.entries(artifacts).map(([name, key]) => (
        <ArtifactDownloadButton key={name} label={name} s3Key={key} />
      ))}
    </div>
  );
}

function ResultField({ label, value, workflowType }: { label: string; value: any; workflowType: string }) {
  // Skip internal/noisy fields
  if (label === 'runPrefix' || label === 'objectKey') return null;

  if (workflowType === 'spec5-sweep' && label === 'tldResults' && typeof value === 'object' && value !== null) {
    return (
      <div className="space-y-2 py-2">
        <span className="text-muted-foreground text-xs block font-semibold uppercase tracking-wider">
          Sweep Results by TLD
        </span>
        <div className="overflow-hidden rounded-md border">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-muted/50 text-xs">
                <th className="px-3 py-2 text-left font-medium">TLD</th>
                <th className="px-3 py-2 text-right font-medium">Matching Domains Count</th>
                <th className="px-3 py-2 text-center font-medium">Artifact</th>
              </tr>
            </thead>
            <tbody>
              {Object.entries(value).map(([tld, details]: [string, any]) => (
                <tr key={tld} className="border-t hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-2 font-mono text-sm">{tld}</td>
                  <td className="px-3 py-2 text-right font-medium tabular-nums">
                    {details.count?.toLocaleString() ?? 0}
                  </td>
                  <td className="px-3 py-2 flex justify-center">
                    {details.artifactKey ? (
                      <ArtifactDownloadButton label="Download CSV" s3Key={details.artifactKey} />
                    ) : (
                      <span className="text-muted-foreground text-xs">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  const displayLabel = FIELD_LABELS[label] || label;

  // QA status badge
  if (label === 'qaPassed') {
    return (
      <div className="flex items-center justify-between py-1">
        <span className="text-muted-foreground text-xs">{displayLabel}</span>
        <Badge variant={value ? 'default' : 'destructive'} className="text-xs">
          {value ? '✓ Passed' : '✗ Failed'}
        </Badge>
      </div>
    );
  }

  // Counts table
  if ((label === 'Counts' || label === 'ingestedCounts' || label === 'IngestedCounts') && typeof value === 'object' && value !== null) {
    return (
      <div className="space-y-1 py-1">
        <span className="text-muted-foreground text-xs">{displayLabel}</span>
        <CountsTable counts={value} />
      </div>
    );
  }

  // Artifacts map
  if (label === 'artifacts' && typeof value === 'object' && value !== null) {
    return (
      <div className="space-y-1 py-1">
        <span className="text-muted-foreground text-xs">{displayLabel}</span>
        <ArtifactsMap artifacts={value} />
      </div>
    );
  }

  // QA report → inline viewer with download fallback
  if (label === 'qaReportKey' && typeof value === 'string' && value) {
    return (
      <div className="space-y-2 py-1">
        <QAReportViewer s3Key={value} />
        <ArtifactDownloadButton label="Download Full Report" s3Key={value} />
      </div>
    );
  }

  // S3 artifact field → download button
  const artifactLabel = isArtifactField(workflowType, label);
  if (artifactLabel && typeof value === 'string' && value) {
    return (
      <div className="py-1">
        <ArtifactDownloadButton label={artifactLabel} s3Key={value} />
      </div>
    );
  }

  // Notes array
  if (label === 'notes' && Array.isArray(value)) {
    if (value.length === 0) return null;
    return (
      <div className="space-y-1 py-1">
        <span className="text-muted-foreground text-xs">{displayLabel}</span>
        {value.map((note, i) => (
          <div key={i} className="text-muted-foreground rounded bg-muted/50 px-2 py-1 text-xs">
            {note}
          </div>
        ))}
      </div>
    );
  }

  // Failures array — generic: works with domainName (ExpiryLoop) or clId (SyncRegistrars)
  if (label === 'failures' && Array.isArray(value)) {
    if (value.length === 0) return null;
    const idLabel = value[0]?.domainName ? 'Domain' : 'Registrar';
    return (
      <div className="space-y-2 py-2">
        <span className="text-muted-foreground text-xs block font-semibold uppercase tracking-wider">
          {displayLabel}
        </span>
        <div className="overflow-hidden rounded-md border">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-muted/50 text-xs">
                <th className="px-3 py-1.5 text-left font-medium">{idLabel}</th>
                <th className="px-3 py-1.5 text-left font-medium">Operation</th>
                <th className="px-3 py-1.5 text-left font-medium">Error</th>
              </tr>
            </thead>
            <tbody>
              {value.map((f: any, i: number) => (
                <tr key={i} className="border-t hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-1.5 font-mono text-xs">{f.domainName || f.clId}</td>
                  <td className="px-3 py-1.5 text-xs">
                    <Badge variant="outline" className="text-[10px] py-0 px-1 capitalize">
                      {f.operation?.replace('-', ' ')}
                    </Badge>
                  </td>
                  <td className="px-3 py-1.5 text-xs text-red-600 dark:text-red-400 font-medium">
                    {f.error}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  // Created registrars array
  if (label === 'createdItems' && Array.isArray(value)) {
    if (value.length === 0) return null;
    return (
      <div className="space-y-2 py-2">
        <span className="text-muted-foreground text-xs block font-semibold uppercase tracking-wider">
          {displayLabel}
        </span>
        <div className="overflow-hidden rounded-md border">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-muted/50 text-xs">
                <th className="px-3 py-1.5 text-left font-medium">Registrar</th>
                <th className="px-3 py-1.5 text-left font-medium">Name</th>
                <th className="px-3 py-1.5 text-right font-medium">GurID</th>
                <th className="px-3 py-1.5 text-left font-medium">Status</th>
                <th className="px-3 py-1.5 text-left font-medium">IANA Status</th>
              </tr>
            </thead>
            <tbody>
              {value.map((item: any, i: number) => (
                <tr key={i} className="border-t hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-1.5 font-mono text-xs">{item.clId}</td>
                  <td className="px-3 py-1.5 text-xs truncate max-w-[200px]" title={item.name}>{item.name}</td>
                  <td className="px-3 py-1.5 text-xs text-right tabular-nums">{item.gurId}</td>
                  <td className="px-3 py-1.5 text-xs">
                    <Badge variant="outline" className="text-[10px] py-0 px-1 capitalize">
                      {item.status}
                    </Badge>
                  </td>
                  <td className="px-3 py-1.5 text-xs">
                    <Badge variant="outline" className="text-[10px] py-0 px-1">
                      {item.ianaStatus}
                    </Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  // Updated registrars array
  if (label === 'updatedItems' && Array.isArray(value)) {
    if (value.length === 0) return null;
    return (
      <div className="space-y-2 py-2">
        <span className="text-muted-foreground text-xs block font-semibold uppercase tracking-wider">
          {displayLabel}
        </span>
        <div className="overflow-hidden rounded-md border">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-muted/50 text-xs">
                <th className="px-3 py-1.5 text-left font-medium">Registrar</th>
                <th className="px-3 py-1.5 text-left font-medium">Status Change</th>
                <th className="px-3 py-1.5 text-left font-medium">IANA Status Change</th>
              </tr>
            </thead>
            <tbody>
              {value.map((item: any, i: number) => (
                <tr key={i} className="border-t hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-1.5 font-mono text-xs">{item.clId}</td>
                  <td className="px-3 py-1.5 text-xs">
                    {item.oldStatus && item.newStatus ? (
                      <span className="inline-flex items-center gap-1">
                        <Badge variant="secondary" className="text-[10px] py-0 px-1 capitalize">{item.oldStatus}</Badge>
                        <span className="text-muted-foreground">→</span>
                        <Badge variant="default" className="text-[10px] py-0 px-1 capitalize">{item.newStatus}</Badge>
                      </span>
                    ) : (
                      <span className="text-muted-foreground text-xs">—</span>
                    )}
                  </td>
                  <td className="px-3 py-1.5 text-xs">
                    {item.oldIanaStatus && item.newIanaStatus ? (
                      <span className="inline-flex items-center gap-1">
                        <Badge variant="secondary" className="text-[10px] py-0 px-1">{item.oldIanaStatus}</Badge>
                        <span className="text-muted-foreground">→</span>
                        <Badge variant="default" className="text-[10px] py-0 px-1">{item.newIanaStatus}</Badge>
                      </span>
                    ) : (
                      <span className="text-muted-foreground text-xs">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  // Boolean
  if (typeof value === 'boolean') {
    return (
      <div className="flex items-center justify-between py-1">
        <span className="text-muted-foreground text-xs">{displayLabel}</span>
        <Badge variant={value ? 'default' : 'secondary'} className="text-xs">
          {value ? 'Yes' : 'No'}
        </Badge>
      </div>
    );
  }

  // Timestamps
  if (label === 'startedAt' || label === 'completedAt') {
    // Skip timing fields — already visible in the header
    return null;
  }

  // Numbers
  if (typeof value === 'number') {
    return (
      <div className="flex items-center justify-between py-1">
        <span className="text-muted-foreground text-xs">{displayLabel}</span>
        <span className="text-sm font-medium tabular-nums">{value.toLocaleString()}</span>
      </div>
    );
  }

  // Simple string
  if (typeof value === 'string' && value) {
    return (
      <div className="flex items-center justify-between gap-4 py-1">
        <span className="text-muted-foreground shrink-0 text-xs">{displayLabel}</span>
        <span className="truncate text-sm">{value}</span>
      </div>
    );
  }

  return null;
}
// ---------------------------------------------------------------------------
// Registrar Mapping Summary — always shows mapping counts, expands gaps if any
// ---------------------------------------------------------------------------
interface UnmappedRegistrar {
  escrowId: string;
  name: string;
  gurId: number;
  domainCount: number;
  hostCount: number;
  contactCount: number;
}

interface RegistrarMappingSummaryProps {
  total?: number;
  mapped?: number;
  unmapped?: UnmappedRegistrar[];
}

function RegistrarMappingSummary({ total, mapped, unmapped }: RegistrarMappingSummaryProps) {
  // Don't render if we have no mapping data at all (e.g. old workflow result)
  if (!total && !mapped && !unmapped?.length) return null;

  const displayTotal = total || 0;
  const displayMapped = mapped || 0;
  const autoFixed = displayTotal - displayMapped;
  const hasUnresolved = unmapped && unmapped.length > 0;
  const isFullyMapped = displayMapped === displayTotal && !hasUnresolved;

  // Determine visual state:
  // - Green: all registrars mapped 1:1 (no auto-fix, no gaps)
  // - Amber: auto-fixed registrars or unresolved gaps
  const variant = isFullyMapped ? 'green' : 'amber';
  const colors = variant === 'green'
    ? { bg: 'bg-green-500/10 border-green-500/20', text: 'text-green-600 dark:text-green-400' }
    : { bg: 'bg-amber-500/10 border-amber-500/20', text: 'text-amber-600 dark:text-amber-400' };

  return (
    <div className="space-y-2 border-t pt-3">
      <p className="text-xs font-semibold text-muted-foreground">Registrar Mapping</p>

      {/* Summary line */}
      <div className={`flex items-start gap-2 rounded-md px-3 py-2 border ${colors.bg}`}>
        {isFullyMapped ? (
          <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-green-500" />
        ) : (
          <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-amber-500" />
        )}
        <div>
          <p className={`text-xs font-medium ${colors.text}`}>
            {displayMapped} of {displayTotal} registrars mapped
            {autoFixed > 0 && !hasUnresolved
              ? ` — ${autoFixed} auto-fixed (host-only registrars resolved via domain ownership)`
              : ''}
            {hasUnresolved ? ` — ${unmapped!.length} unmapped` : ''}
          </p>
          {hasUnresolved && (
            <p className="text-[10px] text-muted-foreground mt-0.5">
              Re-run with <code className="bg-muted px-1 rounded">registrarOverrides</code> to resolve.
            </p>
          )}
        </div>
      </div>

      {/* Unmapped details table */}
      {hasUnresolved && (
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
              </tr>
            </thead>
            <tbody className="divide-y">
              {unmapped!.map((r) => (
                <tr key={r.escrowId} className="hover:bg-muted/20">
                  <td className="px-2 py-1.5 font-mono">{r.escrowId}</td>
                  <td className="px-2 py-1.5 truncate max-w-[140px]">{r.name}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{r.gurId}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{r.domainCount.toLocaleString()}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{r.hostCount.toLocaleString()}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{r.contactCount.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
export function WorkflowResult({ workflowId, workflowType, status, signalName, onStateLoaded }: WorkflowResultProps) {
  const [signalSending, setSignalSending] = useState<'approve' | 'reject' | null>(null);
  const [signalSent, setSignalSent] = useState(false);

  const getSignalConfig = useCallback(() => {
    switch (signalName) {
      case 'ConfirmTLDCleanup':
        return {
          approveText: 'Confirm Deletion',
          rejectText: 'Cancel Deletion',
          approveToast: 'Deletion approved — cleanup starting',
          rejectToast: 'Deletion cancelled',
        };
      case 'ConfirmSeedFromSnapshot':
        return {
          approveText: 'Confirm Seed',
          rejectText: 'Cancel Seed',
          approveToast: 'Seed approved — database import starting',
          rejectToast: 'Seed cancelled',
        };
      case 'ConfirmEscrowImport':
      default:
        return {
          approveText: 'Approve & Ingest',
          rejectText: 'Reject',
          approveToast: 'Import approved — ingestion starting',
          rejectToast: 'Import rejected',
        };
    }
  }, [signalName]);

  const signalConfig = getSignalConfig();

  const handleSignal = useCallback(async (approved: boolean) => {
    if (!signalName) return;
    const action = approved ? 'approve' : 'reject';
    setSignalSending(action);
    try {
      await signalWorkflow(workflowId, signalName, approved);
      setSignalSent(true);
      toast.success(approved ? signalConfig.approveToast : signalConfig.rejectToast, {
        description: `Signal sent to ${workflowId}`,
      });
    } catch (error: any) {
      const message = error?.response?.data?.message || error?.message || 'Failed to send signal';
      toast.error('Signal failed', { description: message });
    } finally {
      setSignalSending(null);
    }
  }, [workflowId, signalName, signalConfig]);

  const { data, isLoading, error } = useQuery({
    queryKey: ['workflow-result', workflowId, status],
    queryFn: () => getWorkflowResult(workflowId),
    refetchInterval: status === 'RUNNING' ? 3000 : false,
    staleTime: status === 'RUNNING' ? 0 : Infinity,
    retry: 1,
  });

  useEffect(() => {
    if (data?.state) {
      onStateLoaded?.(data.state);
    }
  }, [data?.state, onStateLoaded]);

  // Running state
  if (status === 'RUNNING') {
    if (isLoading) {
      return (
        <div className="text-muted-foreground flex items-center gap-2 py-2 text-sm">
          <Loader2 className="size-4 animate-spin text-blue-500" />
          <span>Workflow is running…</span>
        </div>
      );
    }

    if (data?.state) {
      const state = data.state;
      return (
        <div className="space-y-3 py-1">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground font-medium">Phase</span>
            <Badge variant="secondary" className="capitalize text-xs">
              {state.phase?.replace('_', ' ')}
            </Badge>
          </div>

          {(typeof state.domainCount === 'number' || typeof state.contactCount === 'number' || typeof state.hostCount === 'number') && (
            <div className="space-y-1.5 rounded-md border p-2 bg-muted/20">
              <p className="text-muted-foreground text-[10px] font-semibold uppercase tracking-wider">
                Planned deletion counts
              </p>
              <div className="grid grid-cols-3 gap-2 text-center text-xs">
                <div>
                  <div className="font-semibold tabular-nums">{(state.domainCount ?? 0).toLocaleString()}</div>
                  <div className="text-muted-foreground text-[10px]">Domains</div>
                </div>
                <div>
                  <div className="font-semibold tabular-nums">{(state.contactCount ?? 0).toLocaleString()}</div>
                  <div className="text-muted-foreground text-[10px]">Contacts</div>
                </div>
                <div>
                  <div className="font-semibold tabular-nums">{(state.hostCount ?? 0).toLocaleString()}</div>
                  <div className="text-muted-foreground text-[10px]">Hosts</div>
                </div>
              </div>
            </div>
          )}

          {(state.stagedDbKey || state.baseDbKey || state.manifestKey || state.ManifestKey || state.backupKey || state.BackupKey) && (
            <div className="space-y-1.5 pt-2 border-t mt-2">
              <span className="text-muted-foreground text-xs font-semibold">Workflow Artifacts</span>
              {state.stagedDbKey && (
                <ArtifactDownloadButton label="Staged Database" s3Key={state.stagedDbKey} />
              )}
              {state.baseDbKey && (
                <ArtifactDownloadButton label="Base Database" s3Key={state.baseDbKey} />
              )}
              {(state.manifestKey || state.ManifestKey) && (
                <ArtifactDownloadButton label="Cleanup Manifest" s3Key={state.manifestKey || state.ManifestKey} />
              )}
              {(state.backupKey || state.BackupKey) && (
                <ArtifactDownloadButton label="Backup Archive" s3Key={state.backupKey || state.BackupKey} />
              )}
            </div>
          )}

          <RegistrarMappingSummary
            total={state.totalRegistrars}
            mapped={state.mappedRegistrars}
            unmapped={state.unmappedRegistrars}
          />

          {state.qaReportKey && (
            <div className="space-y-2 py-1 border-t pt-3 mt-3">
              <p className="text-xs font-semibold text-muted-foreground">Interactive QA Report</p>
              <QAReportViewer s3Key={state.qaReportKey} />
              <ArtifactDownloadButton label="Download QA Report" s3Key={state.qaReportKey} />
            </div>
          )}

          {state.error && (
            <div className="rounded bg-red-500/10 p-2 text-xs text-red-600 dark:text-red-400">
              {state.error}
            </div>
          )}

          {/* Inline Approve/Reject buttons — shown immediately when phase is pending_confirmation */}
          {signalName && state.phase === 'pending_confirmation' && (
            <div className="border-t pt-3 mt-3 space-y-2">
              <p className="text-xs font-semibold text-muted-foreground">Human-in-the-Loop Decision</p>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="default"
                  disabled={signalSent || signalSending !== null}
                  onClick={() => handleSignal(true)}
                  className="gap-1.5"
                >
                  {signalSending === 'approve' ? (
                    <Loader2 className="size-3 animate-spin" />
                  ) : (
                    <CheckCircle2 className="size-3" />
                  )}
                  {signalConfig.approveText}
                </Button>
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={signalSent || signalSending !== null}
                  onClick={() => handleSignal(false)}
                  className="gap-1.5"
                >
                  {signalSending === 'reject' ? (
                    <Loader2 className="size-3 animate-spin" />
                  ) : (
                    <XCircle className="size-3" />
                  )}
                  {signalConfig.rejectText}
                </Button>
                {signalSent && (
                  <span className="text-muted-foreground text-xs">Signal sent</span>
                )}
              </div>
            </div>
          )}
        </div>
      );
    }

    return (
      <div className="text-muted-foreground flex items-center gap-2 py-2 text-sm">
        <Loader2 className="size-4 animate-spin text-blue-500" />
        <span>Workflow is running…</span>
      </div>
    );
  }

  // Loading result
  if (isLoading) {
    return (
      <div className="text-muted-foreground flex items-center gap-2 py-2 text-sm">
        <Loader2 className="size-4 animate-spin" />
        <span>Loading result…</span>
      </div>
    );
  }

  // Error fetching result
  if (error || !data) {
    return (
      <div className="text-muted-foreground py-2 text-sm">
        Unable to fetch workflow result.
      </div>
    );
  }

  // =========================================================================
  // QA Gate Blocked — completed successfully but QA didn't pass
  // =========================================================================
  if (data.result && data.result.qaPassed === false && !data.error) {
    const r = data.result;
    return (
      <div className="space-y-3">
        {/* Warning banner */}
        <div className="flex items-start gap-2 rounded-md bg-amber-500/10 border border-amber-500/20 px-3 py-2.5">
          <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-500" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-amber-600 dark:text-amber-400">
              QA gate blocked ingestion
            </p>
            <p className="text-xs text-muted-foreground">
              The staged database failed one or more error-severity validation checks.
              Review the QA report below, fix the source data or mappings, and re-run the import.
            </p>
          </div>
        </div>

        {/* Context */}
        <div className="divide-y text-xs">
          {r.tld && (
            <div className="flex items-center justify-between py-1.5">
              <span className="text-muted-foreground">TLD</span>
              <Badge variant="outline" className="text-xs">{r.tld}</Badge>
            </div>
          )}
          {r.runPrefix && (
            <div className="flex items-center justify-between gap-4 py-1.5">
              <span className="text-muted-foreground shrink-0">Run Prefix</span>
              <span className="truncate text-xs font-mono">{r.runPrefix}</span>
            </div>
          )}
        </div>

        {/* Registrar Mapping */}
        <RegistrarMappingSummary
          total={r.totalRegistrars}
          mapped={r.mappedRegistrars}
          unmapped={r.unmappedRegistrars}
        />

        {/* QA Report Viewer */}
        {r.qaReportKey && (
          <div className="space-y-2 border-t pt-3">
            <p className="text-xs font-semibold text-muted-foreground">Interactive QA Report</p>
            <QAReportViewer s3Key={r.qaReportKey} />
          </div>
        )}

        {/* Artifact downloads */}
        <div className="space-y-2 border-t pt-3">
          <p className="text-xs font-semibold text-muted-foreground">Artifacts</p>
          <div className="space-y-1">
            {r.qaReportKey && (
              <ArtifactDownloadButton label="QA Report" s3Key={r.qaReportKey} />
            )}
            {r.stagedDbKey && (
              <ArtifactDownloadButton label="Staged Database" s3Key={r.stagedDbKey} />
            )}
            {r.dbKey && (
              <ArtifactDownloadButton label="Base Database" s3Key={r.dbKey} />
            )}
          </div>
        </div>
      </div>
    );
  }

  // =========================================================================
  // Workflow FAILED with error — infrastructure or activity failure
  // =========================================================================
  if (data.error) {
    // Try to extract useful artifact keys from partial result if available
    const partialResult = data.result;
    const hasArtifacts = partialResult && (partialResult.qaReportKey || partialResult.stagedDbKey || partialResult.QAReportKey || partialResult.StagedDBKey);

    return (
      <div className="space-y-3">
        <div className="flex items-start gap-2 rounded-md bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle className="mt-0.5 size-4 shrink-0" />
          <span>{data.error}</span>
        </div>

        {/* Artifact access for partial results */}
        {hasArtifacts && (
          <div className="space-y-2 border-t pt-3">
            <p className="text-xs font-semibold text-muted-foreground">Available Artifacts</p>
            <div className="space-y-1">
              {(partialResult.qaReportKey || partialResult.QAReportKey) && (
                <ArtifactDownloadButton
                  label="QA Report"
                  s3Key={partialResult.qaReportKey || partialResult.QAReportKey}
                />
              )}
              {(partialResult.stagedDbKey || partialResult.StagedDBKey) && (
                <ArtifactDownloadButton
                  label="Staged Database"
                  s3Key={partialResult.stagedDbKey || partialResult.StagedDBKey}
                />
              )}
            </div>
          </div>
        )}
      </div>
    );
  }

  // No result (error-only workflows like sync-registrars, update-fx)
  if (!data.result) {
    return (
      <div className="flex items-center gap-2 py-2 text-sm text-green-600 dark:text-green-400">
        <CheckCircle2 className="size-4" />
        <span>Workflow completed successfully.</span>
      </div>
    );
  }

  // Render the result fields
  const result = data.result;

  return (
    <div className="divide-y">
      {Object.entries(result).map(([key, value]) => (
        <ResultField key={key} label={key} value={value} workflowType={workflowType} />
      ))}
    </div>
  );
}
