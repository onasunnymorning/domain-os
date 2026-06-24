'use client';

import { useQuery } from '@tanstack/react-query';
import { Loader2, FileText, ChevronRight } from 'lucide-react';
import Link from 'next/link';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Badge } from '@/components/ui/badge';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card';
import { getWorkflowRegistry, type WorkflowMeta } from '@/lib/api/workflows';

export default function DocsIndexPage() {
  const { data: registry, isLoading } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000,
  });

  const workflows = (registry?.items ?? []).filter((wf) => wf.docMarkdown);

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
              <FileText className="h-5 w-5 text-primary" />
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-tight">Documentation</h1>
              <p className="text-sm text-muted-foreground">
                Workflow architecture, step breakdowns, and operational guides
              </p>
            </div>
          </div>
        </div>

        {/* Loading State */}
        {isLoading && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        )}

        {/* Workflow docs list */}
        {!isLoading && workflows.length > 0 && (
          <div className="grid gap-3">
            {workflows.map((wf) => (
              <Link key={wf.key} href={`/docs/${wf.key}`}>
                <Card className="group transition-all duration-200 hover:shadow-md hover:scale-[1.005] cursor-pointer">
                  <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <FileText className="h-4 w-4 text-muted-foreground" />
                        <CardTitle className="text-sm">{wf.name}</CardTitle>
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                          {wf.category}
                        </Badge>
                      </div>
                      <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                    </div>
                    <CardDescription className="mt-1 text-xs">
                      {wf.description}
                    </CardDescription>
                  </CardHeader>
                </Card>
              </Link>
            ))}
          </div>
        )}

        {/* Empty state */}
        {!isLoading && workflows.length === 0 && (
          <div className="rounded-lg border border-dashed p-12 text-center">
            <FileText className="mx-auto h-8 w-8 text-muted-foreground/50" />
            <p className="mt-2 text-sm text-muted-foreground">
              No workflow documentation available yet.
            </p>
          </div>
        )}
      </div>
    </DashboardLayout>
  );
}
