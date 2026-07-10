'use client';

import { useState } from 'react';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { ROCard } from '@/components/registry-operators/ROCard';
import { ROCreateDialog } from '@/components/registry-operators/ROCreateDialog';
import { Button } from '@/components/ui/button';
import { Building2, PlusIcon } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { RegistryOrbitWidget } from '@/components/dashboard/RegistryOrbitWidget';

export default function RegistryOperatorsPage() {
  const { data, isLoading, error } = useRegistryOperators({ pagesize: 50 });
  const operators = data?.Data ?? [];
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="space-y-8">
        {/* Header */}
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
            <Building2 className="h-8 w-8" />
            Registry Operators
          </h1>
          <Button onClick={() => setCreateOpen(true)}>
            <PlusIcon className="mr-2 h-4 w-4" />
            Create Operator
          </Button>
        </div>

        {/* Error */}
        {error && (
          <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            Failed to load registry operators: {error.message}
          </div>
        )}

        {/* Loading skeletons */}
        {isLoading && (
          <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="rounded-xl border bg-card p-5 space-y-4">
                <Skeleton className="h-6 w-40" />
                <Skeleton className="h-4 w-28" />
                <div className="space-y-2">
                  <Skeleton className="h-12 w-full rounded-lg" />
                  <Skeleton className="h-12 w-3/4 rounded-lg" />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Empty state */}
        {!isLoading && operators.length === 0 && !error && (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div className="rounded-full bg-muted p-4 mb-4">
              <Building2 className="h-8 w-8 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold mb-1">No registry operators</h3>
            <p className="text-sm text-muted-foreground mb-4">
              Create your first registry operator to get started.
            </p>
            <Button onClick={() => setCreateOpen(true)}>
              <PlusIcon className="mr-2 h-4 w-4" />
              Create Operator
            </Button>
          </div>
        )}

        {/* Card grid */}
        {!isLoading && operators.length > 0 && (
          <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
            {operators.map((op) => (
              <ROCard key={op.RyID} operator={op} />
            ))}
          </div>
        )}

        {/* Registry Landscape — orbit visualization */}
        {!isLoading && operators.length > 0 && (
          <RegistryOrbitWidget />
        )}
      </div>

      {/* Create dialog */}
      <ROCreateDialog open={createOpen} onOpenChange={setCreateOpen} />
    </DashboardLayout>
  );
}
