"use client";

import Link from "next/link";
import { Clock } from "lucide-react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { eppErrorMessage } from "@/lib/hooks/useEPP";
import { useLifecyclePolicies } from "@/lib/hooks/useLifecyclePolicies";
import { isStandardLifecyclePolicy } from "@/lib/constants/lifecycleDefaults";

export default function LifecyclePoliciesPage() {
  const q = useLifecyclePolicies();
  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="space-y-2">
          <div className="flex items-baseline gap-3">
            <Clock className="h-10 w-10 text-muted-foreground" />
            <h1 className="text-5xl font-bold tracking-tight">Lifecycle Policies</h1>
          </div>
          <p className="ml-[52px] text-sm text-muted-foreground">
            How long each period of a domain&apos;s life lasts: grace periods, the transfer lock, redemption, pending delete, the
            restore report and the renewal horizon. A policy is platform-wide; each TLD phase names one, and a change reaches every
            phase that names it at once.
          </p>
        </div>

        <Card>
          <EPPSectionHeader
            title="Policies"
            description="gtld-standard is the built-in default every new phase follows. Changing a policy needs the registry:policy:admin permission; which policy a phase follows is set on the phase."
            action={
              <Button size="sm" asChild>
                <Link href="/lifecycle-policies/new-policy">New policy</Link>
              </Button>
            }
          />
          <CardContent>
            {q.isLoading ? (
              <Skeleton className="h-24 w-full" />
            ) : q.error ? (
              <div className="text-sm text-red-700 dark:text-red-400">{eppErrorMessage(q.error, "Could not read the lifecycle policies.")}</div>
            ) : (
              <div className="rounded-md border overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Description</TableHead>
                      <TableHead className="text-right">Add grace</TableHead>
                      <TableHead className="text-right">Transfer lock</TableHead>
                      <TableHead className="text-right">Redemption</TableHead>
                      <TableHead className="text-right">Pending delete</TableHead>
                      <TableHead className="text-right">Horizon</TableHead>
                      <TableHead className="text-right">Used by</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(q.data ?? []).map((p) => (
                      <TableRow key={p.name}>
                        <TableCell>
                          <Link href={`/lifecycle-policies/${encodeURIComponent(p.name)}`} className="font-mono font-medium text-primary hover:underline">
                            {p.name}
                          </Link>
                          {isStandardLifecyclePolicy(p.name) && (
                            <Badge variant="secondary" className="ml-2">
                              Built-in
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="max-w-[320px] truncate text-muted-foreground" title={p.description}>
                          {p.description || "–"}
                        </TableCell>
                        <TableCell className="text-right">{p.resolved.registrationGP}d</TableCell>
                        <TableCell className="text-right">{p.resolved.transferLockPeriod === 0 ? <span className="text-muted-foreground">none</span> : `${p.resolved.transferLockPeriod}d`}</TableCell>
                        <TableCell className="text-right">{p.resolved.redemptionGP}d</TableCell>
                        <TableCell className="text-right">{p.resolved.pendingdeleteGP}d</TableCell>
                        <TableCell className="text-right">{p.resolved.maxHorizon}y</TableCell>
                        <TableCell className="text-right">
                          {(p.usedBy?.length ?? 0) === 0 ? <span className="text-muted-foreground">no phase</span> : `${p.usedBy!.length} phase${p.usedBy!.length === 1 ? "" : "s"}`}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
