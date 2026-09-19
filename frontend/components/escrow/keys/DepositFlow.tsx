'use client';

import { ArrowDown, ArrowRight, Lock, LockOpen, Package, ShieldCheck, Signature } from 'lucide-react';
import { cn } from '@/lib/utils';

/**
 * One deposit, drawn end to end: who signs it, who seals it, who opens it.
 *
 * Both directions are the same four steps mirrored, so they are one component
 * with the sides swapped rather than two drawings that can drift apart. The
 * outbound rail is drawn unbuilt on purpose — dashed, muted, and saying so —
 * because an operator who sees it should be able to tell at a glance that
 * nothing here is waiting on them.
 */

interface Step {
  Icon: typeof Lock;
  title: string;
  detail: string;
}

interface Side {
  /** The heading above the pair of steps, e.g. "Their side · the source". */
  label: string;
  /** Ours is drawn in the accent; theirs is drawn dashed, as something we do not hold. */
  ours: boolean;
  steps: [Step, Step];
}

interface Flow {
  title: string;
  /** What actually happens in the middle, plus what the file carries across it. */
  crossing: { title: string; chips: [string, string] };
  from: Side;
  to: Side;
  hint: string;
  footnote: string;
}

const INBOUND: Flow = {
  title: 'Deposits coming to us',
  from: {
    label: 'Their side · the source',
    ours: false,
    steps: [
      { Icon: Signature, title: 'They sign it', detail: 'with their own private signing key' },
      { Icon: Lock, title: 'They seal it for us', detail: 'with the public half of our key' },
    ],
  },
  crossing: { title: 'They send it', chips: ['signed by them', 'sealed for us'] },
  to: {
    label: 'Our side',
    ours: true,
    steps: [
      { Icon: LockOpen, title: 'We open it', detail: 'our private key does this inside the key store' },
      { Icon: ShieldCheck, title: 'We check the signature', detail: 'against the key they gave us' },
    ],
  },
  hint: 'read left to right',
  footnote: 'Dashed is not ours. Nothing secret crosses the middle — only one sealed file.',
};

const OUTBOUND: Flow = {
  title: 'Deposits going out',
  from: {
    label: 'Our side',
    ours: true,
    steps: [
      { Icon: Signature, title: 'We sign it', detail: 'with a private signing key of our own' },
      { Icon: Lock, title: 'We seal it for them', detail: "with the public half of the destination's key" },
    ],
  },
  crossing: { title: 'We send it', chips: ['signed by us', 'sealed for them'] },
  to: {
    label: 'Their side · the destination',
    ours: false,
    steps: [
      { Icon: LockOpen, title: 'They open it', detail: 'with a private key we never see' },
      { Icon: ShieldCheck, title: 'They check our signature', detail: 'against the key we gave them' },
    ],
  },
  hint: 'the same four steps, mirrored',
  footnote:
    'Two things are missing before this can run: a destination to point at, and a signing key of our own. Everything else already exists.',
};

export function DepositFlow({ direction }: { direction: 'inbound' | 'outbound' }) {
  const built = direction === 'inbound';
  const flow = built ? INBOUND : OUTBOUND;

  return (
    <section
      aria-label={flow.title}
      className={cn(
        'flex flex-col gap-4 rounded-xl border p-5',
        built ? 'border-border bg-card' : 'border-dashed border-border bg-muted/20'
      )}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <h4
          className={cn(
            'text-[11px] font-bold uppercase tracking-[0.1em]',
            built ? 'text-primary' : 'text-muted-foreground'
          )}
        >
          {flow.title}
        </h4>
        {built ? (
          <Chip className="border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" aria-hidden />
            Live today
          </Chip>
        ) : (
          <Chip>Not built yet</Chip>
        )}
        <span className="ml-auto text-xs text-muted-foreground">{flow.hint}</span>
      </div>

      <div className="flex flex-col items-stretch gap-3 lg:flex-row lg:items-start">
        <SideBox side={flow.from} built={built} />
        <Crossing crossing={flow.crossing} built={built} />
        <SideBox side={flow.to} built={built} />
      </div>

      <p className="flex items-start gap-2 text-xs leading-relaxed text-muted-foreground">
        <span className="mt-2 hidden w-5 shrink-0 border-t border-dashed border-muted-foreground/60 sm:block" aria-hidden />
        {flow.footnote}
      </p>
    </section>
  );
}

function SideBox({ side, built }: { side: Side; built: boolean }) {
  const accent = side.ours && built;
  return (
    <div
      className={cn(
        'min-w-0 flex-1 rounded-lg border p-4',
        accent ? 'border-primary/25 bg-primary/5' : 'border-dashed border-border bg-background/60'
      )}
    >
      <p
        className={cn(
          'mb-3 text-[10px] font-bold uppercase tracking-[0.09em]',
          accent ? 'text-primary' : 'text-muted-foreground'
        )}
      >
        {side.label}
      </p>
      <div className="flex items-start gap-1">
        <StepCell step={side.steps[0]} accent={accent} />
        <Connector />
        <StepCell step={side.steps[1]} accent={accent} />
      </div>
    </div>
  );
}

function StepCell({ step: { Icon, title, detail }, accent }: { step: Step; accent: boolean }) {
  return (
    <div className="flex flex-1 flex-col items-center gap-2 text-center">
      <span
        className={cn(
          'flex h-11 w-11 items-center justify-center rounded-full border bg-card',
          accent ? 'border-primary/30 text-primary' : 'border-border text-foreground/80'
        )}
      >
        <Icon className="h-5 w-5" aria-hidden />
      </span>
      <span className="text-[13px] font-semibold leading-tight">{title}</span>
      <span className="text-[11px] leading-snug text-muted-foreground">{detail}</span>
    </div>
  );
}

/** The short hop between two steps on the same side. */
function Connector() {
  return (
    <span className="flex h-11 w-6 items-center justify-center text-muted-foreground/70" aria-hidden>
      <span className="h-px flex-1 bg-current" />
      <ArrowRight className="h-3 w-3 shrink-0" />
    </span>
  );
}

/**
 * The hand-off. It is the only part of the drawing that carries a label for
 * what the file is at that moment — signed, sealed — which is the whole point
 * of the picture.
 */
function Crossing({ crossing, built }: { crossing: Flow['crossing']; built: boolean }) {
  return (
    <div className="flex shrink-0 flex-col items-center gap-2 lg:w-44 lg:pt-10">
      <span
        className={cn(
          'flex w-full items-center justify-center gap-1',
          built ? 'text-primary' : 'text-muted-foreground'
        )}
        aria-hidden
      >
        <span className="hidden h-px flex-1 bg-current lg:block" />
        <span
          className={cn(
            'flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border',
            built ? 'border-primary/40 bg-primary/10' : 'border-dashed border-border bg-muted/40'
          )}
        >
          <Package className="h-5 w-5" />
        </span>
        <span className="hidden h-px flex-1 bg-current lg:block" />
        <ArrowRight className="hidden h-3 w-3 shrink-0 lg:block" />
        <ArrowDown className="h-3 w-3 shrink-0 lg:hidden" />
      </span>
      <span className="text-[13px] font-semibold">{crossing.title}</span>
      <div className="flex flex-wrap justify-center gap-1.5">
        <Chip>
          <Signature className="h-3 w-3" aria-hidden />
          {crossing.chips[0]}
        </Chip>
        <Chip>
          <Lock className="h-3 w-3" aria-hidden />
          {crossing.chips[1]}
        </Chip>
      </div>
    </div>
  );
}

function Chip({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border border-border bg-muted px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground',
        className
      )}
    >
      {children}
    </span>
  );
}
