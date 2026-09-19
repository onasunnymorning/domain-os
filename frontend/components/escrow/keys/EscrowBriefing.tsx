'use client';

import { useEffect, useState } from 'react';
import { ArrowDownToLine, ArrowUpFromLine, ChevronUp, Info, Lock } from 'lucide-react';
import { DepositFlow } from '@/components/escrow/keys/DepositFlow';
import { cn } from '@/lib/utils';

const STORAGE_KEY = 'escrow-briefing-collapsed';

/**
 * What each of the three keys is for, and — the part that prevents the
 * expensive mistake — which of them is secret and which is meant to be handed
 * out. The order matches the way an operator meets them: one arrives, one
 * stays put, one goes back out.
 */
const KEYS = [
  {
    Icon: ArrowDownToLine,
    flow: 'comes to us',
    title: 'Their public verification key',
    body: 'They give it to us. It is not secret. We use it to prove a deposit really came from them.',
    className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  },
  {
    Icon: Lock,
    flow: 'stays put',
    title: 'Our private decryption key',
    body: 'It never leaves the key store. The key store opens the file for us — you never handle it.',
    className: 'border-border bg-muted text-foreground',
  },
  {
    Icon: ArrowUpFromLine,
    flow: 'goes to them',
    title: 'The public half of that key',
    body: 'The one thing a source needs from us. Copy it from the receiving identity below and send it over.',
    className: 'border-primary/30 bg-primary/10 text-primary',
  },
];

/**
 * The two minutes of background that stop the expensive mistakes: which key is
 * secret, which one is meant to be handed out, and who does what to a deposit.
 *
 * It opens the first time and stays shut once collapsed, per browser. The
 * preference is a convenience, so a browser that refuses storage simply gets
 * the open panel again.
 */
export function EscrowBriefing() {
  const [open, setOpen] = useState(true);

  useEffect(() => {
    try {
      if (window.localStorage.getItem(STORAGE_KEY) === '1') setOpen(false);
    } catch {
      // Private windows and blocked site data: showing the panel is the safe default.
    }
  }, []);

  const toggle = () => {
    setOpen((was) => {
      try {
        window.localStorage.setItem(STORAGE_KEY, was ? '1' : '0');
      } catch {
        // Not worth telling anyone about: only the panel's default state is lost.
      }
      return !was;
    });
  };

  return (
    <section className="overflow-hidden rounded-xl border border-primary/20 bg-primary/[0.04]">
      <button
        type="button"
        onClick={toggle}
        aria-expanded={open}
        className={cn(
          'flex w-full items-center gap-3 px-5 py-3 text-left',
          open && 'border-b border-primary/15'
        )}
      >
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-primary/50 text-primary">
          <Info className="h-3.5 w-3.5" aria-hidden />
        </span>
        <h2 className="text-[15px] font-semibold">How escrow exchange works</h2>
        <span className="hidden text-xs text-muted-foreground sm:inline">the shape of it, in one picture</span>
        <ChevronUp
          className={cn('ml-auto h-4 w-4 shrink-0 text-muted-foreground transition-transform', !open && 'rotate-180')}
          aria-hidden
        />
      </button>

      {open && (
        <div className="flex flex-col gap-5 px-5 py-5">
          <p className="max-w-3xl text-sm leading-relaxed text-muted-foreground">
            Deposits move between a <strong className="font-semibold text-foreground">source</strong> and a{' '}
            <strong className="font-semibold text-foreground">destination</strong>. Each one is{' '}
            <strong className="font-semibold text-foreground">signed</strong>, so the other side can tell who sent it,
            and <strong className="font-semibold text-foreground">encrypted</strong>, so only the intended recipient
            can open it.
          </p>

          <DepositFlow direction="inbound" />

          <div className="flex flex-col gap-3">
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <h3 className="text-[11px] font-bold uppercase tracking-[0.1em] text-primary">The keys this needs</h3>
              <span className="text-xs text-muted-foreground">two you hold, one you hand out</span>
            </div>
            <ul className="grid gap-3 md:grid-cols-3">
              {KEYS.map(({ Icon, flow, title, body, className }) => (
                <li key={title} className="flex flex-col gap-2 rounded-xl border border-border bg-card p-4">
                  <div className="flex items-center gap-2">
                    <span className={cn('flex h-9 w-9 items-center justify-center rounded-lg border', className)}>
                      <Icon className="h-4 w-4" aria-hidden />
                    </span>
                    <span
                      className={cn(
                        'ml-auto rounded-full border px-2.5 py-0.5 text-[11px] font-medium',
                        className
                      )}
                    >
                      {flow}
                    </span>
                  </div>
                  <span className="text-[13px] font-semibold leading-snug">{title}</span>
                  <span className="text-xs leading-relaxed text-muted-foreground">{body}</span>
                </li>
              ))}
            </ul>
          </div>

          <DepositFlow direction="outbound" />

          <p className="flex items-center gap-3 text-sm leading-relaxed text-muted-foreground">
            <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
              <Info className="h-3.5 w-3.5" aria-hidden />
            </span>
            You do not have to manage any of this by hand. Each step below asks for exactly one key and says what it is
            used for.
          </p>
        </div>
      )}
    </section>
  );
}
