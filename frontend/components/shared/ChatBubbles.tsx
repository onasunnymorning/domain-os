'use client';

import { cn } from '@/lib/utils';
import { Sparkles } from 'lucide-react';

// ---------------------------------------------------------------------------
// Shared conversational UI primitives
// Used by: ROCreateDialog, TLDCreateDialog, PhaseCreateConversation
// ---------------------------------------------------------------------------

/**
 * Left-aligned system bubble with optional icon.
 * Slides up with a fade-in animation.
 */
export function SystemBubble({ children, icon: Icon, className }: {
  children: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
  className?: string;
}) {
  return (
    <div className={cn(
      'flex gap-3 items-start animate-in fade-in slide-in-from-bottom-2 duration-300',
      className,
    )}>
      <div className="shrink-0 mt-0.5 flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-primary">
        {Icon ? <Icon className="h-3.5 w-3.5" /> : <Sparkles className="h-3.5 w-3.5" />}
      </div>
      <div className="min-w-0 flex-1 text-sm text-foreground/80">
        {children}
      </div>
    </div>
  );
}

/**
 * Right-aligned user response bubble.
 * Slides up with a shorter animation than SystemBubble.
 */
export function UserBubble({ children, className }: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn(
      'flex justify-end animate-in fade-in slide-in-from-bottom-2 duration-200',
      className,
    )}>
      <div className="rounded-2xl rounded-br-sm bg-primary/10 border border-primary/20 px-4 py-2 text-sm font-medium text-foreground max-w-[80%]">
        {children}
      </div>
    </div>
  );
}

/**
 * Segmented progress bar for conversational wizards.
 * Each segment represents a step; filled segments are past, half-filled is current.
 */
export function ConversationProgress({ steps, currentIndex }: {
  steps: string[];
  currentIndex: number;
}) {
  return (
    <div className="px-6 py-3 border-t border-border/50 bg-muted/20">
      <div className="flex gap-1.5">
        {steps.map((s, i) => (
          <div
            key={s}
            className={cn(
              'h-1 flex-1 rounded-full transition-colors duration-300',
              i < currentIndex
                ? 'bg-primary'
                : i === currentIndex
                  ? 'bg-primary/50'
                  : 'bg-muted',
            )}
          />
        ))}
      </div>
    </div>
  );
}

/**
 * Inline input row with submit button (Enter key supported).
 */
export function StepInput({ value, onChange, onSubmit, placeholder, disabled, error, maxLength }: {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  placeholder?: string;
  disabled?: boolean;
  error?: string | null;
  maxLength?: number;
}) {
  return (
    <div className="animate-in fade-in slide-in-from-bottom-2 duration-300 space-y-1.5">
      <div className="flex gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !disabled) onSubmit(); }}
          placeholder={placeholder}
          disabled={disabled}
          maxLength={maxLength}
          autoFocus
          className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        />
        <button
          type="button"
          onClick={onSubmit}
          disabled={disabled || !value.trim()}
          className="inline-flex items-center justify-center rounded-md bg-primary text-primary-foreground h-9 w-9 shrink-0 hover:bg-primary/90 disabled:opacity-50 disabled:pointer-events-none transition-colors"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>
        </button>
      </div>
      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}
      {maxLength && (
        <p className="text-[10px] text-muted-foreground/60 text-right tabular-nums">{value.length}/{maxLength}</p>
      )}
    </div>
  );
}
