'use client';

import { useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Download, FileSearch, FlaskConical, Loader2 } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { generateSyntheticDeposit } from '@/lib/api/escrow-synthetic';
import {
  SYNTHETIC_LIMITS,
  estimateSyntheticBytes,
  formatBytes,
  syntheticCounts,
  syntheticParamsProblem,
  type SyntheticDepositParams,
} from '@/lib/escrow/synthetic';

/** Saves a Blob through a temporary object URL. */
function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Revoke after the click has been handled, not synchronously.
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

/**
 * Generates a made-up FULL RDE deposit of a chosen shape, for load-testing an
 * import, exercising the validator or a demo. The deposit is valid by this
 * registry's own validator; nothing in it is real data.
 */
export default function SyntheticDepositPage() {
  const [tld, setTld] = useState('test');
  const [domains, setDomains] = useState('1000');
  const [contactsPerDomain, setContactsPerDomain] = useState('2');
  const [avgHosts, setAvgHosts] = useState('2');
  const [nndns, setNndns] = useState('0');
  const [seed, setSeed] = useState('');
  const [last, setLast] = useState<{ filename: string; size: number; seed: string | null } | null>(null);
  const abort = useRef<AbortController | null>(null);

  const params: SyntheticDepositParams = {
    tld: tld.trim(),
    domains: domains === '' ? NaN : Number(domains),
    contactsPerDomain: Number(contactsPerDomain),
    avgHostsPerDomain: avgHosts === '' ? NaN : Number(avgHosts),
    nndns: nndns === '' ? NaN : Number(nndns),
    ...(seed.trim() === '' ? {} : { seed: Number(seed) }),
  };
  const problem = syntheticParamsProblem(params);
  const counts = useMemo(
    () => (problem ? null : syntheticCounts(params)),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- params is rebuilt every render; these are its inputs
    [problem, domains, contactsPerDomain, avgHosts, nndns],
  );

  const generate = useMutation({
    mutationFn: () => {
      abort.current = new AbortController();
      return generateSyntheticDeposit(params, abort.current.signal);
    },
    onSuccess: (deposit) => {
      saveBlob(deposit.blob, deposit.filename);
      setLast({ filename: deposit.filename, size: deposit.blob.size, seed: deposit.seed });
      toast.success(`Downloaded ${deposit.filename}`);
    },
    onError: (err) => {
      if (abort.current?.signal.aborted) {
        toast.info('Generation cancelled');
        return;
      }
      toast.error(err instanceof Error ? err.message : 'Could not generate the deposit');
    },
    onSettled: () => {
      abort.current = null;
    },
  });

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-semibold">
              <FlaskConical className="h-6 w-6" />
              Synthetic deposit
            </h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Generate a made-up FULL escrow deposit (RFC 8909 / RFC 9022) as gzip-compressed XML. Every contact and
              host belongs to a domain and the header counts are exact, so our validator reports nothing against it.
              Nothing in it is real: names come from word lists, addresses are in documentation ranges.
            </p>
          </div>
          <Button variant="outline" asChild>
            <Link href="/escrow/validations" className="gap-1.5">
              <FileSearch className="h-4 w-4" />
              Escrow runs
            </Link>
          </Button>
        </div>

        <form
          className="space-y-4 rounded-lg border border-border p-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (!problem && !generate.isPending) generate.mutate();
          }}
        >
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Field id="tld" label="TLD" hint="e.g. test or co.test; xn-- form for an IDN TLD">
              <Input id="tld" value={tld} onChange={(e) => setTld(e.target.value)} placeholder="test" autoComplete="off" />
            </Field>
            <Field id="domains" label="Domains" hint={`0 – ${SYNTHETIC_LIMITS.domains.toLocaleString('en-US')}`}>
              <Input id="domains" type="number" inputMode="numeric" min={0} max={SYNTHETIC_LIMITS.domains} step={1} value={domains} onChange={(e) => setDomains(e.target.value)} />
            </Field>
            <Field id="contacts" label="Contacts per domain" hint="Registrant, admin, tech, billing — each its own contact">
              <select
                id="contacts"
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                value={contactsPerDomain}
                onChange={(e) => setContactsPerDomain(e.target.value)}
              >
                {[0, 1, 2, 3, 4].map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </Field>
            <Field id="hosts" label="Average hosts per domain" hint={`0 – ${SYNTHETIC_LIMITS.avgHostsPerDomain}; fractions spread evenly (2.5 → 2 and 3)`}>
              <Input id="hosts" type="number" inputMode="decimal" min={0} max={SYNTHETIC_LIMITS.avgHostsPerDomain} step={0.1} value={avgHosts} onChange={(e) => setAvgHosts(e.target.value)} />
            </Field>
            <Field id="nndns" label="NNDNs" hint={`Blocked or withheld names, 0 – ${SYNTHETIC_LIMITS.nndns.toLocaleString('en-US')}`}>
              <Input id="nndns" type="number" inputMode="numeric" min={0} max={SYNTHETIC_LIMITS.nndns} step={1} value={nndns} onChange={(e) => setNndns(e.target.value)} />
            </Field>
            <Field id="seed" label="Seed (optional)" hint="Same seed and settings give the same domains; blank picks one">
              <Input id="seed" type="number" inputMode="numeric" min={0} step={1} value={seed} onChange={(e) => setSeed(e.target.value)} placeholder="random" />
            </Field>
          </div>

          {problem ? (
            <p role="alert" className="text-sm text-destructive">
              {problem}
            </p>
          ) : (
            counts && (
              <p className="text-sm text-muted-foreground" data-testid="synthetic-summary">
                {counts.domains.toLocaleString('en-US')} domains · {counts.contacts.toLocaleString('en-US')} contacts ·{' '}
                {counts.hosts.toLocaleString('en-US')} hosts · {counts.registrars} registrars · {counts.nndns.toLocaleString('en-US')} NNDNs — about{' '}
                {formatBytes(estimateSyntheticBytes(counts))} compressed
              </p>
            )
          )}

          <div className="flex flex-wrap items-center gap-2">
            <Button type="submit" disabled={!!problem || generate.isPending} className="gap-1.5">
              {generate.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
              {generate.isPending ? 'Generating…' : 'Generate .xml.gz'}
            </Button>
            {generate.isPending && (
              <Button type="button" variant="outline" onClick={() => abort.current?.abort()}>
                Cancel
              </Button>
            )}
          </div>
        </form>

        {last && (
          <p className="text-sm text-muted-foreground" data-testid="synthetic-last">
            Last download: <span className="font-mono">{last.filename}</span> ({formatBytes(last.size)})
            {last.seed && (
              <>
                {' '}— seed <span className="font-mono">{last.seed}</span>
              </>
            )}
            . To validate it, start an escrow validation from{' '}
            <Link href="/workflows" className="underline underline-offset-2">
              Workflows
            </Link>{' '}
            with the Plaintext (.xml or .xml.gz) profile.
          </p>
        )}
      </div>
    </DashboardLayout>
  );
}

function Field({ id, label, hint, children }: { id: string; label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}
