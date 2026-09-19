'use client';

import { useEffect, useState } from 'react';
import { ChevronDown, Info } from 'lucide-react';
import { cn } from '@/lib/utils';

const STORAGE_KEY = 'escrow-briefing-collapsed';

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
    <section className="rounded-lg border border-border bg-muted/30">
      <button
        type="button"
        onClick={toggle}
        aria-expanded={open}
        className="flex w-full items-center gap-2 px-4 py-3 text-left text-sm font-medium"
      >
        <Info className="h-4 w-4 text-muted-foreground" />
        How escrow exchange works
        <ChevronDown className={cn('ml-auto h-4 w-4 text-muted-foreground transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <div className="space-y-3 border-t border-border px-4 py-3 text-sm text-muted-foreground">
          <p>
            Deposits move between a <strong className="font-medium text-foreground">source</strong> and a{' '}
            <strong className="font-medium text-foreground">destination</strong>. Each one is signed, so the other side
            can tell who sent it, and encrypted, so only the intended recipient can open it.
          </p>
          <div className="space-y-1">
            <p className="font-medium text-foreground">Deposits coming to us</p>
            <p className="font-mono text-xs">
              source signs → encrypts it for us → sends → we decrypt → we verify the signature
            </p>
            <p>
              So we need two keys per source: <strong className="font-medium text-foreground">their public
              verification key</strong>, which they give us and which is not secret, and{' '}
              <strong className="font-medium text-foreground">our own private decryption key</strong>, which never
              leaves the key store. They need one thing from us: the public half of that decryption key.
            </p>
          </div>
          <div className="space-y-1">
            <p className="font-medium text-foreground">Deposits going out</p>
            <p>
              The same steps, reversed — we sign and encrypt for a destination. Sending deposits out is not built yet.
            </p>
          </div>
          <p>
            You do not have to manage the cryptography by hand. Each step asks for exactly one key and says what it is
            used for.
          </p>
        </div>
      )}
    </section>
  );
}
