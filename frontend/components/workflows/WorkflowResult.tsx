'use client';

import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Download, AlertCircle, CheckCircle2, Loader2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { getWorkflowResult, getStorageDownloadURL } from '@/lib/api/workflows';
import { QAReportViewer } from './QAReportViewer';

interface WorkflowResultProps {
  workflowId: string;
  workflowType: string;
  status: string;
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

  // Failures array
  if (label === 'failures' && Array.isArray(value)) {
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
                <th className="px-3 py-1.5 text-left font-medium">Domain</th>
                <th className="px-3 py-1.5 text-left font-medium">Operation</th>
                <th className="px-3 py-1.5 text-left font-medium">Error</th>
              </tr>
            </thead>
            <tbody>
              {value.map((f: any, i: number) => (
                <tr key={i} className="border-t hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-1.5 font-mono text-xs">{f.domainName}</td>
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

export function WorkflowResult({ workflowId, workflowType, status, onStateLoaded }: WorkflowResultProps) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['workflow-result', workflowId],
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

          {(state.domainCount > 0 || state.contactCount > 0 || state.hostCount > 0) && (
            <div className="space-y-1.5 rounded-md border p-2 bg-muted/20">
              <p className="text-muted-foreground text-[10px] font-semibold uppercase tracking-wider">
                Planned deletion counts
              </p>
              <div className="grid grid-cols-3 gap-2 text-center text-xs">
                <div>
                  <div className="font-semibold tabular-nums">{state.domainCount.toLocaleString()}</div>
                  <div className="text-muted-foreground text-[10px]">Domains</div>
                </div>
                <div>
                  <div className="font-semibold tabular-nums">{state.contactCount.toLocaleString()}</div>
                  <div className="text-muted-foreground text-[10px]">Contacts</div>
                </div>
                <div>
                  <div className="font-semibold tabular-nums">{state.hostCount.toLocaleString()}</div>
                  <div className="text-muted-foreground text-[10px]">Hosts</div>
                </div>
              </div>
            </div>
          )}

          {state.stagedDbKey && (
            <div className="space-y-1.5 pt-2 border-t mt-2">
              <span className="text-muted-foreground text-xs font-semibold">Staged Database</span>
              <ArtifactDownloadButton label="Download Staged DB" s3Key={state.stagedDbKey} />
            </div>
          )}

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

  // Workflow failed with error
  if (data.error) {
    return (
      <div className="space-y-2">
        <div className="flex items-start gap-2 rounded-md bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle className="mt-0.5 size-4 shrink-0" />
          <span>{data.error}</span>
        </div>
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
