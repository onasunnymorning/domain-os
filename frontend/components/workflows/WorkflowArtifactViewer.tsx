'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { Download, Loader2, CheckCircle2, XCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { signalWorkflow } from '@/lib/api/workflows';
import { cn } from '@/lib/utils';

interface WorkflowArtifactViewerProps {
  workflowId: string;
  artifacts?: Record<string, string>; // label → URL
  signalName?: string;
  onSignalSent?: () => void;
}

export function WorkflowArtifactViewer({
  workflowId,
  artifacts,
  signalName,
  onSignalSent,
}: WorkflowArtifactViewerProps) {
  const [signalSending, setSignalSending] = useState<'approve' | 'reject' | null>(null);
  const [signalSent, setSignalSent] = useState(false);

  const handleSignal = async (approved: boolean) => {
    if (!signalName) return;

    const action = approved ? 'approve' : 'reject';
    setSignalSending(action);

    try {
      await signalWorkflow(workflowId, signalName, approved);
      setSignalSent(true);
      toast.success(
        approved
          ? 'Workflow approved'
          : 'Workflow rejected',
        { description: `Signal sent to ${workflowId}` }
      );
      onSignalSent?.();
    } catch (error: any) {
      const message =
        error?.response?.data?.message || error?.message || 'Failed to send signal';
      toast.error('Signal failed', { description: message });
    } finally {
      setSignalSending(null);
    }
  };

  return (
    <div className="space-y-3">
      {/* Artifact download links */}
      {artifacts && Object.keys(artifacts).length > 0 && (
        <div className="space-y-1.5">
          <p className="text-muted-foreground text-xs font-medium">Artifacts</p>
          <div className="flex flex-col gap-1">
            {Object.entries(artifacts).map(([label, url]) => (
              <a
                key={label}
                href={url}
                target="_blank"
                rel="noopener noreferrer"
                className={cn(
                  'text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-xs transition-colors'
                )}
              >
                <Download className="size-3" />
                <span>{label}</span>
              </a>
            ))}
          </div>
        </div>
      )}

      {/* HITL Signal Buttons */}
      {signalName && (
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="default"
            disabled={signalSent || signalSending !== null}
            onClick={() => handleSignal(true)}
            className="gap-1.5"
          >
            {signalSending === 'approve' ? (
              <Loader2 className="size-3 animate-spin" />
            ) : (
              <CheckCircle2 className="size-3" />
            )}
            Approve
          </Button>
          <Button
            size="sm"
            variant="destructive"
            disabled={signalSent || signalSending !== null}
            onClick={() => handleSignal(false)}
            className="gap-1.5"
          >
            {signalSending === 'reject' ? (
              <Loader2 className="size-3 animate-spin" />
            ) : (
              <XCircle className="size-3" />
            )}
            Reject
          </Button>
          {signalSent && (
            <span className="text-muted-foreground text-xs">Signal sent</span>
          )}
        </div>
      )}
    </div>
  );
}
