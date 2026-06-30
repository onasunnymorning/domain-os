'use client';

import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { Loader2, FileText, ChevronRight, Shield, Database, RefreshCw, Wrench, BarChart3 } from 'lucide-react';
import Link from 'next/link';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { getWorkflowRegistry, type WorkflowMeta } from '@/lib/api/workflows';

const categoryIcons: Record<string, React.ComponentType<any>> = {
  data: Database,
  lifecycle: RefreshCw,
  operations: Wrench,
};

const categoryLabels: Record<string, string> = {
  data: 'Data Ingestion & Sync',
  lifecycle: 'Domain Lifecycle & Expiry',
  operations: 'Registry Utilities & Maintenance',
};

export default function DocsIndexPage() {
  const { data: registry, isLoading } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000,
  });

  const workflows = (registry?.items ?? []).filter((wf) => wf.docMarkdown);

  const groupedWorkflows = useMemo(() => {
    const groups: Record<string, WorkflowMeta[]> = {
      data: [],
      lifecycle: [],
      operations: [],
    };
    workflows.forEach((wf) => {
      const cat = wf.category || 'operations';
      if (!groups[cat]) groups[cat] = [];
      groups[cat].push(wf);
    });
    return groups;
  }, [workflows]);

  return (
    <DashboardLayout>
      <div className="space-y-8">
        {/* Header */}
        <div className="flex items-center gap-3 pb-4 border-b border-border/40">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
            <FileText className="h-5 w-5 text-primary" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Documentation Portal</h1>
            <p className="text-sm text-muted-foreground">
              Technical manuals, lifecycle descriptions, and policy reference sheets
            </p>
          </div>
        </div>

        {/* Loading State */}
        {isLoading && (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        )}

        {!isLoading && (
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
            {/* Left Sidebar: Core Guides & References */}
            <div className="lg:col-span-1 space-y-6">
              <div className="sticky top-24 space-y-4">
                <div>
                  <h2 className="text-xs font-bold uppercase tracking-wider text-muted-foreground/80 mb-3">
                    Reference Guides
                  </h2>
                  <div className="space-y-2">
                    <Link href="/docs/contact-data-policy" className="group block">
                      <Card className="border border-border/60 bg-card/40 transition-all duration-200 hover:border-orange-500/30 hover:bg-orange-500/[0.01]">
                        <CardHeader className="p-3.5 space-y-1">
                          <div className="flex items-center gap-2">
                            <Shield className="h-4 w-4 text-orange-500 shrink-0" />
                            <CardTitle className="text-sm font-semibold group-hover:text-primary transition-colors">
                              Contact Data Policy
                            </CardTitle>
                          </div>
                          <CardDescription className="text-xs leading-relaxed line-clamp-3">
                            Enforcement levels, compliance standards, and validation behaviors for contact registration details.
                          </CardDescription>
                        </CardHeader>
                      </Card>
                    </Link>
                    <Link href="/docs/posthog-analytics" className="group block">
                      <Card className="border border-border/60 bg-card/40 transition-all duration-200 hover:border-orange-500/30 hover:bg-orange-500/[0.01]">
                        <CardHeader className="p-3.5 space-y-1">
                          <div className="flex items-center gap-2">
                            <BarChart3 className="h-4 w-4 text-orange-500 shrink-0" />
                            <CardTitle className="text-sm font-semibold group-hover:text-primary transition-colors">
                              PostHog Analytics
                            </CardTitle>
                          </div>
                          <CardDescription className="text-xs leading-relaxed line-clamp-3">
                            Event tracking, session recordings, error capture, and behavioral analytics configuration.
                          </CardDescription>
                        </CardHeader>
                      </Card>
                    </Link>
                    <Link href="/docs/database-index-strategy" className="group block">
                      <Card className="border border-border/60 bg-card/40 transition-all duration-200 hover:border-orange-500/30 hover:bg-orange-500/[0.01]">
                        <CardHeader className="p-3.5 space-y-1">
                          <div className="flex items-center gap-2">
                            <Database className="h-4 w-4 text-orange-500 shrink-0" />
                            <CardTitle className="text-sm font-semibold group-hover:text-primary transition-colors">
                              Database Index Strategy
                            </CardTitle>
                          </div>
                          <CardDescription className="text-xs leading-relaxed line-clamp-3">
                            PostgreSQL indexing strategy for 80M+ domain scale, storage budgets, and query optimization.
                          </CardDescription>
                        </CardHeader>
                      </Card>
                    </Link>
                    <Link href="/docs/event-consumer" className="group block">
                      <Card className="border border-border/60 bg-card/40 transition-all duration-200 hover:border-amber-500/30 hover:bg-amber-500/[0.01]">
                        <CardHeader className="p-3.5 space-y-1">
                          <div className="flex items-center gap-2">
                            <Database className="h-4 w-4 text-amber-500 shrink-0" />
                            <CardTitle className="text-sm font-semibold group-hover:text-primary transition-colors">
                              Event Consumer Cloud
                            </CardTitle>
                          </div>
                          <CardDescription className="text-xs leading-relaxed line-clamp-3">
                            Tiered event lifecycle: hot PostgreSQL → warm S3 → cold archive, with automated relay and pruning.
                          </CardDescription>
                        </CardHeader>
                      </Card>
                    </Link>
                  </div>
                </div>
              </div>
            </div>

            {/* Right Main Area: Grouped Workflow Manuals */}
            <div className="lg:col-span-3 space-y-8">
              <div>
                <h2 className="text-xs font-bold uppercase tracking-wider text-muted-foreground/80 mb-4">
                  Workflow Operations
                </h2>
                {workflows.length > 0 ? (
                  <div className="space-y-6">
                    {Object.entries(groupedWorkflows).map(([cat, items]) => {
                      if (items.length === 0) return null;
                      const Icon = categoryIcons[cat] || FileText;
                      const label = categoryLabels[cat] || cat;

                      return (
                        <div key={cat} className="space-y-3">
                          <div className="flex items-center gap-2 text-foreground/80">
                            <Icon className="h-4 w-4 text-muted-foreground/80" />
                            <h3 className="text-xs font-bold uppercase tracking-wider">
                              {label}
                            </h3>
                          </div>
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                            {items.map((wf) => (
                              <Link key={wf.key} href={`/docs/${wf.key}`}>
                                <Card className="group h-full transition-all duration-200 hover:border-orange-500/30 hover:bg-orange-500/[0.01] hover:shadow-sm cursor-pointer bg-card/60 backdrop-blur-sm">
                                  <CardHeader className="p-4">
                                    <div className="flex items-start justify-between gap-2">
                                      <div className="space-y-1 min-w-0">
                                        <CardTitle className="text-sm font-semibold group-hover:text-primary transition-colors flex items-center gap-2">
                                          <FileText className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                                          {wf.name}
                                        </CardTitle>
                                        <CardDescription className="text-xs line-clamp-2 leading-relaxed">
                                          {wf.description}
                                        </CardDescription>
                                      </div>
                                      <ChevronRight className="h-4 w-4 text-muted-foreground/60 transition-transform group-hover:translate-x-0.5 mt-0.5 shrink-0" />
                                    </div>
                                  </CardHeader>
                                </Card>
                              </Link>
                            ))}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <Card className="rounded-lg border border-dashed p-12 text-center bg-card/30">
                    <FileText className="mx-auto h-8 w-8 text-muted-foreground/50" />
                    <p className="mt-2 text-sm text-muted-foreground">
                      No workflow manuals registered yet.
                    </p>
                  </Card>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </DashboardLayout>
  );
}
