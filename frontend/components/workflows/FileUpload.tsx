'use client';

import { useCallback, useRef, useState } from 'react';
import { Upload, X, FileText, CheckCircle2, AlertCircle, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { uploadEscrowFile, type UploadProgress } from '@/lib/api/escrow';

interface FileUploadProps {
  /** Workflow type key, e.g. "escrow-staging" */
  workflowType: string;
  /** TLD, e.g. "com" — must be set before upload */
  tld: string;
  /** Callback when upload completes successfully */
  onUploaded: (objectKey: string) => void;
  /** Callback when upload is cleared/removed */
  onClear?: () => void;
  /** Disable interaction */
  disabled?: boolean;
}

type UploadState =
  | { status: 'idle' }
  | { status: 'uploading'; progress: UploadProgress; file: File }
  | { status: 'completed'; key: string; file: File }
  | { status: 'error'; message: string; file?: File };

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(i > 0 ? 1 : 0)} ${sizes[i]}`;
}

export function FileUpload({ workflowType, tld, onUploaded, onClear, disabled }: FileUploadProps) {
  const [state, setState] = useState<UploadState>({ status: 'idle' });
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  const startUpload = useCallback(
    async (file: File) => {
      if (!tld) {
        setState({ status: 'error', message: 'Please select a TLD before uploading', file });
        return;
      }

      const abort = new AbortController();
      abortRef.current = abort;

      setState({
        status: 'uploading',
        progress: { loaded: 0, total: file.size, percentage: 0, activeParts: [], completedParts: [] },
        file,
      });

      try {
        const key = await uploadEscrowFile(file, workflowType, tld, {
          onProgress: (progress) => {
            setState({ status: 'uploading', progress, file });
          },
          abortSignal: abort.signal,
        });

        setState({ status: 'completed', key, file });
        onUploaded(key);
      } catch (err: any) {
        if (err?.name === 'AbortError') {
          setState({ status: 'idle' });
        } else {
          setState({
            status: 'error',
            message: err?.message || 'Upload failed',
            file,
          });
        }
      } finally {
        abortRef.current = null;
      }
    },
    [tld, workflowType, onUploaded]
  );

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) startUpload(file);
    // Reset input so re-selecting same file triggers change
    e.target.value = '';
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    const file = e.dataTransfer.files?.[0];
    if (file) startUpload(file);
  };

  const handleCancel = () => {
    abortRef.current?.abort();
    setState({ status: 'idle' });
  };

  const handleClear = () => {
    setState({ status: 'idle' });
    onClear?.();
  };

  // ── Completed ──
  if (state.status === 'completed') {
    return (
      <div className="flex items-center gap-3 rounded-lg border border-green-500/30 bg-green-500/5 px-4 py-3">
        <CheckCircle2 className="size-5 shrink-0 text-green-500" />
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium">{state.file.name}</div>
          <div className="text-muted-foreground text-xs">
            {formatBytes(state.file.size)} · Uploaded
          </div>
        </div>
        <Button variant="ghost" size="icon" className="size-7" onClick={handleClear}>
          <X className="size-3.5" />
        </Button>
      </div>
    );
  }

  // ── Error ──
  if (state.status === 'error') {
    // Split message: first line is the summary, rest is diagnostic detail
    const [summary, ...detailLines] = state.message.split('\n');
    const detail = detailLines.join('\n').trim();

    return (
      <div className="space-y-2">
        <div className="rounded-lg border border-red-500/30 bg-red-500/5 px-4 py-3">
          <div className="flex items-start gap-3">
            <AlertCircle className="mt-0.5 size-5 shrink-0 text-red-500" />
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-red-600 dark:text-red-400">{summary}</div>
            </div>
            <Button variant="ghost" size="icon" className="size-7 shrink-0" onClick={handleClear}>
              <X className="size-3.5" />
            </Button>
          </div>
          {detail && (
            <pre className="text-muted-foreground mt-2 whitespace-pre-wrap break-words border-t border-red-500/20 pt-2 font-mono text-[11px] leading-relaxed">
              {detail}
            </pre>
          )}
        </div>
        {state.file && (
          <Button variant="outline" size="sm" onClick={() => startUpload(state.file!)}>
            Retry
          </Button>
        )}
      </div>
    );
  }

  // ── Uploading ──
  if (state.status === 'uploading') {
    const { progress, file } = state;
    return (
      <div className="space-y-2 rounded-lg border px-4 py-3">
        <div className="flex items-center gap-3">
          <Loader2 className="size-5 shrink-0 animate-spin text-blue-500" />
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium">{file.name}</div>
            <div className="text-muted-foreground text-xs">
              {formatBytes(progress.loaded)} / {formatBytes(progress.total)}
              {progress.activeParts.length > 0 && (
                <span> · {progress.activeParts.length} part{progress.activeParts.length > 1 ? 's' : ''} active</span>
              )}
            </div>
          </div>
          <span className="shrink-0 text-sm font-medium tabular-nums text-blue-500">
            {progress.percentage}%
          </span>
          <Button variant="ghost" size="icon" className="size-7" onClick={handleCancel}>
            <X className="size-3.5" />
          </Button>
        </div>
        {/* Progress bar */}
        <div className="h-1.5 overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-blue-500 transition-all duration-300"
            style={{ width: `${progress.percentage}%` }}
          />
        </div>
      </div>
    );
  }

  // ── Idle — drop zone ──
  return (
    <div
      onDragOver={(e) => e.preventDefault()}
      onDrop={disabled ? undefined : handleDrop}
      onClick={disabled ? undefined : () => inputRef.current?.click()}
      className={cn(
        'flex cursor-pointer flex-col items-center gap-2 rounded-lg border-2 border-dashed px-6 py-8 text-center transition-colors',
        disabled
          ? 'cursor-not-allowed opacity-50'
          : 'hover:border-primary/50 hover:bg-muted/50'
      )}
    >
      <Upload className="text-muted-foreground size-8" />
      <div>
        <p className="text-sm font-medium">Drop escrow file here or click to browse</p>
        <p className="text-muted-foreground text-xs">
          Supports files up to 10GB · Uploads directly to storage
        </p>
      </div>
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        onChange={handleFileChange}
        disabled={disabled}
      />
    </div>
  );
}
