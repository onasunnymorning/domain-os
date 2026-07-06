'use client';

import { useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { ChevronDown, Wrench } from 'lucide-react';
import type { Evidence } from '@/lib/api/agent';

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EvidenceDrawer({ evidence }: { evidence: Evidence[] }) {
  const [isOpen, setIsOpen] = useState(false);

  if (!evidence || evidence.length === 0) return null;

  return (
    <div className="mt-3 rounded-lg border border-border/40 bg-muted/10 transition-colors">
      {/* Summary trigger */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="flex w-full items-center gap-2 px-3 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
      >
        <Wrench className="h-3 w-3" />
        <span>
          Based on {evidence.length} tool call{evidence.length !== 1 ? 's' : ''}
        </span>
        <ChevronDown
          className={cn(
            'ml-auto h-3 w-3 transition-transform duration-200',
            isOpen && 'rotate-180',
          )}
        />
      </button>

      {/* Expanded evidence list */}
      <div
        className={cn(
          'grid transition-all duration-300 ease-in-out',
          isOpen ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0',
        )}
      >
        <div className="overflow-hidden">
          <div className="space-y-2 border-t border-border/30 px-3 py-2">
            {evidence.map((item, idx) => (
              <EvidenceEntry key={`${item.tool}-${idx}`} item={item} />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Individual evidence entry
// ---------------------------------------------------------------------------

function EvidenceEntry({ item }: { item: Evidence }) {
  const [showResult, setShowResult] = useState(false);

  return (
    <div className="rounded-md border border-border/30 bg-background/50 p-2 text-xs animate-in fade-in duration-200">
      {/* Tool name + input */}
      <div className="flex items-start gap-2">
        <Badge variant="secondary" className="shrink-0 text-[10px] px-1.5 py-0 font-mono">
          {item.tool}
        </Badge>
        {item.input && (
          <code className="flex-1 break-all rounded bg-muted/50 px-1.5 py-0.5 text-[10px] text-muted-foreground font-mono">
            {typeof item.input === 'string' ? item.input : JSON.stringify(item.input)}
          </code>
        )}
      </div>

      {/* Result toggle */}
      {item.result !== undefined && item.result !== null && (
        <div className="mt-1.5">
          <button
            type="button"
            onClick={() => setShowResult(!showResult)}
            className="inline-flex items-center gap-1 text-[10px] text-muted-foreground/70 hover:text-muted-foreground transition-colors"
          >
            <ChevronDown
              className={cn(
                'h-2.5 w-2.5 transition-transform duration-200',
                showResult && 'rotate-180',
              )}
            />
            {showResult ? 'Hide result' : 'Show result'}
          </button>

          {showResult && (
            <pre className="mt-1 max-h-40 overflow-auto rounded bg-muted/30 p-2 text-[10px] text-foreground/70 font-mono leading-relaxed animate-in fade-in slide-in-from-top-1 duration-200">
              {typeof item.result === 'string'
                ? item.result
                : JSON.stringify(item.result, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
