"use client";

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { isBuiltInPolicy } from "@/lib/constants/eppPolicyDefaults";
import { useEPPPolicies, eppErrorMessage } from "@/lib/hooks/useEPP";

export function PoliciesTab() {
  const q = useEPPPolicies();
  return (
    <Card>
      <EPPSectionHeader
        title="Policies"
        description="A policy is a named set of limits: sessions, names per check, command rates, creates per day and the failed-create budget. Every registrar is on exactly one; default applies to any registrar not assigned another, probation is for new registrars."
        action={
          <Button size="sm" asChild>
            <Link href="/epp/new-policy">New policy</Link>
          </Button>
        }
      />
      <CardContent>
        {q.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : q.error ? (
          <div className="text-sm text-red-700 dark:text-red-400">{eppErrorMessage(q.error, "Could not read the policies.")}</div>
        ) : (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead className="text-right">Sessions</TableHead>
                  <TableHead className="text-right">Names per check</TableHead>
                  <TableHead className="text-right">Creates per day</TableHead>
                  <TableHead className="text-right">Changes per second</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(q.data ?? []).map((p) => (
                  <TableRow key={p.name}>
                    <TableCell>
                      <Link href={`/epp/policies/${encodeURIComponent(p.name)}`} className="font-mono font-medium text-primary hover:underline">
                        {p.name}
                      </Link>
                      {isBuiltInPolicy(p.name) && (
                        <Badge variant="secondary" className="ml-2">
                          Built-in
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="max-w-[320px] truncate text-muted-foreground" title={p.description}>
                      {p.description || "–"}
                    </TableCell>
                    <TableCell className="text-right">{p.limits.sessions_per_registrar}</TableCell>
                    <TableCell className="text-right">{p.limits.check_names}</TableCell>
                    <TableCell className="text-right">{p.limits.creates_per_day === 0 ? <span className="text-muted-foreground">unlimited</span> : p.limits.creates_per_day}</TableCell>
                    <TableCell className="text-right">{p.limits.rates.transform.per_second}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
