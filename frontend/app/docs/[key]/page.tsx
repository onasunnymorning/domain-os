'use client';

import { use } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Loader2, FileText, ArrowLeft, Zap } from 'lucide-react';
import Link from 'next/link';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { WorkflowDocViewer } from '@/components/workflows/WorkflowDocViewer';
import { getWorkflowRegistry, type WorkflowMeta } from '@/lib/api/workflows';

export default function WorkflowDocPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);

  const { data: registry, isLoading } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000,
  });

  const workflow = (registry?.items ?? []).find((wf) => wf.key === key);

  return (
    <DashboardLayout>
      {/* Loading */}
      {isLoading && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      )}

      {/* Not Found */}
      {!isLoading && !workflow && (
        <div className="space-y-4">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/docs" className="gap-2">
              <ArrowLeft className="h-4 w-4" />
              Back to Documentation
            </Link>
          </Button>
          <div className="rounded-lg border border-dashed p-12 text-center">
            <FileText className="mx-auto h-8 w-8 text-muted-foreground/50" />
            <p className="mt-2 text-sm text-muted-foreground">
              Workflow &ldquo;{key}&rdquo; not found.
            </p>
          </div>
        </div>
      )}

      {/* Doc Content */}
      {!isLoading && workflow && (
        <div className="space-y-6">
          {/* Breadcrumb + Header */}
          <div className="space-y-4">
            <Button variant="ghost" size="sm" asChild>
              <Link href="/docs" className="gap-2">
                <ArrowLeft className="h-4 w-4" />
                Documentation
              </Link>
            </Button>

            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
                  <FileText className="h-5 w-5 text-primary" />
                </div>
                <div>
                  <h1 className="text-2xl font-bold tracking-tight">{workflow.name}</h1>
                  <p className="text-sm text-muted-foreground">
                    {workflow.description}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline">{workflow.category}</Badge>
                <Button variant="outline" size="sm" asChild>
                  <Link href={`/workflows?highlight=${encodeURIComponent(workflow.key)}`} className="gap-1.5">
                    <Zap className="h-3 w-3" />
                    Go to Launchpad
                  </Link>
                </Button>
              </div>
            </div>
          </div>

          {/* Full page doc viewer */}
          <div className="rounded-lg border bg-card p-6 md:p-8">
            <WorkflowDocViewer markdown={workflow.docMarkdown!} />
          </div>
        </div>
      )}
    </DashboardLayout>
  );
}
