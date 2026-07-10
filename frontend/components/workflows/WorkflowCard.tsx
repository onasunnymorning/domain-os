'use client';

import Link from 'next/link';
import { Database, RefreshCw, Settings, Clock, Play, Zap, FileText, ExternalLink } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { WorkflowMeta } from '@/lib/api/workflows';
import { getTemporalUiUrl, getTemporalNamespace } from '@/lib/env';

interface WorkflowCardProps {
  workflow: WorkflowMeta;
  onLaunch: (workflow: WorkflowMeta) => void;
}

const categoryIcons: Record<string, React.ElementType> = {
  data: Database,
  lifecycle: RefreshCw,
};

export function WorkflowCard({ workflow, onLaunch }: WorkflowCardProps) {
  const Icon = categoryIcons[workflow.category] || Zap;
  const scheduleUrl = `${getTemporalUiUrl()}/namespaces/${getTemporalNamespace()}/schedules/${workflow.scheduleId}`;

  return (
    <Card
      className={cn(
        'group relative transition-all duration-200',
        'hover:shadow-md hover:scale-[1.01]'
      )}
    >
      <CardHeader>
        <div className="flex items-start gap-3">
          <div className="bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg">
            <Icon className="text-muted-foreground size-4" />
          </div>
          <div className="min-w-0 flex-1">
            <CardTitle className="text-sm">{workflow.name}</CardTitle>
            <CardDescription className="mt-1 line-clamp-2 text-xs">
              {workflow.description}
            </CardDescription>
          </div>
        </div>
      </CardHeader>

      <CardContent className="pt-0">
        <div className="flex flex-wrap gap-1.5">
          {workflow.tags.map((tag) => (
            <Badge key={tag} variant="outline" className="text-[10px] px-1.5 py-0">
              {tag}
            </Badge>
          ))}
        </div>

        {workflow.scheduled && workflow.scheduleInfo && (
          <div className="text-muted-foreground mt-3 flex items-center gap-1.5 text-xs">
            <Clock className="size-3" />
            <span>{workflow.scheduleInfo}</span>
            {workflow.scheduleId && (
              <>
                <span className="text-muted-foreground/50">·</span>
                <a
                  href={scheduleUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-0.5 text-muted-foreground hover:text-foreground transition-colors"
                  title={`View schedule "${workflow.scheduleId}" in Temporal`}
                >
                  <span className="font-mono">{workflow.scheduleId}</span>
                  <ExternalLink className="size-2.5" />
                </a>
              </>
            )}
          </div>
        )}
      </CardContent>

      <CardFooter className="flex items-center justify-between pt-0">
        <Badge variant="secondary" className="text-[10px]">
          {workflow.queue}
        </Badge>
        <div className="flex items-center gap-1.5">
          {workflow.docMarkdown && (
            <Button
              size="sm"
              variant="ghost"
              asChild
              className="gap-1.5 text-muted-foreground hover:text-foreground"
            >
              <Link href={`/docs/${workflow.key}`}>
                <FileText className="size-3" />
                Docs
              </Link>
            </Button>
          )}
          <Button
            size="sm"
            variant={workflow.scheduled ? 'outline' : 'default'}
            onClick={() => onLaunch(workflow)}
            className="gap-1.5"
          >
            <Play className="size-3" />
            {workflow.scheduled ? 'Trigger' : 'Launch'}
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
}
