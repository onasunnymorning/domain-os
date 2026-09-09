'use client';

import { useState } from 'react';
import { Download, FileJson, FileText, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { getStorageDownloadURL } from '@/lib/api/workflows';
import { cn } from '@/lib/utils';

export interface EscrowArtifact {
  label: string;
  /** What this document is for, shown under the label. */
  hint: string;
  objectKey?: string;
  kind: 'json' | 'xml';
}

/**
 * Download links for the documents a run emitted.
 *
 * An absent key is rendered, not hidden. "No notification" is a fact about the
 * run — an unsigned deposit never produces one — and a reader who cannot see
 * that the slot exists is left wondering whether the document failed to write.
 */
export function EscrowArtifactLinks({
  artifacts,
  className,
}: {
  artifacts: EscrowArtifact[];
  className?: string;
}) {
  return (
    <div className={cn('grid gap-2 sm:grid-cols-2 lg:grid-cols-3', className)}>
      {artifacts.map((a) => (
        <ArtifactCard key={a.label} artifact={a} />
      ))}
    </div>
  );
}

function ArtifactCard({ artifact }: { artifact: EscrowArtifact }) {
  const [downloading, setDownloading] = useState(false);
  const Icon = artifact.kind === 'json' ? FileJson : FileText;

  const download = async () => {
    if (!artifact.objectKey) return;
    setDownloading(true);
    try {
      const { url } = await getStorageDownloadURL(artifact.objectKey);
      window.open(url, '_blank');
    } catch {
      toast.error(`Could not get a download link for the ${artifact.label.toLowerCase()}`);
    } finally {
      setDownloading(false);
    }
  };

  if (!artifact.objectKey) {
    return (
      <div className="rounded-lg border border-dashed border-border p-3 opacity-60">
        <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <Icon className="h-4 w-4" />
          {artifact.label}
        </div>
        <p className="mt-1 text-xs text-muted-foreground">Not emitted for this run.</p>
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={download}
      disabled={downloading}
      className="group rounded-lg border border-border p-3 text-left transition-colors hover:border-primary/50 hover:bg-muted/30 disabled:opacity-60"
    >
      <div className="flex items-center gap-2 text-sm font-medium">
        <Icon className="h-4 w-4 text-muted-foreground" />
        {artifact.label}
        {downloading ? (
          <Loader2 className="ml-auto h-3.5 w-3.5 animate-spin" />
        ) : (
          <Download className="ml-auto h-3.5 w-3.5 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
        )}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">{artifact.hint}</p>
    </button>
  );
}
