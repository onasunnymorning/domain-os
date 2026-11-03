"use client";

import { Suspense } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Plug } from "lucide-react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ExpiringCertificatesCard } from "@/components/epp/ExpiringCertificatesCard";
import { PoliciesTab } from "@/components/epp/policies/PoliciesTab";
import { FamiliesTab } from "@/components/epp/families/FamiliesTab";
import { LockoutsCard, OpenSessionsCard, SuspensionsCard } from "@/components/epp/live/NowCards";
import { SessionsTab } from "@/components/epp/live/SessionsTab";

const TABS = ["now", "sessions", "policies", "families"] as const;
type Tab = (typeof TABS)[number];

function EPPPageInner() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  // The URL is the state: sidebar and history navigation change the tab too.
  const param = searchParams?.get("tab");
  const tab: Tab = TABS.includes(param as Tab) ? (param as Tab) : "now";

  const onTabChange = (value: string) => {
    const next = new URLSearchParams(searchParams?.toString() ?? "");
    next.set("tab", value);
    router.replace(`${pathname}?${next.toString()}`, { scroll: false });
  };

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <div className="flex items-baseline gap-3">
          <Plug className="h-10 w-10 text-muted-foreground" />
          <h1 className="text-5xl font-bold tracking-tight">EPP</h1>
        </div>
        <p className="ml-[52px] text-sm text-muted-foreground">
          What registrars may do over EPP, and what is happening right now. Each registrar&apos;s own logins,
          certificates and networks are on its page, under the EPP tab.
        </p>
      </div>

      <Tabs value={tab} onValueChange={onTabChange} className="space-y-6">
        <TabsList>
          <TabsTrigger value="now">Now</TabsTrigger>
          <TabsTrigger value="sessions">Sessions</TabsTrigger>
          <TabsTrigger value="policies">Policies</TabsTrigger>
          <TabsTrigger value="families">Families</TabsTrigger>
        </TabsList>

        <TabsContent value="now" className="space-y-6">
          <div className="grid gap-6 lg:grid-cols-3">
            <OpenSessionsCard active={tab === "now"} />
            <LockoutsCard active={tab === "now"} />
            <SuspensionsCard active={tab === "now"} />
          </div>
          <ExpiringCertificatesCard />
        </TabsContent>

        <TabsContent value="sessions">
          <SessionsTab active={tab === "sessions"} />
        </TabsContent>

        <TabsContent value="policies">
          <PoliciesTab />
        </TabsContent>

        <TabsContent value="families">
          <FamiliesTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default function EPPPage() {
  return (
    <DashboardLayout>
      <Suspense fallback={null}>
        <EPPPageInner />
      </Suspense>
    </DashboardLayout>
  );
}
