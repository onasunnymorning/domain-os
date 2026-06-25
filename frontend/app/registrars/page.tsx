/**
 * Registrars Management Page
 * Provides tabs for managing IANA and System Registrars
 */

"use client";

import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { IANARegistrarsTab } from "@/components/registrars/iana-registrars-tab";
import { SystemRegistrarsTab } from "@/components/registrars/system-registrars-tab";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { useRegistrarCount, useIANARegistrarCount } from "@/lib/hooks/useRegistrars";
import { Plus, UserCheck } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { formatCompactNumber } from "@/lib/utils/numberUtils";

export default function RegistrarsPage() {
  const { data: systemCount } = useRegistrarCount();
  const { data: ianaCount } = useIANARegistrarCount();

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <UserCheck className="h-8 w-8" />
              Registrar Management
            </h1>
            <p className="text-muted-foreground mt-2">
              Manage IANA registrars and system registrars
            </p>
          </div>
          <div>
            <Button asChild>
              <Link href="/registrars/create">
                <Plus className="mr-2 h-4 w-4" />
                Create Registrar
              </Link>
            </Button>
          </div>
        </div>

        {/* Tabs */}
        <Tabs defaultValue="system" className="w-full">
          <TabsList className="grid w-full max-w-md grid-cols-2">
            <TooltipProvider delayDuration={100}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <TabsTrigger value="system">
                    System Registrars {systemCount?.Count !== undefined ? `(${formatCompactNumber(systemCount.Count)})` : ''}
                  </TabsTrigger>
                </TooltipTrigger>
                <TooltipContent className="max-w-[300px]" side="bottom" sideOffset={8}>
                  <p>System registrars are the registrar accounts to transact on the system.</p>
                  {systemCount?.Count !== undefined && <p className="mt-2 text-xs font-medium border-t pt-2 border-border/50 text-muted-foreground mr-1">{systemCount.Count.toLocaleString()}</p>}
                </TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <TabsTrigger value="iana">
                    IANA Registrars {ianaCount?.Count !== undefined ? `(${formatCompactNumber(ianaCount.Count)})` : ''}
                  </TabsTrigger>
                </TooltipTrigger>
                <TooltipContent className="max-w-[350px]" side="bottom" sideOffset={8}>
                  <p>Data taken from ICANN accredited registrars as they appear in the <a href="https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">IANA Registrar ID repository</a>. Their only purpose is to keep the accreditation status synced with the ICANN/IANA repo.</p>
                  {ianaCount?.Count !== undefined && <p className="mt-2 text-xs font-medium border-t pt-2 border-border/50 text-muted-foreground mr-1">{ianaCount.Count.toLocaleString()}</p>}
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </TabsList>

          <TabsContent value="system" className="mt-6">
            <SystemRegistrarsTab />
          </TabsContent>

          <TabsContent value="iana" className="mt-6">
            <IANARegistrarsTab />
          </TabsContent>
        </Tabs>
      </div>
    </DashboardLayout>
  );
}
