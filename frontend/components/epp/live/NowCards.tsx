"use client";

import Link from "next/link";
import { toast } from "sonner";
import { format } from "date-fns";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import {
  useEPPLockouts,
  useEPPSessions,
  useEPPSuspensions,
  useLiftEPPLockouts,
  useLiftEPPSuspension,
  eppErrorMessage,
} from "@/lib/hooks/useEPP";

/** Matches EPP_SESSION_HEARTBEAT: the registry's counts are this fresh. */
export const LIVE_REFRESH_MS = 30_000;

function Count({ n, more }: { n: number; more?: boolean }) {
  return (
    <div className="text-4xl font-semibold tabular-nums">
      {n}
      {more ? "+" : ""}
    </div>
  );
}

function Problem({ error, what }: { error: unknown; what: string }) {
  return <div className="text-sm text-red-700 dark:text-red-400">Could not read {what}: {eppErrorMessage(error, "the request failed")}</div>;
}

export function OpenSessionsCard({ active }: { active: boolean }) {
  const q = useEPPSessions({ pagesize: 100 }, { refetchInterval: active ? LIVE_REFRESH_MS : undefined, enabled: active });
  const rows = q.data?.Data ?? [];
  const registrars = new Set(rows.map((s) => s.clid).filter(Boolean)).size;
  return (
    <Card>
      <EPPSectionHeader title="Open sessions" description="Live EPP sessions across every node, as of each node's last heartbeat (about 30 seconds)." />
      <CardContent className="space-y-2">
        {q.isLoading ? (
          <Skeleton className="h-12 w-24" />
        ) : q.error ? (
          <Problem error={q.error} what="sessions" />
        ) : (
          <>
            <Count n={rows.length} more={!!q.data?.Meta?.PageCursor} />
            <div className="text-sm text-muted-foreground">
              {rows.length === 0 ? "No registrar is connected." : `from ${registrars} registrar${registrars === 1 ? "" : "s"}`}
            </div>
            <Link href="/epp?tab=sessions" className="text-sm text-primary hover:underline">
              See who is connected
            </Link>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export function LockoutsCard({ active }: { active: boolean }) {
  const q = useEPPLockouts(undefined, { refetchInterval: active ? LIVE_REFRESH_MS : undefined, enabled: active });
  const lift = useLiftEPPLockouts();
  const rows = q.data ?? [];
  const onLift = async (clid: string, login: string) => {
    try {
      await lift.mutateAsync({ clid, login });
      toast.success(`${login} is no longer locked out.`);
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not lift the lockout"));
    }
  };
  return (
    <Card>
      <EPPSectionHeader
        title="Locked-out logins"
        description="A login is locked out from one address after 5 wrong passwords in 15 minutes, for 15 minutes, doubling on repeats. Lifting clears the count for that login from every address."
      />
      <CardContent className="space-y-3">
        {q.isLoading ? (
          <Skeleton className="h-12 w-24" />
        ) : q.error ? (
          <Problem error={q.error} what="lockouts" />
        ) : (
          <>
            <Count n={rows.length} />
            {rows.length === 0 ? (
              <div className="text-sm text-muted-foreground">Nobody is locked out.</div>
            ) : (
              <ul className="divide-y rounded-md border text-sm">
                {rows.map((k) => (
                  <li key={`${k.login}-${k.client_ip}`} className="flex items-center justify-between gap-3 px-3 py-2">
                    <div>
                      <Link href={`/registrars/${encodeURIComponent(k.clid)}?tab=epp`} className="font-mono text-primary hover:underline">
                        {k.clid}
                      </Link>
                      <span className="text-muted-foreground"> · login </span>
                      <span className="font-mono">{k.login}</span>
                      <span className="text-muted-foreground"> from </span>
                      <span className="font-mono">{k.client_ip}</span>
                      <span className="text-muted-foreground"> until {format(new Date(k.ends_at), "MMM d, HH:mm")}</span>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => onLift(k.clid, k.login)} disabled={lift.isPending}>
                      Lift
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

export function SuspensionsCard({ active }: { active: boolean }) {
  const q = useEPPSuspensions(undefined, { refetchInterval: active ? LIVE_REFRESH_MS : undefined, enabled: active });
  const lift = useLiftEPPSuspension();
  const rows = q.data ?? [];
  const onLift = async (clid: string, login: string) => {
    try {
      await lift.mutateAsync({ clid, login });
      toast.success(`${login} is no longer suspended. Its strikes are cleared.`);
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not lift the suspension"));
    }
  };
  return (
    <Card>
      <EPPSectionHeader
        title="Suspended logins"
        description="A login is suspended for 15 minutes when its registrar, or its family, keeps creating names that are already registered past its budget. Lifting also clears the strikes, so the next create starts the count again."
      />
      <CardContent className="space-y-3">
        {q.isLoading ? (
          <Skeleton className="h-12 w-24" />
        ) : q.error ? (
          <Problem error={q.error} what="suspensions" />
        ) : (
          <>
            <Count n={rows.length} />
            {rows.length === 0 ? (
              <div className="text-sm text-muted-foreground">Nobody is suspended.</div>
            ) : (
              <ul className="divide-y rounded-md border text-sm">
                {rows.map((s) => (
                  <li key={s.login} className="flex items-center justify-between gap-3 px-3 py-2">
                    <div>
                      <Link href={`/registrars/${encodeURIComponent(s.clid)}?tab=epp`} className="font-mono text-primary hover:underline">
                        {s.clid}
                      </Link>
                      <span className="text-muted-foreground"> · login </span>
                      <span className="font-mono">{s.login}</span>
                      <span className="text-muted-foreground">
                        {" "}
                        · counted for {s.scope === "family" ? `family ${s.family}` : "this registrar"} · until {format(new Date(s.ends_at), "MMM d, HH:mm")}
                      </span>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => onLift(s.clid, s.login)} disabled={lift.isPending}>
                      Lift
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
