"use client";

import { useParams, useRouter } from "next/navigation";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { useRegistrar } from "@/lib/hooks/useRegistrars";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export default function RegistrarDetailPage() {
  const params = useParams();
  const router = useRouter();
  const clid = typeof params?.clid === "string" ? params.clid : Array.isArray(params?.clid) ? params?.clid[0] : "";
  const { data, isLoading, error } = useRegistrar(clid, !!clid);

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold">Registrar Detail</h1>
            <p className="text-sm text-muted-foreground mt-1">Read-only view of registrar {clid}</p>
          </div>
          <Button variant="outline" onClick={() => router.push("/registrars")}>Back to list</Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>{isLoading ? <Skeleton className="h-6 w-48" /> : data?.Name ?? clid}</CardTitle>
            {!isLoading && (
              <CardDescription className="flex items-center gap-2">
                <span className="font-mono">{data?.ClID}</span>
                {data?.Status && <Badge>{data.Status}</Badge>}
                {data?.IANAStatus && (
                  <Badge variant="secondary">{`IANA ${data.IANAStatus}`}</Badge>
                )}
              </CardDescription>
            )}
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            {isLoading ? (
              <>
                {Array.from({ length: 8 }).map((_, i) => (
                  <div key={i} className="space-y-1">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-5 w-64" />
                  </div>
                ))}
              </>
            ) : error ? (
              <div className="text-red-600">Failed to load registrar: {(error as any)?.message ?? "Unknown error"}</div>
            ) : (
              <>
                <Field label="Name" value={data?.Name} />
                <Field label="Client ID" value={data?.ClID} mono />
                <Field label="IANA ID" value={data?.GurID?.toString()} mono />
                <Field label="IANA Status" value={data?.IANAStatus} />
                <Field label="Status" value={data?.Status} />
                <Field label="Email" value={data?.Email} />
                <Field label="URL" value={data?.URL} />
                <Field label="RDAP Base URL" value={data?.RdapBaseURL} />
                <Field label="Auto-renew" value={data?.Autorenew ? "Enabled" : "Disabled"} />
                <Field label="Created" value={data?.CreatedAt} />
                <Field label="Updated" value={data?.UpdatedAt} />
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}

function Field({ label, value, mono = false }: { label: string; value?: string | number | boolean; mono?: boolean }) {
  return (
    <div className="space-y-1">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={mono ? "font-mono" : ""}>{value ?? "-"}</div>
    </div>
  );
}
