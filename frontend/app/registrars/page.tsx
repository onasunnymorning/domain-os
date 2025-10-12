/**
 * Registrars Management Page
 * Provides tabs for managing IANA and System Registrars
 */

"use client";

import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { IANARegistrarsTab } from "@/components/registrars/iana-registrars-tab";
import { SystemRegistrarsTab } from "@/components/registrars/system-registrars-tab";
import { UserCheck } from "lucide-react";

export default function RegistrarsPage() {
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
        </div>

        {/* Tabs */}
        <Tabs defaultValue="system" className="w-full">
          <TabsList className="grid w-full max-w-md grid-cols-2">
            <TabsTrigger value="system">System Registrars</TabsTrigger>
            <TabsTrigger value="iana">IANA Registrars</TabsTrigger>
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
