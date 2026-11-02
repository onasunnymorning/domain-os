"use client";

import { useState } from "react";
import Link from "next/link";
import { differenceInCalendarDays, format } from "date-fns";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { CredentialStateBadge } from "@/components/epp/CredentialStateBadge";
import { useEPPExpiringCertificatePins, eppErrorMessage } from "@/lib/hooks/useEPP";
import { cn } from "@/lib/utils";

const WINDOW = "720h"; // 30 days, the first expiry notice
const PAGE = 50;

/**
 * Every registrar's certificates expiring within 30 days, soonest first,
 * each row leading to the registrar's EPP tab where the new one is pinned.
 */
export function ExpiringCertificatesCard() {
  const [cursor, setCursor] = useState<string | undefined>();
  const [stack, setStack] = useState<string[]>([]);
  const q = useEPPExpiringCertificatePins(WINDOW, { pagesize: PAGE, cursor });
  const rows = q.data?.Data ?? [];
  const next = q.data?.Meta?.PageCursor;
  const now = new Date();

  return (
    <Card>
      <EPPSectionHeader
        title="Certificates expiring within 30 days"
        description="A client certificate past its end date is refused at login with no grace period. Pin the registrar's new certificate before then; the old pin can be retired afterwards."
      />
      <CardContent className="space-y-3">
        {q.isLoading ? (
          <Skeleton className="h-20 w-full" />
        ) : q.error ? (
          <div className="text-sm text-red-700 dark:text-red-400">{eppErrorMessage(q.error, "Could not read the certificates.")}</div>
        ) : rows.length === 0 ? (
          <div className="text-sm text-muted-foreground">None. Every pinned certificate is good for more than 30 days.</div>
        ) : (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Registrar</TableHead>
                  <TableHead>Certificate</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead>State</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((p) => {
                  const days = differenceInCalendarDays(new Date(p.not_after), now);
                  return (
                    <TableRow key={p.id}>
                      <TableCell>
                        <Link href={`/registrars/${encodeURIComponent(p.clid)}?tab=epp`} className="font-mono text-primary hover:underline">
                          {p.clid}
                        </Link>
                      </TableCell>
                      <TableCell>
                        <div>{p.subject || <span className="text-muted-foreground">(no subject)</span>}</div>
                        {p.logins && p.logins.length > 0 && (
                          <div className="text-xs text-muted-foreground">only for {p.logins.join(", ")}</div>
                        )}
                      </TableCell>
                      <TableCell>
                        <div>{format(new Date(p.not_after), "MMM d, yyyy")}</div>
                        <div className={cn("text-xs", days < 0 ? "text-red-600" : days <= 7 ? "text-red-600" : "text-amber-600")}>
                          {days < 0 ? `expired ${-days} day${days === -1 ? "" : "s"} ago` : days === 0 ? "expires today" : `in ${days} day${days === 1 ? "" : "s"}`}
                        </div>
                      </TableCell>
                      <TableCell>
                        <CredentialStateBadge credential={p} notBefore={p.not_before} notAfter={p.not_after} />
                      </TableCell>
                    </TableRow>
                  );
                })}
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
                setCursor(c);
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
    </Card>
  );
}
