'use client';

import { Suspense } from 'react';
import Link from 'next/link';
import { useParams, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { EscrowRunOverview } from '@/components/escrow/EscrowRunOverview';
import { EscrowFindingsPanel } from '@/components/escrow/EscrowFindingsPanel';
import { EscrowArtifactLinks } from '@/components/escrow/EscrowArtifactLinks';
import { CopyButton } from '@/components/ui/copy-button';
import { getEscrowValidation } from '@/lib/api/escrow-runs';
import { formatUtc } from '@/lib/escrow/runOverview';

export default function EscrowValidationRunPage() {
  return (
    <Suspense
      fallback={
        <DashboardLayout>
          <div className="flex justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        </DashboardLayout>
      }
    >
      <RunDetail />
    </Suspense>
  );
}

function RunDetail() {
  const params = useParams<{ id: string }>();
  const search = useSearchParams();
  const tenantId = search.get('tenantId') ?? '';
  const id = params.id;

  const { data, isLoading, isError } = useQuery({
    queryKey: ['escrow-validation', tenantId, id],
    queryFn: () => getEscrowValidation(tenantId, id),
    enabled: tenantId !== '' && Boolean(id),
  });

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <Link
          href="/escrow/validations"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          All validation runs
        </Link>

        {!tenantId ? (
          <Notice>
            This run can only be opened in its operator scope. Return to the list and pick
            the operator that owns it.
          </Notice>
        ) : isLoading ? (
          <div className="flex justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : isError || !data ? (
          <Notice>
            This run was not found in this operator&apos;s scope. A run belongs to exactly one
            operator, and one operator cannot read another&apos;s.
          </Notice>
        ) : (
          <Loaded detail={data} />
        )}
      </div>
    </DashboardLayout>
  );
}

function Loaded({ detail }: { detail: Awaited<ReturnType<typeof getEscrowValidation>> }) {
  const { run, deposit } = detail;
  const signed = run.profile === 'ryde+sig';

  return (
    <>
      <EscrowRunOverview run={run} deposit={deposit} />

      <section className="space-y-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          Run
        </h2>
        <dl className="grid gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
          <Field label="Run ID" value={run.id} mono copyable />
          <Field label="Deposit ID" value={run.depositId} mono copyable />
          <Field label="Stage reached" value={run.stageReached} />
          <Field label="Workflow ID" value={run.workflowId} mono copyable />
          <Field label="Started" value={formatUtc(run.startedAt)} />
          <Field label="Completed" value={formatUtc(run.completedAt)} />
          <Field
            label="Notification"
            value={
              run.notificationStatus ||
              (signed ? 'none' : 'none — unsigned deposits claim nothing to ICANN')
            }
          />
          <Field label="RDE deposit ID" value={run.rdeDepositId} mono />
          <Field label="RDE kind" value={run.rdeKind} />
          <Field label="Watermark" value={run.rdeWatermark ? formatUtc(run.rdeWatermark) : undefined} />
          <Field label="Plaintext SHA-256" value={run.plaintextSha256} mono copyable />
          <Field label="Signing key" value={run.signingKeyFingerprint} mono />
        </dl>
      </section>

      {run.keys && (
        <section className="space-y-3">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Keys used
          </h2>
          <dl className="grid gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
            <Field label="Signing key version" value={run.keys.signingKeyVersionId} mono copyable />
            <Field label="Decryption key version" value={run.keys.decryptionKeyVersionId} mono copyable />
            <Field label="Depositor party" value={run.keys.depositorPartyId} mono copyable />
            <Field label="Receiver party" value={run.keys.receiverPartyId} mono copyable />
            <Field
              label="Arrangement revisions"
              value={`TLD ${run.keys.tldArrangementRevision ?? 0} · operator ${run.keys.operatorArrangementRevision ?? 0} · platform ${run.keys.platformArrangementRevision ?? 0}`}
            />
          </dl>
        </section>
      )}

      {deposit && (
        <section className="space-y-3">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Deposit
          </h2>
          <dl className="grid gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
            <Field label="Submitted by" value={deposit.submittedBy} />
            <Field label="Intake ref" value={deposit.intakeRef} />
            <Field label="Received" value={formatUtc(deposit.receivedAt)} />
            <Field label="Artifact" value={deposit.artifactObjectKey} mono copyable />
            <Field label="Artifact SHA-256" value={deposit.artifactSha256} mono copyable />
            <Field label="Artifact bytes" value={deposit.artifactBytes?.toLocaleString()} />
            {signed && <Field label="Signature" value={deposit.signatureObjectKey} mono />}
          </dl>
        </section>
      )}

      <section className="space-y-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          Documents
        </h2>
        <EscrowArtifactLinks
          artifacts={[
            {
              label: 'Findings summary',
              hint: 'Every finding, counted exactly by reason code. Written for every run.',
              objectKey: run.summaryObjectKey,
              kind: 'json',
            },
            {
              label: 'rdeReport',
              hint: 'RFC 9022 counts for the deposit. Written once a run reaches a decision.',
              objectKey: run.reportObjectKey,
              kind: 'xml',
            },
            {
              label: 'rdeNotification',
              hint: signed
                ? 'The DVPN/DVFN claim for this deposit.'
                : 'Only a signed deposit produces one.',
              objectKey: run.notificationObjectKey,
              kind: 'xml',
            },
          ]}
        />
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          Findings
        </h2>
        <EscrowFindingsPanel tally={run.findingTally ?? []} findings={run.findings ?? []} />
      </section>
    </>
  );
}

function Field({
  label,
  value,
  mono,
  copyable,
}: {
  label: string;
  value?: string | number;
  mono?: boolean;
  copyable?: boolean;
}) {
  const text = value === undefined || value === '' ? '—' : String(value);
  return (
    <div className="min-w-0">
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt>
      <dd
        className={`mt-0.5 flex items-center gap-1 break-all text-sm ${mono ? 'font-mono text-xs' : ''}`}
      >
        {text}
        {copyable && text !== '—' && <CopyButton value={text} />}
      </dd>
    </div>
  );
}

function Notice({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed border-border py-12 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
