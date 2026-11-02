"use client";

import { useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { RetireCredentialDialog, type RetireTarget } from "@/components/epp/RetireCredentialDialog";
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

  if (access.error) {
    return (
      <div className="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
        {eppErrorMessage(access.error, "Could not load this registrar's EPP access.")}
      </div>
    );
  }

  const sessionCount = sessions.data
    ? sessions.data.Data.length + (sessions.data.Meta?.NextCursor ? 1 : 0)
    : undefined;

  return (
    <div className="space-y-6">
      <EPPStatusStrip
        clid={clid}
        access={access.data}
        lockouts={lockouts.data}
        suspensions={suspensions.data}
        sessionCount={sessionCount}
        isLoading={access.isLoading}
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

      <EPPLimitsSection clid={clid} policy={policy.data} isLoading={policy.isLoading} />

      <RetireCredentialDialog clid={clid} target={retireTarget} onOpenChange={(v) => !v && setRetireTarget(null)} />
    </div>
  );
}
