"use client";

import { DomainStatus } from "@/lib/types/domain";
import { LockKeyhole, UnlockKeyhole, AlertCircle, Clock, Repeat, Copy, Loader2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useState } from "react";
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "@/components/ui/tooltip";
import { STATUS_LABELS, STATUS_DESCRIPTIONS } from "@/lib/constants/domainStatus";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { setDomainStatus, unsetDomainStatus } from "@/lib/api/domains";
import { useQueryClient } from "@tanstack/react-query";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

const ALLOWED_LABELS: Record<string, string> = {
  ClientTransferProhibited: "Client Transfer Allowed",
  ServerTransferProhibited: "Server Transfer Allowed",
  ClientUpdateProhibited: "Client Update Allowed",
  ServerUpdateProhibited: "Server Update Allowed",
  ClientDeleteProhibited: "Client Delete Allowed",
  ServerDeleteProhibited: "Server Delete Allowed",
  ClientRenewProhibited: "Client Renew Allowed",
  ServerRenewProhibited: "Server Renew Allowed",
  ClientHold: "No Client Hold",
  ServerHold: "No Server Hold",
};

const ALLOWED_DESCRIPTIONS: Record<string, string> = {
  ClientTransferProhibited: "Registrar allows transfer operations.",
  ServerTransferProhibited: "Registry allows transfer operations.",
  ClientUpdateProhibited: "Registrar allows update operations.",
  ServerUpdateProhibited: "Registry allows update operations.",
  ClientDeleteProhibited: "Registrar allows delete operations.",
  ServerDeleteProhibited: "Registry allows delete operations.",
  ClientRenewProhibited: "Registrar allows renew operations.",
  ServerRenewProhibited: "Registry allows renew operations.",
  ClientHold: "Registrar allows DNS publishing (domain resolves).",
  ServerHold: "Registry allows DNS publishing (domain resolves).",
};

interface Props {
  status?: DomainStatus;
  domainName?: string;
}

export function DomainStatusWidget({ status, domainName }: Props) {
  const [showRaw, setShowRaw] = useState(false);
  const [isEditable, setIsEditable] = useState(false);
  const [pendingStatusToggle, setPendingStatusToggle] = useState<{
    key: string;
    label: string;
    newValue: boolean;
  } | null>(null);
  const [isUpdating, setIsUpdating] = useState(false);

  const queryClient = useQueryClient();

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
    { label: "Update", client: status.ClientUpdateProhibited, server: status.ServerUpdateProhibited, clientKey: "ClientUpdateProhibited", serverKey: "ServerUpdateProhibited" },
    { label: "Delete", client: status.ClientDeleteProhibited, server: status.ServerDeleteProhibited, clientKey: "ClientDeleteProhibited", serverKey: "ServerDeleteProhibited" },
    { label: "Transfer", client: status.ClientTransferProhibited, server: status.ServerTransferProhibited, clientKey: "ClientTransferProhibited", serverKey: "ServerTransferProhibited" },
    { label: "Renew", client: status.ClientRenewProhibited, server: status.ServerRenewProhibited, clientKey: "ClientRenewProhibited", serverKey: "ServerRenewProhibited" },
    { label: "Hold", client: status.ClientHold, server: status.ServerHold, clientKey: "ClientHold", serverKey: "ServerHold" },
  ];

  const handleConfirmToggle = async () => {
    if (!pendingStatusToggle || !domainName) return;
    setIsUpdating(true);
    try {
      const backendKey = pendingStatusToggle.key.charAt(0).toLowerCase() + pendingStatusToggle.key.slice(1);
      if (pendingStatusToggle.newValue) {
        await setDomainStatus(domainName, backendKey);
      } else {
        await unsetDomainStatus(domainName, backendKey);
      }
      toast.success(`Successfully ${pendingStatusToggle.newValue ? "set" : "removed"} status ${pendingStatusToggle.label}`);
      queryClient.invalidateQueries({ queryKey: ["domain", domainName] });
    } catch (err: any) {
      const errMsg = err.response?.data?.error || err.message || "Failed to update EPP status";
      toast.error(`Error: ${errMsg}`);
    } finally {
      setIsUpdating(false);
      setPendingStatusToggle(null);
    }
  };

  return (
    <TooltipProvider delayDuration={150}>
      <div className="space-y-4">
        {/* Primary Top-level State Indicator and Actions */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {status.OK && !hasAnyProhibition && !hasPending && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <div className="flex items-center gap-2 text-stone-600 bg-stone-50 px-3 py-1.5 rounded-md border border-stone-200 cursor-help">
                    <span className="font-semibold text-sm">Active / OK</span>
                  </div>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Active / OK</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.OK}</div>
                </TooltipContent>
              </Tooltip>
            )}
            {status.Inactive && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <div className="flex items-center gap-2 text-amber-600 bg-amber-50 px-3 py-1.5 rounded-md border border-amber-200 cursor-help">
                    <AlertCircle className="h-5 w-5" />
                    <span className="font-semibold text-sm">Inactive</span>
                  </div>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Inactive</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.Inactive}</div>
                </TooltipContent>
              </Tooltip>
            )}
            {!status.OK && !status.Inactive && !hasAnyProhibition && !hasPending && (
               <div className="flex items-center gap-2 text-muted-foreground bg-muted px-3 py-1.5 rounded-md border">
                 <span className="font-semibold text-sm">No specific status</span>
               </div>
            )}
            
            {/* Pending Badges */}
            {status.PendingCreate && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200 cursor-help">
                    <Clock className="w-3 h-3" /> Pending Create
                  </Badge>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Pending Create</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.PendingCreate}</div>
                </TooltipContent>
              </Tooltip>
            )}
            {status.PendingUpdate && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200 cursor-help">
                    <Clock className="w-3 h-3" /> Pending Update
                  </Badge>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Pending Update</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.PendingUpdate}</div>
                </TooltipContent>
              </Tooltip>
            )}
            {status.PendingDelete && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="destructive" className="flex gap-1 cursor-help">
                    <Clock className="w-3 h-3" /> Pending Delete
                  </Badge>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Pending Delete</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.PendingDelete}</div>
                </TooltipContent>
              </Tooltip>
            )}
            {status.PendingTransfer && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="secondary" className="flex gap-1 bg-amber-100 text-amber-800 border-amber-200 cursor-help">
                    <Clock className="w-3 h-3" /> Pending Transfer
                  </Badge>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Pending Transfer</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.PendingTransfer}</div>
                </TooltipContent>
              </Tooltip>
            )}
            {status.PendingRenew && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200 cursor-help">
                    <Clock className="w-3 h-3" /> Pending Renew
                  </Badge>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Pending Renew</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.PendingRenew}</div>
                </TooltipContent>
              </Tooltip>
            )}
            {status.PendingRestore && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="secondary" className="flex gap-1 bg-blue-100 text-blue-800 border-blue-200 cursor-help">
                    <Clock className="w-3 h-3" /> Pending Restore
                  </Badge>
                </TooltipTrigger>
                <TooltipContent side="top" align="center">
                  <div className="font-semibold">Pending Restore</div>
                  <div className="text-xs text-muted-foreground">{STATUS_DESCRIPTIONS.PendingRestore}</div>
                </TooltipContent>
              </Tooltip>
            )}
          </div>
          {domainName && (
            <Button
              variant={isEditable ? "destructive" : "outline"}
              size="sm"
              onClick={() => setIsEditable(!isEditable)}
              className="h-8 gap-1.5 font-medium transition-all duration-200 shadow-sm"
            >
              {isEditable ? (
                <>
                  <UnlockKeyhole className="w-3.5 h-3.5" />
                  Lock Editing
                </>
              ) : (
                <>
                  <LockKeyhole className="w-3.5 h-3.5" />
                  Unlock Statuses
                </>
              )}
            </Button>
          )}
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
                {prohibitions.map((row) => {
                  const clientKey = row.clientKey;
                  const clientTitle = row.client ? STATUS_LABELS[clientKey] : ALLOWED_LABELS[clientKey];
                  const clientDesc = row.client ? STATUS_DESCRIPTIONS[clientKey] : ALLOWED_DESCRIPTIONS[clientKey];

                  const serverKey = row.serverKey;
                  const serverTitle = row.server ? STATUS_LABELS[serverKey] : ALLOWED_LABELS[serverKey];
                  const serverDesc = row.server ? STATUS_DESCRIPTIONS[serverKey] : ALLOWED_DESCRIPTIONS[serverKey];

                  const handleClientClick = (e: React.MouseEvent) => {
                    if (!isEditable) return;
                    e.stopPropagation();
                    setPendingStatusToggle({
                      key: clientKey,
                      label: STATUS_LABELS[clientKey] || clientKey,
                      newValue: !row.client,
                    });
                  };

                  const handleServerClick = (e: React.MouseEvent) => {
                    if (!isEditable) return;
                    e.stopPropagation();
                    setPendingStatusToggle({
                      key: serverKey,
                      label: STATUS_LABELS[serverKey] || serverKey,
                      newValue: !row.server,
                    });
                  };

                  return (
                    <tr key={row.label} className="transition-colors hover:bg-muted/10">
                      <td className="px-3 py-2 font-medium bg-muted/20">{row.label}</td>
                      <td 
                        className={`px-3 py-2 text-center transition-colors ${
                          isEditable 
                            ? "cursor-pointer hover:bg-red-50/50 dark:hover:bg-red-950/20 active:bg-red-100/50" 
                            : ""
                        }`}
                        onClick={handleClientClick}
                      >
                        <div className="flex justify-center">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button 
                                className={`focus:outline-none focus:ring-1 focus:ring-primary focus:rounded-full p-1 ${isEditable ? "cursor-pointer" : "cursor-help"}`}
                                type="button"
                                disabled={isUpdating}
                                onClick={handleClientClick}
                              >
                                {row.client ? (
                                  <LockKeyhole className="w-4 h-4 text-red-500 transition-transform duration-200 hover:scale-110" aria-label={clientTitle} />
                                ) : (
                                  <UnlockKeyhole className="w-4 h-4 text-muted-foreground/30 transition-transform duration-200 hover:scale-110" aria-label={clientTitle} />
                                )}
                              </button>
                            </TooltipTrigger>
                            <TooltipContent side="top" align="center" className="max-w-xs text-xs">
                              <div className="font-semibold">{clientTitle}</div>
                              <div className="text-muted-foreground">{clientDesc}</div>
                              {isEditable && (
                                <div className="mt-1.5 text-[10px] font-medium text-red-500">
                                  Click to {row.client ? "allow" : "prohibit"} {row.label.toLowerCase()} operations
                                </div>
                              )}
                            </TooltipContent>
                          </Tooltip>
                        </div>
                      </td>
                      <td 
                        className={`px-3 py-2 text-center transition-colors ${
                          isEditable 
                            ? "cursor-pointer hover:bg-red-50/50 dark:hover:bg-red-950/20 active:bg-red-100/50" 
                            : ""
                        }`}
                        onClick={handleServerClick}
                      >
                        <div className="flex justify-center">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button 
                                className={`focus:outline-none focus:ring-1 focus:ring-primary focus:rounded-full p-1 ${isEditable ? "cursor-pointer" : "cursor-help"}`}
                                type="button"
                                disabled={isUpdating}
                                onClick={handleServerClick}
                              >
                                {row.server ? (
                                  <LockKeyhole className="w-4 h-4 text-red-500 transition-transform duration-200 hover:scale-110" aria-label={serverTitle} />
                                ) : (
                                  <UnlockKeyhole className="w-4 h-4 text-muted-foreground/30 transition-transform duration-200 hover:scale-110" aria-label={serverTitle} />
                                )}
                              </button>
                            </TooltipTrigger>
                            <TooltipContent side="top" align="center" className="max-w-xs text-xs">
                              <div className="font-semibold">{serverTitle}</div>
                              <div className="text-muted-foreground">{serverDesc}</div>
                              {isEditable && (
                                <div className="mt-1.5 text-[10px] font-medium text-red-500">
                                  Click to {row.server ? "allow" : "prohibit"} {row.label.toLowerCase()} operations
                                </div>
                              )}
                            </TooltipContent>
                          </Tooltip>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Safety Confirmation Dialog */}
      <AlertDialog 
        open={pendingStatusToggle !== null} 
        onOpenChange={(open) => {
          if (!open) setPendingStatusToggle(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2 text-lg">
              {pendingStatusToggle?.newValue ? (
                <>
                  <LockKeyhole className="w-5 h-5 text-red-500" />
                  Enable {pendingStatusToggle?.label}?
                </>
              ) : (
                <>
                  <UnlockKeyhole className="w-5 h-5 text-zinc-500" />
                  Disable {pendingStatusToggle?.label}?
                </>
              )}
            </AlertDialogTitle>
            <AlertDialogDescription className="text-sm pt-2">
              {pendingStatusToggle?.newValue ? (
                <span>
                  Are you sure you want to <strong>restrict</strong> operations by setting EPP status <strong>{pendingStatusToggle?.label}</strong> on <strong>{domainName}</strong>?
                </span>
              ) : (
                <span>
                  Are you sure you want to <strong>allow</strong> operations by removing EPP status restriction <strong>{pendingStatusToggle?.label}</strong> from <strong>{domainName}</strong>?
                </span>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter className="pt-4">
            <AlertDialogCancel disabled={isUpdating}>Cancel</AlertDialogCancel>
            <AlertDialogAction 
              onClick={(e) => {
                e.preventDefault();
                handleConfirmToggle();
              }}
              disabled={isUpdating}
              className={pendingStatusToggle?.newValue ? "bg-red-600 hover:bg-red-700 text-white" : "bg-zinc-900 hover:bg-zinc-800 text-white dark:bg-zinc-50 dark:hover:bg-zinc-200 dark:text-zinc-900"}
            >
              {isUpdating ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  Updating...
                </>
              ) : (
                pendingStatusToggle?.newValue ? "Restrict" : "Allow"
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </TooltipProvider>
  );
}


