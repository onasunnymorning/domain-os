'use client';

import { ArrowLeft, Variable } from 'lucide-react';
import Link from 'next/link';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { WorkflowDocViewer } from '@/components/workflows/WorkflowDocViewer';
import { RUNTIME_ENV_DOC_MARKDOWN } from '@/lib/constants/runtimeEnvDoc';

export default function RuntimeEnvDocPage() {
  return (
    <DashboardLayout>
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
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-500/10">
                <Variable className="h-5 w-5 text-sky-500" />
              </div>
              <div>
                <h1 className="text-2xl font-bold tracking-tight">Runtime Environment Variables</h1>
                <p className="text-sm text-muted-foreground">
                  How NEXT_PUBLIC_* values reach the browser, and why process.env is banned
                </p>
              </div>
            </div>
            <Badge variant="outline" className="border-sky-500/30 text-sky-500 bg-sky-500/5">
              Reference Guide
            </Badge>
          </div>
        </div>

        {/* Full page doc viewer */}
        <div className="rounded-lg border bg-card p-6 md:p-8">
          <WorkflowDocViewer markdown={RUNTIME_ENV_DOC_MARKDOWN} />
        </div>
      </div>
    </DashboardLayout>
  );
}
