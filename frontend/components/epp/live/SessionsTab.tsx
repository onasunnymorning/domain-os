"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { format, formatDistanceToNow } from "date-fns";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { useDebounce } from "@/lib/hooks/useDebounce";
import { useEPPSessions, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPSession } from "@/lib/types/epp";
import { EndSessionsDialog, type EndSessionsTarget } from "./EndSessionsDialog";
import { LIVE_REFRESH_MS } from "./NowCards";

const PAGE = 50;
const FILTER_KEYS = ["clid", "login", "client_ip"] as const;
type FilterKey = (typeof FILTER_KEYS)[number];

/** The words for a filter, for the "End all" dialog's title. */
export function describeFilter(f: Partial<Record<FilterKey, string>>): string {
  const parts: string[] = [];
  if (f.clid) parts.push(`of registrar ${f.clid}`);
  if (f.login) parts.push(`of login ${f.login}`);
  if (f.client_ip) parts.push(`from ${f.client_ip}`);
  return parts.join(", ");
}

export function SessionsTab({ active }: { active: boolean }) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const [draft, setDraft] = useState<Record<FilterKey, string>>({
    clid: searchParams?.get("clid") ?? "",
    login: searchParams?.get("login") ?? "",
    client_ip: searchParams?.get("client_ip") ?? "",
  });
  const debounced = useDebounce(draft, 300);
  const [cursor, setCursor] = useState<string | undefined>();
  const [stack, setStack] = useState<string[]>([]);
  const [target, setTarget] = useState<EndSessionsTarget | null>(null);

  const filter = useMemo(() => {
    const f: Partial<Record<FilterKey, string>> = {};
    for (const k of FILTER_KEYS) if (debounced[k].trim()) f[k] = debounced[k].trim();
    return f;
  }, [debounced]);
  const hasFilter = Object.keys(filter).length > 0;

  const setFilter = (k: FilterKey, v: string) => {
    setDraft((d) => ({ ...d, [k]: v }));
    setCursor(undefined);
    setStack([]);
    const next = new URLSearchParams(searchParams?.toString() ?? "");
    if (v.trim()) next.set(k, v.trim());
    else next.delete(k);
    next.set("tab", "sessions");
    router.replace(`${pathname}?${next.toString()}`, { scroll: false });
  };

  const q = useEPPSessions({ ...filter, pagesize: PAGE, cursor }, { refetchInterval: active ? LIVE_REFRESH_MS : undefined, enabled: active });
  const rows = q.data?.Data ?? [];
  const next = q.data?.Meta?.PageCursor;

  return (
    <Card>
      <EPPSectionHeader
        title="Sessions"
        description="Every live EPP session on every node, as of each node's last heartbeat. Ending one disconnects that client; the registrar can log in again at once, so end a credential's sessions by retiring it as compromised on the registrar's EPP tab when that is what you mean."
        action={
          hasFilter && rows.length > 0 ? (
            <Button
              size="sm"
              variant="destructive"
              onClick={() => setTarget({ kind: "matching", filter, count: rows.length, label: describeFilter(filter) })}
            >
              End all {rows.length}
              {next ? "+" : ""} matching
            </Button>
          ) : undefined
        }
      />
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-3">
          {(
            [
              ["clid", "Registrar", "acme-rar"],
              ["login", "Login", "acme-rar"],
              ["client_ip", "Client IP", "203.0.113.9"],
            ] as [FilterKey, string, string][]
          ).map(([k, label, ph]) => (
            <div key={k} className="space-y-1">
              <Label htmlFor={`sessions-${k}`}>{label}</Label>
              <div className="relative">
                <Input id={`sessions-${k}`} value={draft[k]} onChange={(e) => setFilter(k, e.target.value)} placeholder={ph} className="pr-8 font-mono" />
                {draft[k] && (
                  <button type="button" aria-label={`Clear ${label}`} onClick={() => setFilter(k, "")} className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground">
                    <X className="h-4 w-4" />
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>

        {q.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : q.error ? (
          <div className="text-sm text-red-700 dark:text-red-400">{eppErrorMessage(q.error, "Could not read the sessions.")}</div>
        ) : rows.length === 0 ? (
          <div className="text-sm text-muted-foreground">{hasFilter ? "No session matches." : "No registrar is connected right now."}</div>
        ) : (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Registrar</TableHead>
                  <TableHead>Login</TableHead>
                  <TableHead>Client IP</TableHead>
                  <TableHead>Node</TableHead>
                  <TableHead>Connected</TableHead>
                  <TableHead>Last command</TableHead>
                  <TableHead className="text-right">Commands</TableHead>
                  <TableHead className="w-[120px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((s: EPPSession) => (
                  <TableRow key={s.id}>
                    <TableCell>
                      {s.clid ? (
                        <Link href={`/registrars/${encodeURIComponent(s.clid)}?tab=epp`} className="font-mono text-primary hover:underline">
                          {s.clid}
                        </Link>
                      ) : (
                        <span className="text-muted-foreground">not logged in</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <span className="font-mono">{s.login_id || "–"}</span>
                      {s.role === "readonly" && (
                        <Badge variant="outline" className="ml-2">
                          Read-only
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="font-mono">{s.client_ip}</TableCell>
                    <TableCell className="font-mono text-xs">{s.node}</TableCell>
                    <TableCell title={format(new Date(s.connected_at), "MMM d, yyyy HH:mm:ss")}>{formatDistanceToNow(new Date(s.connected_at))} ago</TableCell>
                    <TableCell>{s.last_command_at ? `${formatDistanceToNow(new Date(s.last_command_at))} ago` : <span className="text-muted-foreground">none yet</span>}</TableCell>
                    <TableCell className="text-right tabular-nums">{s.commands}</TableCell>
                    <TableCell className="text-right">
                      <Button size="sm" variant="ghost" onClick={() => setTarget({ kind: "one", id: s.id, label: `${s.login_id || s.client_ip}` })}>
                        End session
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        {(stack.length > 0 || next) && (
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={stack.length === 0}
              onClick={() => {
                const prev = [...stack];
                const c = prev.pop();
                setStack(prev);
                setCursor(c || undefined);
              }}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={!next}
              onClick={() => {
                setStack((s) => [...s, cursor ?? ""]);
                setCursor(next);
              }}
            >
              Next
            </Button>
          </div>
        )}
      </CardContent>
      <EndSessionsDialog target={target} onOpenChange={(v) => !v && setTarget(null)} />
    </Card>
  );
}
