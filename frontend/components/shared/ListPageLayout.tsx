import React from 'react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

export interface ListPageLayoutProps {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
  actionButton?: React.ReactNode;
  filters?: React.ReactNode;
  children: React.ReactNode;
}

export function ListPageLayout({
  icon: Icon,
  title,
  description,
  actionButton,
  filters,
  children
}: ListPageLayoutProps) {
  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <Icon className="h-8 w-8" />
              {title}
            </h1>
            <p className="text-muted-foreground mt-2">{description}</p>
          </div>
          {actionButton && <div>{actionButton}</div>}
        </div>

        {/* Filters */}
        {filters && (
          <Card>
            <CardHeader>
              <CardTitle>Filters</CardTitle>
              <CardDescription>Search and filter {title.toLowerCase()}</CardDescription>
            </CardHeader>
            <CardContent>
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
