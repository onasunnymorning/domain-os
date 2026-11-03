"use client";

import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { format } from "date-fns";
import { Skeleton } from "@/components/ui/skeleton";
import { useRegistrarEPPCreateOutcomes } from "@/lib/hooks/useEPP";

/** The API's day is midnight UTC; label it by its UTC calendar date, whatever the browser's zone. */
export function utcDayLabel(day: string): string {
  const d = new Date(day);
  return format(new Date(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()), "MMM d");
}

/**
 * Domain creates per UTC day over the last 30 days: created, and refused
 * because the name was taken (the failed-create budget's count). Rendered
 * only when any day has data.
 */
export function CreateOutcomesChart({ clid }: { clid: string }) {
  const q = useRegistrarEPPCreateOutcomes(clid);
  if (q.isLoading) return <Skeleton className="h-40 w-full" />;
  const rows = (q.data ?? []).filter((r) => r.succeeded > 0 || r.failed > 0);
  if (q.error || rows.length === 0) return null;
  const data = rows
    .slice()
    .sort((a, b) => a.day.localeCompare(b.day))
    .map((r) => ({ day: utcDayLabel(r.day), Created: r.succeeded, "Refused, name taken": r.failed }));
  return (
    <div className="space-y-2">
      <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Domain creates, last 30 days</div>
      <div className="h-40 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="currentColor" opacity={0.15} />
            <XAxis dataKey="day" tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} allowDecimals={false} />
            <Tooltip />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            {/* The theme's variables are complete colours (oklch), used as they are. */}
            <Bar dataKey="Created" stackId="a" fill="var(--primary)" />
            <Bar dataKey="Refused, name taken" stackId="a" fill="var(--muted-foreground)" opacity={0.6} />
          </BarChart>
        </ResponsiveContainer>
      </div>
      <p className="text-xs text-muted-foreground">
        Many refusals for taken names in a short time is what the failed-create budget catches.
      </p>
    </div>
  );
}
