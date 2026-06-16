"use client";

import { DomainStatus } from "@/lib/types/domain";
import { Lock, Unlock, AlertCircle, Clock, Repeat, Copy } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useState } from "react";


interface Props {
  status?: DomainStatus;
}

export function DomainStatusWidget({ status }: Props) {
  const [showRaw, setShowRaw] = useState(false);

  if (!status) return <span className="text-muted-foreground">-</span>;

  // Detect basic states
  const hasClientProhibitions =
    status.ClientHold ||
    status.ClientUpdateProhibited ||
    status.ClientDeleteProhibited ||
    status.ClientTransferProhibited ||
    status.ClientRenewProhibited;

  const hasServerProhibitions =
    status.ServerHold ||
    status.ServerUpdateProhibited ||
    status.ServerDeleteProhibited ||
    status.ServerTransferProhibited ||
    status.ServerRenewProhibited;

  const hasPending =
    status.PendingCreate ||
    status.PendingUpdate ||
    status.PendingDelete ||
    status.PendingTransfer ||
    status.PendingRenew ||
    status.PendingRestore;

  const hasAnyProhibition = hasClientProhibitions || hasServerProhibitions;

  // The matrix rows
  const prohibitions = [
    { label: "Update", client: status.ClientUpdateProhibited, server: status.ServerUpdateProhibited },
    { label: "Delete", client: status.ClientDeleteProhibited, server: status.ServerDeleteProhibited },
    { label: "Transfer", client: status.ClientTransferProhibited, server: status.ServerTransferProhibited },
    { label: "Renew", client: status.ClientRenewProhibited, server: status.ServerRenewProhibited },
    { label: "Hold", client: status.ClientHold, server: status.ServerHold },
  ];

  return (
    <div className="space-y-4">
      {/* Primary Top-level State Indicator and Actions */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {status.OK && !hasAnyProhibition && !hasPending && (
            <div className="flex items-center gap-2 text-stone-600 bg-stone-50 px-3 py-1.5 rounded-md border border-stone-200">
              <span className="font-semibold text-sm">Active / OK</span>
            </div>
          )}
          {status.Inactive && (
            <div className="flex items-center gap-2 text-amber-600 bg-amber-50 px-3 py-1.5 rounded-md border border-amber-200">
              <AlertCircle className="h-5 w-5" />
              <span className="font-semibold text-sm">Inactive</span>
            </div>
          )}
          {!status.OK && !status.Inactive && !hasAnyProhibition && !hasPending && (
             <div className="flex items-center gap-2 text-muted-foreground bg-muted px-3 py-1.5 rounded-md border">
               <span className="font-semibold text-sm">No specific status</span>
             </div>
          )}
          
          {/* Pending Badges */}
          {status.PendingCreate && <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200"><Clock className="w-3 h-3" /> Pending Create</Badge>}
          {status.PendingUpdate && <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200"><Clock className="w-3 h-3" /> Pending Update</Badge>}
          {status.PendingDelete && <Badge variant="destructive" className="flex gap-1"><Clock className="w-3 h-3" /> Pending Delete</Badge>}
          {status.PendingTransfer && <Badge variant="secondary" className="flex gap-1 bg-amber-100 text-amber-800 border-amber-200"><Clock className="w-3 h-3" /> Pending Transfer</Badge>}
          {status.PendingRenew && <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200"><Clock className="w-3 h-3" /> Pending Renew</Badge>}
          {status.PendingRestore && <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200"><Clock className="w-3 h-3" /> Pending Restore</Badge>}
        </div>
      </div>

      <div className="rounded-md border overflow-hidden relative">
        <button
          onClick={() => setShowRaw(!showRaw)}
          className="absolute top-2 right-2 p-1.5 z-10 text-muted-foreground hover:text-foreground bg-background/50 hover:bg-background rounded-md transition-colors"
          title={showRaw ? "Show Visual Dashboard" : "Show Raw JSON"}
          aria-label="Toggle raw JSON"
        >
          <Repeat className="w-4 h-4" />
        </button>

        {showRaw ? (
          <div className="bg-muted text-xs p-4 pt-10 overflow-x-auto group">
            <button
              onClick={async () => {
                try {
                  const arr = Object.entries(status).filter(([, v]) => Boolean(v)).map(([k]) => k);
                  await navigator.clipboard.writeText(JSON.stringify(arr, null, 2));
                } catch {}
              }}
              className="absolute top-2 right-10 p-1.5 z-10 text-muted-foreground hover:text-foreground bg-background/50 hover:bg-background rounded-md transition-colors opacity-0 group-hover:opacity-100"
              title="Copy JSON array"
              aria-label="Copy JSON"
            >
              <Copy className="h-4 w-4" />
            </button>
            <pre><code>{JSON.stringify(Object.entries(status).filter(([, v]) => Boolean(v)).map(([k]) => k), null, 2)}</code></pre>
          </div>
        ) : (
          <table className="w-full text-sm text-left">
            <thead className="bg-muted/50 border-b">
              <tr>
                <th className="px-3 py-2 font-medium">Prohibition</th>
                <th className="px-3 py-2 font-medium text-center">Client</th>
                <th className="px-3 py-2 pr-10 font-medium text-center">Server</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {prohibitions.map((row) => (
                <tr key={row.label}>
                  <td className="px-3 py-2 font-medium bg-muted/20">{row.label}</td>
                  <td className="px-3 py-2 text-center">
                    <div className="flex justify-center">
                      {row.client ? (
                        <Lock className="w-4 h-4 text-destructive" aria-label={`Client ${row.label} Prohibited`} />
                      ) : (
                        <Unlock className="w-4 h-4 text-muted-foreground/30" aria-label={`Client ${row.label} Allowed`} />
                      )}
                    </div>
                  </td>
                  <td className="px-3 py-2 text-center">
                    <div className="flex justify-center">
                      {row.server ? (
                        <Lock className="w-4 h-4 text-destructive" aria-label={`Server ${row.label} Prohibited`} />
                      ) : (
                        <Unlock className="w-4 h-4 text-muted-foreground/30" aria-label={`Server ${row.label} Allowed`} />
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
