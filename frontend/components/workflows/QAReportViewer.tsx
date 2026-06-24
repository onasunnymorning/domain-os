'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Info,
  ChevronDown,
  ChevronRight,
  Loader2,
  FileWarning,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { getStorageDownloadURL } from '@/lib/api/workflows';

// =============================================================================
// QA Report types (matches backend QAReport / QACheck structs)
// =============================================================================

interface QACheck {
  rule: string;
  description: string;
  severity: 'error' | 'warning' | 'info';
  passed: boolean;
  affectedCount: number;
  message: string;
  detail?: Record<string, any>;
  sampledItems?: Record<string, any>[];
}

interface QAReport {
  version: string;
  timestamp: string;
  pipeline: string;
  context?: Record<string, string>;
  sourceKey?: string;
  passed: boolean;
  summary?: Record<string, number>;
  checks: QACheck[];
}

// =============================================================================
// Helpers
// =============================================================================

function severityIcon(severity: string, passed: boolean) {
  if (passed) return <CheckCircle2 className="size-3.5 text-green-500" />;
  switch (severity) {
    case 'error':
      return <XCircle className="size-3.5 text-red-500" />;
    case 'warning':
      return <AlertTriangle className="size-3.5 text-amber-500" />;
    default:
      return <Info className="size-3.5 text-blue-500" />;
  }
}

function severityBorder(severity: string, passed: boolean) {
  if (passed) return 'border-green-500/20';
  switch (severity) {
    case 'error':
      return 'border-red-500/30';
    case 'warning':
      return 'border-amber-500/30';
    default:
      return 'border-blue-500/20';
  }
}

// =============================================================================
// Sampled Items Table
// =============================================================================

function SampledItemsTable({ items, totalCount }: { items: Record<string, any>[]; totalCount: number }) {
  if (!items || items.length === 0) return null;

  // Derive columns from the first item's keys
  const columns = Object.keys(items[0]);

  return (
    <div className="mt-2 space-y-1">
      <div className="text-muted-foreground text-[10px] uppercase tracking-wider">
        Sample ({items.length} of {totalCount.toLocaleString()})
      </div>
      <div className="overflow-x-auto rounded border">
        <table className="w-full text-xs">
          <thead>
            <tr className="bg-muted/50">
              {columns.map((col) => (
                <th key={col} className="whitespace-nowrap px-2 py-1 text-left font-medium">
                  {col}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.map((item, i) => (
              <tr key={i} className="border-t">
                {columns.map((col) => (
                  <td key={col} className="text-muted-foreground max-w-[200px] truncate whitespace-nowrap px-2 py-1 font-mono">
                    {formatCellValue(item[col])}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function formatCellValue(value: any): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'string' && value.match(/^\d{4}-\d{2}-\d{2}T/)) {
    // ISO date → friendly format
    try {
      return new Date(value).toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      });
    } catch {
      return value;
    }
  }
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

// =============================================================================
// Single Check Row
// =============================================================================

function QACheckRow({ check }: { check: QACheck }) {
  const [expanded, setExpanded] = useState(!check.passed && check.sampledItems && check.sampledItems.length > 0);
  const hasSamples = check.sampledItems && check.sampledItems.length > 0;

  return (
    <div className={cn('border-b last:border-b-0', severityBorder(check.severity, check.passed))}>
      <button
        type="button"
        onClick={() => hasSamples && setExpanded(!expanded)}
        className={cn(
          'flex w-full items-start gap-2 px-3 py-2 text-left text-xs',
          hasSamples && 'hover:bg-muted/50 cursor-pointer',
          !hasSamples && 'cursor-default'
        )}
      >
        <div className="mt-0.5 shrink-0">{severityIcon(check.severity, check.passed)}</div>
        <div className="min-w-0 flex-1">
          <div className="font-medium">{check.message}</div>
          <div className="text-muted-foreground mt-0.5">{check.description}</div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {!check.passed && check.affectedCount > 0 && (
            <Badge
              variant={check.severity === 'error' ? 'destructive' : 'secondary'}
              className="text-[10px] px-1.5 py-0"
            >
              {check.affectedCount.toLocaleString()}
            </Badge>
          )}
          {hasSamples && (
            <div className="text-muted-foreground">
              {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
            </div>
          )}
        </div>
      </button>

      {expanded && hasSamples && (
        <div className="bg-muted/20 border-t px-3 pb-3">
          <SampledItemsTable items={check.sampledItems!} totalCount={check.affectedCount} />
        </div>
      )}
    </div>
  );
}

// =============================================================================
// Main QA Report Viewer
// =============================================================================

async function fetchQAReport(s3Key: string): Promise<QAReport> {
  const { url } = await getStorageDownloadURL(s3Key);
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to fetch QA report: HTTP ${response.status}`);
  }
  return response.json();
}

export function QAReportViewer({ s3Key }: { s3Key: string }) {
  const { data: report, isLoading, error } = useQuery({
    queryKey: ['qa-report', s3Key],
    queryFn: () => fetchQAReport(s3Key),
    staleTime: Infinity, // QA reports don't change
    retry: 1,
  });

  if (isLoading) {
    return (
      <div className="text-muted-foreground flex items-center gap-2 py-2 text-xs">
        <Loader2 className="size-3.5 animate-spin" />
        <span>Loading QA report…</span>
      </div>
    );
  }

  if (error || !report) {
    return (
      <div className="text-muted-foreground flex items-center gap-2 py-2 text-xs">
        <FileWarning className="size-3.5" />
        <span>Unable to load QA report.</span>
      </div>
    );
  }

  const failedErrors = report.checks.filter((c) => !c.passed && c.severity === 'error');
  const failedWarnings = report.checks.filter((c) => !c.passed && c.severity === 'warning');
  const passedChecks = report.checks.filter((c) => c.passed);

  return (
    <div className="space-y-2">
      {/* Header */}
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-xs">QA Checks</span>
        <div className="flex items-center gap-2">
          {failedErrors.length > 0 && (
            <Badge variant="destructive" className="text-[10px] px-1.5 py-0">
              {failedErrors.length} error{failedErrors.length !== 1 ? 's' : ''}
            </Badge>
          )}
          {failedWarnings.length > 0 && (
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0 text-amber-600 dark:text-amber-400">
              {failedWarnings.length} warning{failedWarnings.length !== 1 ? 's' : ''}
            </Badge>
          )}
          {failedErrors.length === 0 && failedWarnings.length === 0 && (
            <Badge variant="default" className="text-[10px] px-1.5 py-0">
              All passed
            </Badge>
          )}
        </div>
      </div>

      {/* Failed checks first, then passed */}
      <div className="overflow-hidden rounded-lg border">
        {/* Errors first */}
        {failedErrors.map((check) => (
          <QACheckRow key={check.rule} check={check} />
        ))}
        {/* Warnings */}
        {failedWarnings.map((check) => (
          <QACheckRow key={check.rule} check={check} />
        ))}
        {/* Passed checks (collapsed by default) */}
        {passedChecks.length > 0 && (
          <PassedChecksAccordion checks={passedChecks} />
        )}
      </div>
    </div>
  );
}

// =============================================================================
// Passed Checks Accordion (collapsed by default to reduce noise)
// =============================================================================

function PassedChecksAccordion({ checks }: { checks: QACheck[] }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <>
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="hover:bg-muted/50 flex w-full items-center gap-2 border-t px-3 py-2 text-left text-xs"
      >
        <CheckCircle2 className="size-3.5 text-green-500" />
        <span className="text-muted-foreground flex-1">
          {checks.length} check{checks.length !== 1 ? 's' : ''} passed
        </span>
        {expanded ? <ChevronDown className="text-muted-foreground size-3.5" /> : <ChevronRight className="text-muted-foreground size-3.5" />}
      </button>
      {expanded && checks.map((check) => (
        <QACheckRow key={check.rule} check={check} />
      ))}
    </>
  );
}
