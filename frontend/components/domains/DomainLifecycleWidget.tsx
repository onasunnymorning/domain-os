import { DomainDetail } from "@/lib/types/domain";
import { useCategorizedPhases } from "@/lib/hooks/usePhases";
import { Card, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { useMemo } from "react";
import { format, differenceInMilliseconds, formatDistanceToNowStrict } from "date-fns";
import { Badge } from "@/components/ui/badge";

interface Props {
  domain: DomainDetail;
}

export function DomainLifecycleWidget({ domain }: Props) {
  const { categorized } = useCategorizedPhases(domain.TLDName || "");

  const policy = useMemo(() => {
    return categorized?.ga?.current?.policy || categorized?.launch?.current?.[0]?.policy;
  }, [categorized]);

  // Analyze Registration Period
  const registrationPeriod = useMemo(() => {
    if (!domain.CreatedAt || !domain.ExpiryDate) return null;

    const now = new Date();
    const created = new Date(domain.CreatedAt);
    const expiry = new Date(domain.ExpiryDate);

    // If we're past expiry, we just show the final year ending at expiry
    // Otherwise, we find the most recent anniversary to the next anniversary
    let startYearDate = new Date(created);
    startYearDate.setFullYear(now.getFullYear());

    if (now < startYearDate && startYearDate > created) {
      startYearDate.setFullYear(now.getFullYear() - 1);
    }
    
    let endYearDate = new Date(startYearDate);
    endYearDate.setFullYear(startYearDate.getFullYear() + 1);

    // If endYearDate exceeds expiry and we are in the final year, cap at expiry
    if (endYearDate > expiry) {
      endYearDate = expiry;
      startYearDate = new Date(expiry);
      startYearDate.setFullYear(expiry.getFullYear() - 1);
    }

    const totalMs = differenceInMilliseconds(endYearDate, startYearDate);
    const elapsedMs = differenceInMilliseconds(now, startYearDate);

    let progress = totalMs > 0 ? (elapsedMs / totalMs) * 100 : 0;
    progress = Math.max(0, Math.min(100, progress));

    const additionalYears = Math.round((expiry.getTime() - endYearDate.getTime()) / (1000 * 60 * 60 * 24 * 365.25));

    return {
      start: startYearDate,
      end: endYearDate,
      progress,
      isExpired: now > expiry,
      additionalYears: additionalYears > 0 ? additionalYears : 0,
    };
  }, [domain.CreatedAt, domain.ExpiryDate]);

  // Analyze Grace Periods
  const gracePeriods = useMemo(() => {
    if (!domain.RGPStatus) return [];
    const now = new Date();

    const activePeriods: { name: string; end: Date; start: Date; progress: number }[] = [];

    const addPeriod = (endDateStr: string | undefined, name: string, durationDays: number | undefined, defaultDays: number) => {
      if (!endDateStr) return;
      const end = new Date(endDateStr);
      if (end > now) {
        const days = durationDays ?? defaultDays;
        const start = new Date(end);
        start.setDate(start.getDate() - days);

        const totalMs = differenceInMilliseconds(end, start);
        const elapsedMs = differenceInMilliseconds(now, start);
        let progress = totalMs > 0 ? (elapsedMs / totalMs) * 100 : 0;
        progress = Math.max(0, Math.min(100, progress));

        activePeriods.push({ name, end, start, progress });
      }
    };

    // Default durations (in days) fallback if policy is not loaded or missing
    addPeriod(domain.RGPStatus.addPeriodEnd, "Add Grace Period", policy?.registrationGP, 5);
    addPeriod(domain.RGPStatus.autoRenewPeriodEnd, "Auto-Renew Grace Period", policy?.autoRenewalGP, 45);
    addPeriod(domain.RGPStatus.renewPeriodEnd, "Renew Grace Period", policy?.renewalGP, 5);
    addPeriod(domain.RGPStatus.transferLockPeriodEnd, "Transfer Lock", policy?.transferLockPeriod, 60);
    addPeriod(domain.RGPStatus.redemptionPeriodEnd, "Redemption Grace Period", policy?.redemptionGP, 30);
    addPeriod(domain.RGPStatus.purgeDate, "Pending Delete", policy?.pendingdeleteGP, 5);

    return activePeriods;
  }, [domain.RGPStatus, policy]);

  if (!domain.CreatedAt && !domain.ExpiryDate && gracePeriods.length === 0) {
    return null;
  }

  return (
    <Card>
      <CardContent className="space-y-6">
        {/* Domain Age / Expiry */}
        {(domain.CreatedAt || domain.ExpiryDate) && (
          <div className="flex items-center gap-2 pb-2 flex-wrap">
            {domain.CreatedAt && (
              <>
                <div className="text-lg font-semibold text-primary">
                  {formatDistanceToNowStrict(new Date(domain.CreatedAt))} old
                </div>
                <div className="text-sm text-muted-foreground mt-0.5">
                  ({format(new Date(domain.CreatedAt), "MMM d, yyyy")})
                </div>
              </>
            )}
            
            {domain.CreatedAt && domain.ExpiryDate && (
              <div className="text-muted-foreground text-sm mx-1">•</div>
            )}
            
            {domain.ExpiryDate && new Date(domain.ExpiryDate) > new Date() ? (
              <>
                <div className="text-sm font-medium">
                  expires in {formatDistanceToNowStrict(new Date(domain.ExpiryDate))}
                </div>
                <div className="text-sm text-muted-foreground mt-0.5">
                  ({format(new Date(domain.ExpiryDate), "MMM d, yyyy")})
                </div>
              </>
            ) : domain.ExpiryDate && new Date(domain.ExpiryDate) <= new Date() ? (
              <>
                <div className="text-sm font-medium text-destructive">
                  expired {formatDistanceToNowStrict(new Date(domain.ExpiryDate))} ago
                </div>
                <div className="text-sm text-muted-foreground mt-0.5">
                  ({format(new Date(domain.ExpiryDate), "MMM d, yyyy")})
                </div>
              </>
            ) : null}
          </div>
        )}

        {/* Registration Year Progress */}
        {registrationPeriod && (
          <div className="space-y-2">
            <div className="flex justify-between items-center text-sm">
              <span className="font-medium">
                Current Registration Year
                {registrationPeriod.additionalYears > 0 && (
                  <span className="text-muted-foreground font-normal ml-2">
                    (+ {registrationPeriod.additionalYears} year{registrationPeriod.additionalYears !== 1 ? 's' : ''} registered)
                  </span>
                )}
                {registrationPeriod.isExpired && <Badge variant="destructive" className="ml-2 py-0 h-5">Expired</Badge>}
              </span>
              <span className="text-muted-foreground">{Math.round(registrationPeriod.progress)}%</span>
            </div>
            <Progress value={registrationPeriod.progress} className="h-2" />
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>{format(registrationPeriod.start, "MMM d, yyyy")}</span>
              <span>{format(registrationPeriod.end, "MMM d, yyyy")}</span>
            </div>
          </div>
        )}

        {/* Grace Periods */}
        {gracePeriods.length > 0 && (
          <div className="space-y-4 pt-4 border-t">
            <div className="text-sm font-medium">Active Grace Periods</div>
            {gracePeriods.map((gp, idx) => (
              <div key={idx} className="space-y-2">
                <div className="flex justify-between items-center text-sm">
                  <span>{gp.name}</span>
                  <span className="text-muted-foreground">{Math.round(gp.progress)}%</span>
                </div>
                <Progress value={gp.progress} className="h-2" />
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>Started: {format(gp.start, "MMM d")}</span>
                  <span>Ends: {format(gp.end, "MMM d, yyyy HH:mm")}</span>
                </div>
              </div>
            ))}
          </div>
        )}
        
        {gracePeriods.length === 0 && (
          <div className="pt-2 text-sm text-muted-foreground">
            No active grace periods.
          </div>
        )}
      </CardContent>
    </Card>
  );
}
