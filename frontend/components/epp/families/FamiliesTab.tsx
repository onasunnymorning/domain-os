"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { useRegistrarFamilies, eppErrorMessage } from "@/lib/hooks/useEPP";

export function FamiliesTab() {
  const q = useRegistrarFamilies();
  return (
    <Card>
      <EPPSectionHeader
        title="Families"
        description="A family groups affiliated registrars, often one company's accreditations, so their sessions and commands are also counted together. A registrar is in at most one family. The family's policy sets the shared limits; each member keeps its own policy for its own."
        action={
          <Button size="sm" asChild>
            <Link href="/epp/new-family">New family</Link>
          </Button>
        }
      />
      <CardContent>
        {q.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : q.error ? (
          <div className="text-sm text-red-700 dark:text-red-400">{eppErrorMessage(q.error, "Could not read the families.")}</div>
        ) : (q.data ?? []).length === 0 ? (
          <div className="text-sm text-muted-foreground">No families yet. Registrars are counted on their own until one is made.</div>
        ) : (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Policy</TableHead>
                  <TableHead>Members</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(q.data ?? []).map((f) => (
                  <TableRow key={f.name}>
                    <TableCell>
                      <Link href={`/epp/families/${encodeURIComponent(f.name)}`} className="font-mono font-medium text-primary hover:underline">
                        {f.name}
                      </Link>
                    </TableCell>
                    <TableCell className="max-w-[320px] truncate text-muted-foreground" title={f.description}>
                      {f.description || "–"}
                    </TableCell>
                    <TableCell className="font-mono">{f.policy || "default"}</TableCell>
                    <TableCell>
                      <span title={f.members.join(", ")}>
                        {f.members.length} {f.members.length === 1 ? "registrar" : "registrars"}
                      </span>
                      {f.members.length > 0 && (
                        <span className="ml-2 font-mono text-xs text-muted-foreground">
                          {f.members.slice(0, 3).join(", ")}
                          {f.members.length > 3 ? ", …" : ""}
                        </span>
                      )}
                    </TableCell>
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
