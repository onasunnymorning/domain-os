"use client";

import { useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { RetireCredentialDialog, type RetireTarget } from "@/components/epp/RetireCredentialDialog";
import { EndSessionsDialog, type EndSessionsTarget } from "@/components/epp/live/EndSessionsDialog";
import { CreateOutcomesChart } from "./CreateOutcomesChart";
import {
  useEPPAccess,
  useEPPLockouts,
  useEPPSessions,
  useEPPSuspensions,
  useRegistrarEPPPolicy,
  eppErrorMessage,
} from "@/lib/hooks/useEPP";
import { EPPStatusStrip } from "./EPPStatusStrip";
import { EPPLoginsSection } from "./EPPLoginsSection";
import { EPPNetworksSection } from "./EPPNetworksSection";
import { EPPCertificatesSection } from "./EPPCertificatesSection";
import { EPPLimitsSection } from "./EPPLimitsSection";

/**
 * Everything about one registrar's EPP access, top to bottom: can it log in
 * now, who (logins), from where (networks), with what (certificates), and how
 * much (limits).
 */
export function RegistrarEPPTab({ clid }: { clid: string }) {
  const access = useEPPAccess(clid);
  const policy = useRegistrarEPPPolicy(clid);
  const lockouts = useEPPLockouts(clid);
  const suspensions = useEPPSuspensions(clid);
  const sessions = useEPPSessions({ clid, pagesize: 100 });
  const [retireTarget, setRetireTarget] = useState<RetireTarget | null>(null);
  const [endTarget, setEndTarget] = useState<EndSessionsTarget | null>(null);

  if (access.error) {
    return (
      <div className="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
        {eppErrorMessage(access.error, "Could not load this registrar's EPP access.")}
      </div>
    );
  }

  // The list is cursor-paginated: a non-empty PageCursor means a further page,
  // so the first page's length is a lower bound, not a total.
  const sessionCount = sessions.data ? sessions.data.Data.length : undefined;
  const moreSessions = !!sessions.data?.Meta?.PageCursor;

  return (
    <div className="space-y-6">
      <EPPStatusStrip
        clid={clid}
        access={access.data}
        lockouts={{ data: lockouts.data, error: lockouts.error, isLoading: lockouts.isLoading }}
        suspensions={{ data: suspensions.data, error: suspensions.error, isLoading: suspensions.isLoading }}
        sessionCount={sessionCount}
        moreSessions={moreSessions}
        isLoading={access.isLoading}
        onEndAllSessions={() =>
          setEndTarget({ kind: "matching", filter: { clid }, count: sessionCount ?? 0, label: `of registrar ${clid}` })
        }
      />

      {access.isLoading || !access.data ? (
        <div className="space-y-6">
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-32 w-full" />
          <Skeleton className="h-32 w-full" />
        </div>
      ) : (
        <>
          <EPPLoginsSection
            clid={clid}
            logins={access.data.logins}
            lockouts={lockouts.data ?? []}
            suspensions={suspensions.data ?? []}
            onRetire={setRetireTarget}
          />
          <EPPNetworksSection clid={clid} networks={access.data.allowed_networks} onRetire={setRetireTarget} />
          <EPPCertificatesSection
            clid={clid}
            pins={access.data.certificate_pins}
            logins={access.data.logins}
            onRetire={setRetireTarget}
          />
        </>
      )}

      <EPPLimitsSection clid={clid} policy={policy.data} isLoading={policy.isLoading} error={policy.error}>
        <CreateOutcomesChart clid={clid} />
      </EPPLimitsSection>

      <RetireCredentialDialog clid={clid} target={retireTarget} onOpenChange={(v) => !v && setRetireTarget(null)} />
      <EndSessionsDialog target={endTarget} onOpenChange={(v) => !v && setEndTarget(null)} />
    </div>
  );
}
