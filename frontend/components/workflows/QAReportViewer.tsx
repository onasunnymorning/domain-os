'use client';

import { useQuery } from '@tanstack/react-query';
import {
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Info,
  Loader2,
  FileWarning,
  ChevronDown,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
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
  if (passed && severity !== 'info') return <CheckCircle2 className="size-3.5 text-green-500 shrink-0" />;
  switch (severity) {
    case 'error':
      return <XCircle className="size-3.5 text-red-500 shrink-0" />;
    case 'warning':
      return <AlertTriangle className="size-3.5 text-amber-500 shrink-0" />;
    default:
      return <Info className="size-3.5 text-blue-500 shrink-0" />;
  }
}

function severityBg(severity: string, passed: boolean) {
  if (passed && severity !== 'info') return '';
  switch (severity) {
    case 'error':   return 'bg-red-500/5';
    case 'warning': return 'bg-amber-500/5';
    default:        return 'bg-blue-500/5';
  }
}

function formatCellValue(value: any): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'string' && value.match(/^\d{4}-\d{2}-\d{2}T/)) {
    try {
      return new Date(value).toLocaleDateString('en-US', {
        year: 'numeric', month: 'short', day: 'numeric',
      });
    } catch {
      return value;
    }
  }
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

// =============================================================================
// Sampled Items Table
// =============================================================================

function SampledItemsTable({ items, totalCount }: { items: Record<string, any>[]; totalCount: number }) {
  if (!items || items.length === 0) return null;
  const columns = Object.keys(items[0]);

  return (
    <div className="space-y-1.5">
      <div className="text-muted-foreground text-[10px] uppercase tracking-wider">
        Sample ({items.length} of {totalCount.toLocaleString()})
      </div>
      <div className="overflow-x-auto rounded border">
        <table className="w-full text-xs">
          <thead>
            <tr className="bg-muted/50">
              {columns.map((col) => (
                <th key={col} className="whitespace-nowrap px-2 py-1.5 text-left font-medium text-muted-foreground">
                  {col}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.map((item, i) => (
              <tr key={i} className="border-t">
                {columns.map((col) => (
                  <td key={col} className="text-muted-foreground max-w-[200px] truncate whitespace-nowrap px-2 py-1.5 font-mono">
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

// =============================================================================
// Single Check Row — uses Accordion for smooth expand/collapse
// =============================================================================

function QACheckRow({ check, value }: { check: QACheck; value: string }) {
  const hasSamples = check.sampledItems && check.sampledItems.length > 0;
  const showCount = (!check.passed && check.affectedCount > 0) || (check.severity === 'info' && check.affectedCount > 0);

  if (!hasSamples) {
    // Non-expandable row — plain div, no accordion overhead
    return (
      <div className={cn(
        'flex items-start gap-2 px-3 py-2.5 text-xs border-b last:border-b-0',
        severityBg(check.severity, check.passed)
      )}>
        <div className="mt-0.5">{severityIcon(check.severity, check.passed)}</div>
        <div className="min-w-0 flex-1">
          <div className="font-medium leading-snug">{check.message}</div>
          <div className="text-muted-foreground mt-0.5 leading-snug">{check.description}</div>
        </div>
        {showCount && (
          <Badge
            variant={check.severity === 'error' ? 'destructive' : 'secondary'}
            className="text-[10px] px-1.5 py-0 shrink-0"
          >
            {check.affectedCount.toLocaleString()}
          </Badge>
        )}
      </div>
    );
  }

  return (
    <AccordionItem
      value={value}
      className={cn(
        'border-b last:border-b-0',
        severityBg(check.severity, check.passed)
      )}
    >
      <AccordionTrigger className="px-3 py-2.5 text-xs hover:no-underline hover:bg-muted/40 transition-colors [&>svg]:hidden group">
        <div className="flex items-start gap-2 w-full">
          <div className="mt-0.5">{severityIcon(check.severity, check.passed)}</div>
          <div className="min-w-0 flex-1 text-left">
            <div className="font-medium leading-snug">{check.message}</div>
            <div className="text-muted-foreground mt-0.5 leading-snug">{check.description}</div>
          </div>
          <div className="flex items-center gap-1.5 shrink-0">
            {showCount && (
              <Badge
                variant={check.severity === 'error' ? 'destructive' : 'secondary'}
                className="text-[10px] px-1.5 py-0"
              >
                {check.affectedCount.toLocaleString()}
              </Badge>
            )}
            <ChevronDown className="size-3.5 text-muted-foreground transition-transform duration-200 group-data-[state=open]:rotate-180 shrink-0" />
          </div>
        </div>
      </AccordionTrigger>
      <AccordionContent className="px-3 pb-3 pt-0">
        <SampledItemsTable items={check.sampledItems!} totalCount={check.affectedCount} />
      </AccordionContent>
    </AccordionItem>
  );
}

// =============================================================================
// Fetch
// =============================================================================

async function fetchQAReport(s3Key: string): Promise<QAReport> {
  const { url } = await getStorageDownloadURL(s3Key);
  const response = await fetch(url);
  if (!response.ok) throw new Error(`Failed to fetch QA report: HTTP ${response.status}`);
  return response.json();
}

// =============================================================================
// Main QA Report Viewer
// =============================================================================

export function QAReportViewer({ s3Key }: { s3Key: string }) {
  const { data: report, isLoading, error } = useQuery({
    queryKey: ['qa-report', s3Key],
    queryFn: () => fetchQAReport(s3Key),
    staleTime: Infinity,
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

  const failedErrors   = report.checks.filter((c) => !c.passed && c.severity === 'error');
  const failedWarnings = report.checks.filter((c) => !c.passed && c.severity === 'warning');
  const infoChecks     = report.checks.filter((c) => c.severity === 'info' && c.affectedCount > 0);
  const passedChecks   = report.checks.filter(
    (c) => c.passed && !(c.severity === 'info' && c.affectedCount > 0)
  );

  const allTopChecks = [...failedErrors, ...failedWarnings, ...infoChecks];

  return (
    <div className="space-y-2">
      {/* Header */}
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-xs">QA Checks</span>
        <div className="flex items-center gap-1.5">
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
          {infoChecks.length > 0 && (
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0 text-blue-600 dark:text-blue-400">
              {infoChecks.length} info
            </Badge>
          )}
          {failedErrors.length === 0 && failedWarnings.length === 0 && infoChecks.length === 0 && (
            <Badge variant="default" className="text-[10px] px-1.5 py-0">All passed</Badge>
          )}
        </div>
      </div>

      {/* Check list */}
      <div className="overflow-hidden rounded-lg border">
        <Accordion type="multiple" className="w-full">
          {/* Errors, warnings, info — always visible */}
          {allTopChecks.map((check) => (
            <QACheckRow key={check.rule} check={check} value={check.rule} />
          ))}

          {/* Passed checks — collapsed in a single accordion item */}
          {passedChecks.length > 0 && (
            <AccordionItem value="__passed__" className="border-b-0">
              <AccordionTrigger className="px-3 py-2.5 text-xs hover:no-underline hover:bg-muted/40 transition-colors border-t group [&>svg]:hidden">
                <div className="flex items-center gap-2 w-full">
                  <CheckCircle2 className="size-3.5 text-green-500 shrink-0" />
                  <span className="text-muted-foreground flex-1 text-left">
                    {passedChecks.length} check{passedChecks.length !== 1 ? 's' : ''} passed
                  </span>
                  <ChevronDown className="size-3.5 text-muted-foreground transition-transform duration-200 group-data-[state=open]:rotate-180 shrink-0" />
                </div>
              </AccordionTrigger>
              <AccordionContent className="pb-0">
                {passedChecks.map((check) => (
                  <QACheckRow key={check.rule} check={check} value={check.rule} />
                ))}
              </AccordionContent>
            </AccordionItem>
          )}
        </Accordion>
      </div>
    </div>
  );
}
