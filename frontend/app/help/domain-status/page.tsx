"use client";

import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { STATUS_LABELS, STATUS_DESCRIPTIONS, RGP_LABELS, RGP_DESCRIPTIONS } from "@/lib/constants/domainStatus";

export default function DomainStatusHelpPage() {
  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Domain status & grace periods</h1>
          <p className="text-muted-foreground mt-2">Quick reference for EPP status flags and RGP windows.</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Status flags</CardTitle>
            <CardDescription>What do these mean?</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-sm grid grid-cols-[max-content,1fr] gap-x-3 gap-y-3">
              {Object.entries(STATUS_LABELS).map(([key, label]) => (
                <>
                  <Badge key={`${key}-badge`} variant="outline" className="px-2 py-0.5 h-7 self-start">{label}</Badge>
                  <span key={`${key}-desc`} className="text-foreground/80 leading-snug">{STATUS_DESCRIPTIONS[key] || label}</span>
                </>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Grace periods (RGP)</CardTitle>
            <CardDescription>Shown only when the period end date is in the future</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-sm grid grid-cols-[max-content,1fr] gap-x-3 gap-y-3">
              {Object.values(RGP_LABELS).map((lbl) => (
                <>
                  <Badge key={`${lbl}-badge`} variant="secondary" className="px-2 py-0.5 h-7 self-start">{lbl}</Badge>
                  <span key={`${lbl}-desc`} className="text-foreground/80 leading-snug">{RGP_DESCRIPTIONS[lbl] || lbl}</span>
                </>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
