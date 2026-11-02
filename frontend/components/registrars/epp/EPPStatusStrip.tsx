"use client";

import Link from "next/link";
import { AlertTriangle, CheckCircle2 } from "lucide-react";
import { differenceInCalendarDays, format } from "date-fns";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import type { EPPLoginLockout, EPPLoginSuspension, EPPRegistrarAccess } from "@/lib/types/epp";

export interface StatusReason {
  key: string;
  text: string;
  /** Anchor of the section that fixes it. */
  href: string;
}

const CERT_WARNING_DAYS = 30;

/**
 * The reasons a registrar cannot log in right now, or will soon not be able
 * to, in the order an operator should fix them. Pure, so it is tested alone.
 */
export function statusReasons(
  access: EPPRegistrarAccess,
  lockouts: EPPLoginLockout[],
  suspensions: EPPLoginSuspension[],
  now = new Date(),
): StatusReason[] {
  const reasons: StatusReason[] = [];
  const usable = <T extends { state: string }>(xs: T[]) => xs.filter((x) => x.state !== "retired");

  const enabledLogins = access.logins.filter((l) => l.enabled);
  if (access.logins.length === 0) {
    reasons.push({ key: "no-login", text: "No login yet. Nobody can connect until one is added.", href: "#epp-logins" });
  } else if (enabledLogins.length === 0) {
    reasons.push({ key: "no-enabled-login", text: "Every login is disabled, so none can log in.", href: "#epp-logins" });
  }
  for (const l of enabledLogins) {
    if (usable(l.passwords).length === 0) {
      reasons.push({
        key: `no-password:${l.login_id}`,
        text: `Login ${l.login_id} has no usable password. Add one so it can log in.`,
        href: "#epp-logins",
      });
    }
  }

  if (usable(access.allowed_networks).length === 0) {
    reasons.push({
      key: "no-network",
      text: "No allowed network. Connections from anywhere are refused until one is added.",
      href: "#epp-networks",
    });
  }

  const pins = usable(access.certificate_pins);
  const validPins = pins.filter((p) => new Date(p.not_after) > now);
  if (pins.length === 0) {
    reasons.push({
      key: "no-cert",
      text: "No certificate. A registrar must connect with a pinned client certificate.",
      href: "#epp-certificates",
    });
  } else if (validPins.length === 0) {
    reasons.push({
      key: "all-certs-expired",
      text: "Every certificate has expired. Add the registrar's current certificate.",
      href: "#epp-certificates",
    });
  } else {
    for (const p of validPins) {
      const days = differenceInCalendarDays(new Date(p.not_after), now);
      if (days <= CERT_WARNING_DAYS) {
        reasons.push({
          key: `cert-expiring:${p.id}`,
          text: `Certificate "${p.subject || p.spki_sha256.slice(0, 12)}" expires in ${days} day${days === 1 ? "" : "s"} (${format(new Date(p.not_after), "MMM d")}). Pin the new one before then.`,
          href: "#epp-certificates",
        });
      }
    }
  }

  for (const lk of lockouts) {
    reasons.push({
      key: `lockout:${lk.login}:${lk.client_ip}`,
      text: `Login ${lk.login} is locked out from ${lk.client_ip} until ${format(new Date(lk.ends_at), "HH:mm")} after too many wrong passwords.`,
      href: "#epp-logins",
    });
  }
  for (const s of suspensions) {
    reasons.push({
      key: `suspension:${s.login}`,
      text: `Login ${s.login} is suspended until ${format(new Date(s.ends_at), "HH:mm")} for creating registered names${s.scope === "family" ? ` (family ${s.family})` : ""}.`,
      href: "#epp-logins",
    });
  }

  return reasons;
}

export function EPPStatusStrip({
  clid,
  access,
  lockouts,
  suspensions,
  sessionCount,
  isLoading,
}: {
  clid: string;
  access?: EPPRegistrarAccess;
  lockouts?: EPPLoginLockout[];
  suspensions?: EPPLoginSuspension[];
  sessionCount?: number;
  isLoading: boolean;
}) {
  if (isLoading || !access) {
    return <Skeleton className="h-16 w-full" />;
  }
  const reasons = statusReasons(access, lockouts ?? [], suspensions ?? []);
  const sessionsText =
    sessionCount === undefined ? null : `${sessionCount} session${sessionCount === 1 ? "" : "s"} open`;

  if (reasons.length === 0) {
    return (
      <Alert className="border-emerald-300 bg-emerald-50 dark:border-emerald-900 dark:bg-emerald-950/40">
        <CheckCircle2 className="text-emerald-600" />
        <AlertTitle>Ready: this registrar can log in</AlertTitle>
        <AlertDescription>
          {sessionsText && (
            <Link href={`/epp?tab=sessions&clid=${encodeURIComponent(clid)}`} className="underline underline-offset-2">
              {sessionsText}
            </Link>
          )}
        </AlertDescription>
      </Alert>
    );
  }

  return (
    <Alert className="border-amber-300 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40">
      <AlertTriangle className="text-amber-600" />
      <AlertTitle>
        {reasons.length === 1 ? "One thing needs attention" : `${reasons.length} things need attention`}
        {sessionsText ? ` · ${sessionsText}` : ""}
      </AlertTitle>
      <AlertDescription>
        <ul className="list-disc space-y-1 pl-4">
          {reasons.map((r) => (
            <li key={r.key}>
              <a href={r.href} className="hover:underline">
                {r.text}
              </a>
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  );
}
