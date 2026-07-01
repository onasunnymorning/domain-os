"use client";

import { useState, useMemo } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { CopyButton } from "@/components/ui/copy-button";
import {
  GitCompare,
  Table,
  FileCode,
  ArrowRight,
  Eye,
  CheckCircle2,
  XCircle,
  HelpCircle,
} from "lucide-react";

interface StateDiffModalProps {
  isOpen: boolean;
  onClose: () => void;
  before: any;
  after: any;
  title?: string;
  subtitle?: string;
}

interface DiffLine {
  type: "added" | "removed" | "unchanged";
  value: string;
  oldLineNumber?: number;
  newLineNumber?: number;
}

interface ChangeItem {
  key: string;
  beforeValue: any;
  afterValue: any;
  type: "added" | "removed" | "modified";
}

// Helper to flatten nested objects recursively
function flattenObject(obj: any, prefix = ""): Record<string, any> {
  if (obj === null || obj === undefined) return {};
  if (typeof obj !== "object") {
    return { [prefix]: obj };
  }
  if (Array.isArray(obj)) {
    const result: Record<string, any> = {};
    obj.forEach((item, index) => {
      Object.assign(
        result,
        flattenObject(item, prefix ? `${prefix}.${index}` : `${index}`)
      );
    });
    return result;
  }
  const result: Record<string, any> = {};
  for (const key of Object.keys(obj)) {
    const value = obj[key];
    const newPrefix = prefix ? `${prefix}.${key}` : key;
    if (typeof value === "object" && value !== null) {
      Object.assign(result, flattenObject(value, newPrefix));
    } else {
      result[newPrefix] = value;
    }
  }
  return result;
}

// Compute key-value differences
function getChanges(before: any, after: any): ChangeItem[] {
  const flatBefore = flattenObject(before);
  const flatAfter = flattenObject(after);

  const allKeys = Array.from(
    new Set([...Object.keys(flatBefore), ...Object.keys(flatAfter)])
  );
  const changes: ChangeItem[] = [];

  for (const key of allKeys) {
    const valBefore = flatBefore[key];
    const valAfter = flatAfter[key];

    if (!(key in flatBefore)) {
      changes.push({
        key,
        beforeValue: undefined,
        afterValue: valAfter,
        type: "added",
      });
    } else if (!(key in flatAfter)) {
      changes.push({
        key,
        beforeValue: valBefore,
        afterValue: undefined,
        type: "removed",
      });
    } else if (JSON.stringify(valBefore) !== JSON.stringify(valAfter)) {
      changes.push({
        key,
        beforeValue: valBefore,
        afterValue: valAfter,
        type: "modified",
      });
    }
  }

  return changes.sort((a, b) => a.key.localeCompare(b.key));
}

// LCS algorithm to perform line-by-line diff of two arrays of strings
function diffLines(oldLines: string[], newLines: string[]): DiffLine[] {
  const M = oldLines.length;
  const N = newLines.length;

  const dp: number[][] = Array.from({ length: M + 1 }, () =>
    new Int32Array(N + 1) as any
  );

  for (let i = 1; i <= M; i++) {
    for (let j = 1; j <= N; j++) {
      if (oldLines[i - 1] === newLines[j - 1]) {
        dp[i][j] = dp[i - 1][j - 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1]);
      }
    }
  }

  const result: DiffLine[] = [];
  let i = M;
  let j = N;

  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && oldLines[i - 1] === newLines[j - 1]) {
      result.unshift({
        type: "unchanged",
        value: oldLines[i - 1],
        oldLineNumber: i,
        newLineNumber: j,
      });
      i--;
      j--;
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      result.unshift({
        type: "added",
        value: newLines[j - 1],
        newLineNumber: j,
      });
      j--;
    } else {
      result.unshift({
        type: "removed",
        value: oldLines[i - 1],
        oldLineNumber: i,
      });
      i--;
    }
  }

  return result;
}

function formatValue(value: any): string {
  if (value === null) return "null";
  if (value === undefined) return "-";
  if (typeof value === "boolean") return value ? "true" : "false";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

export function StateDiffModal({
  isOpen,
  onClose,
  before,
  after,
  title = "State Comparison",
  subtitle = "View changes to the entity state",
}: StateDiffModalProps) {
  const [activeTab, setActiveTab] = useState<string>("keys");

  // Compute key diffs
  const keyChanges = useMemo(() => getChanges(before, after), [before, after]);

  // Compute line-by-line diff
  const lineDiff = useMemo(() => {
    const beforeStr = before ? JSON.stringify(before, null, 2) : "{}";
    const afterStr = after ? JSON.stringify(after, null, 2) : "{}";
    return diffLines(beforeStr.split("\n"), afterStr.split("\n"));
  }, [before, after]);

  const rawDiffText = useMemo(() => {
    return lineDiff
      .map((line) => {
        const prefix =
          line.type === "added" ? "+ " : line.type === "removed" ? "- " : "  ";
        return prefix + line.value;
      })
      .join("\n");
  }, [lineDiff]);

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl max-h-[85vh] flex flex-col p-6 overflow-hidden">
        <DialogHeader className="pb-4 border-b border-border">
          <DialogTitle className="flex items-center gap-2 text-xl font-bold">
            <GitCompare className="h-5 w-5 text-primary animate-pulse" />
            {title}
          </DialogTitle>
          <DialogDescription className="text-sm">
            {subtitle}
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-hidden flex flex-col mt-4">
          <Tabs
            value={activeTab}
            onValueChange={setActiveTab}
            className="flex-1 flex flex-col overflow-hidden"
          >
            <div className="flex items-center justify-between mb-4">
              <TabsList className="grid grid-cols-2 w-[280px]">
                <TabsTrigger value="keys" className="flex items-center gap-1.5 cursor-pointer">
                  <Table className="h-3.5 w-3.5" />
                  Key Changes
                </TabsTrigger>
                <TabsTrigger value="json" className="flex items-center gap-1.5 cursor-pointer">
                  <FileCode className="h-3.5 w-3.5" />
                  Full JSON Diff
                </TabsTrigger>
              </TabsList>

              {activeTab === "json" && (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">Copy Diff</span>
                   <CopyButton value={rawDiffText} className="border border-border bg-card shadow-sm" />
                </div>
              )}
            </div>

            {/* TAB: Key Changes */}
            <TabsContent
              value="keys"
              className="flex-1 overflow-auto scrollbar-thin mt-0"
            >
              {keyChanges.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12 text-center border border-dashed rounded-md bg-muted/20">
                  <CheckCircle2 className="h-8 w-8 text-emerald-500 mb-2" />
                  <p className="font-medium text-sm text-foreground">No differences found</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    The before and after states are identical.
                  </p>
                </div>
              ) : (
                <div className="overflow-x-auto rounded-md border border-border">
                  <table className="w-full text-left border-collapse text-xs">
                    <thead className="bg-muted/70 text-muted-foreground font-semibold uppercase tracking-wider text-[10px] border-b border-border">
                      <tr>
                        <th className="px-4 py-3 w-[30%]">Property</th>
                        <th className="px-4 py-3 w-[15%] text-center">Change</th>
                        <th className="px-4 py-3 w-[27.5%]">Before</th>
                        <th className="px-4 py-3 w-[27.5%]">After</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border font-mono">
                      {keyChanges.map((change) => {
                        let badgeColor = "bg-amber-100 text-amber-800 dark:bg-amber-950/40 dark:text-amber-400 border-amber-200/50";
                        let beforeBg = "";
                        let afterBg = "";
                        let changeLabel = "Modified";

                        if (change.type === "added") {
                          badgeColor = "bg-emerald-100 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-400 border-emerald-200/50";
                          afterBg = "bg-emerald-50 dark:bg-emerald-950/10 text-emerald-600 dark:text-emerald-400";
                          changeLabel = "Added";
                        } else if (change.type === "removed") {
                          badgeColor = "bg-red-100 text-red-800 dark:bg-red-950/40 dark:text-red-400 border-red-200/50";
                          beforeBg = "bg-red-50 dark:bg-red-950/10 text-red-600 dark:text-red-400 line-through decoration-red-400/50";
                          changeLabel = "Removed";
                        } else {
                          beforeBg = "text-muted-foreground/80";
                          afterBg = "text-foreground font-medium";
                        }

                        return (
                          <tr
                            key={change.key}
                            className="hover:bg-muted/40 transition-colors"
                          >
                            <td className="px-4 py-2.5 font-medium text-foreground text-[11px] break-all select-all align-top">
                              {change.key}
                            </td>
                            <td className="px-4 py-2.5 text-center align-top select-none">
                              <Badge
                                variant="outline"
                                className={`text-[10px] px-1.5 py-0 font-sans tracking-wide uppercase ${badgeColor}`}
                              >
                                {changeLabel}
                              </Badge>
                            </td>
                            <td className={`px-4 py-2.5 break-all max-w-[250px] align-top text-[11px] ${beforeBg}`}>
                              {formatValue(change.beforeValue)}
                            </td>
                            <td className={`px-4 py-2.5 break-all max-w-[250px] align-top text-[11px] ${afterBg}`}>
                              {formatValue(change.afterValue)}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </TabsContent>

            {/* TAB: Full JSON Diff */}
            <TabsContent
              value="json"
              className="flex-1 overflow-auto rounded-md bg-zinc-950 border border-zinc-800 p-2 scrollbar-thin mt-0"
            >
              <pre className="text-zinc-100 text-[11px] font-mono leading-relaxed select-text min-h-full">
                <code>
                  {lineDiff.map((line, idx) => {
                    let lineClass = "text-zinc-400";
                    let bgClass = "";
                    let prefix = "  ";

                    if (line.type === "added") {
                      lineClass = "text-emerald-400 font-medium";
                      bgClass = "bg-emerald-950/30 border-l-2 border-emerald-500 pl-0.5";
                      prefix = "+ ";
                    } else if (line.type === "removed") {
                      lineClass = "text-red-400 font-medium";
                      bgClass = "bg-red-950/30 border-l-2 border-red-500 pl-0.5";
                      prefix = "- ";
                    }

                    return (
                      <div
                        key={idx}
                        className={`flex items-start ${bgClass} hover:bg-zinc-900/50 py-0.5`}
                      >
                        {/* Line numbers (not selectable) */}
                        <span className="w-10 text-right pr-3 select-none text-zinc-600 text-[10px] tabular-nums font-sans shrink-0">
                          {line.oldLineNumber || ""}
                        </span>
                        <span className="w-10 text-right pr-3 select-none text-zinc-600 text-[10px] tabular-nums font-sans border-r border-zinc-800 shrink-0">
                          {line.newLineNumber || ""}
                        </span>
                        {/* Content */}
                        <span
                          className={`pl-3 whitespace-pre-wrap break-all ${lineClass}`}
                        >
                          {prefix}
                          {line.value}
                        </span>
                      </div>
                    );
                  })}
                </code>
              </pre>
            </TabsContent>
          </Tabs>
        </div>

        <div className="flex justify-end pt-4 border-t border-border mt-4">
          <Button
            variant="outline"
            size="sm"
            onClick={onClose}
            className="cursor-pointer"
          >
            Close Comparison
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
