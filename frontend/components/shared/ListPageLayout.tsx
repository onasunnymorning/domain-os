import React from 'react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardContent } from '@/components/ui/card';

export interface ListPageLayoutProps {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  /** @deprecated Prefer omitting — titles should be self-explanatory */
  description?: string;
  actionButton?: React.ReactNode;
  /** Rendered inline to the right of the title (e.g. WorkflowShortcuts) */
  headerActions?: React.ReactNode;
  filters?: React.ReactNode;
  children: React.ReactNode;
}

export function ListPageLayout({
  icon: Icon,
  title,
  description,
  actionButton,
  headerActions,
  filters,
  children
}: ListPageLayoutProps) {
  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <Icon className="h-8 w-8" />
              {title}
            </h1>
            {headerActions}
          </div>
          <div className="flex items-center gap-3">
            {actionButton}
          </div>
        </div>

        {/* Filters */}
        {filters && (
          <Card>
            <CardContent className="pt-6">
              {filters}
            </CardContent>
          </Card>
        )}

        {/* Content (usually a DataTable) */}
        {children}
      </div>
    </DashboardLayout>
  );
}
