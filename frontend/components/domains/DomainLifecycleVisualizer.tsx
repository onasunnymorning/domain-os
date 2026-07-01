"use client";

import { useMemo } from "react";
import { format } from "date-fns";
import { Badge } from "@/components/ui/badge";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface DomainLifecycleVisualizerProps {
  registeredAt: string;
  expiredAt?: string;
  purgedAt?: string;
  registrar: string;
  roid: string;
  isActive?: boolean;
  events?: any[];
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

interface Milestone {
  label: string;
  date: Date;
  colorClasses: { dot: string; ring: string };
  pulse?: boolean;
}

function formatMilestoneDate(d: Date): string {
  return format(d, "MMM yyyy");
}

const COLORS = {
  green: {
    dot: "bg-green-500",
    ring: "ring-green-500/30",
  },
  blue: {
    dot: "bg-blue-500",
    ring: "ring-blue-500/30",
  },
  amber: {
    dot: "bg-amber-500",
    ring: "ring-amber-500/30",
  },
  red: {
    dot: "bg-red-500",
    ring: "ring-red-500/30",
  },
} as const;

function buildMilestones(props: DomainLifecycleVisualizerProps): Milestone[] {
  const milestones: Milestone[] = [];

  // Registration — always present
  milestones.push({
    label: "Registered",
    date: new Date(props.registeredAt),
    colorClasses: COLORS.green,
  });

  // Renewal events (optional enrichment)
  if (props.events?.length) {
    const renewals = props.events.filter(
      (e: any) =>
        typeof e.type === "string" &&
        e.type.toLowerCase().includes("renew") &&
        e.time
    );
    for (const r of renewals) {
      milestones.push({
        label: "Renewed",
        date: new Date(r.time),
        colorClasses: COLORS.blue,
      });
    }
  }

  // Expiry
  if (props.expiredAt) {
    milestones.push({
      label: "Expired",
      date: new Date(props.expiredAt),
      colorClasses: COLORS.amber,
    });
  }

  // Purge
  if (props.purgedAt) {
    milestones.push({
      label: "Purged",
      date: new Date(props.purgedAt),
      colorClasses: COLORS.red,
    });
  }

  // Active marker
  if (props.isActive) {
    milestones.push({
      label: "Active",
      date: new Date(),
      colorClasses: COLORS.green,
      pulse: true,
    });
  }

  return milestones;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function DomainLifecycleVisualizer(
  props: DomainLifecycleVisualizerProps
) {
  const milestones = useMemo(() => buildMilestones(props), [props]);

  if (milestones.length === 0) return null;

  return (
    <div className="space-y-4">
      {/* Horizontal track (vertical on mobile) */}
      <div className="flex items-start pt-2 max-[480px]:flex-col max-[480px]:items-start max-[480px]:gap-0">
        {milestones.map((m, i) => (
          <div
            key={`${m.label}-${i}`}
            className="flex flex-col items-center relative flex-1 min-w-0 max-[480px]:flex-row max-[480px]:items-center max-[480px]:gap-3 max-[480px]:py-1.5"
          >
            {/* Connector line */}
            {i > 0 && (
              <>
                {/* Horizontal line (desktop) */}
                <div className="absolute top-[7px] right-1/2 w-full h-[2px] bg-border z-0 max-[480px]:hidden" />
                {/* Vertical line (mobile) */}
                <div className="hidden max-[480px]:block absolute left-[7px] bottom-1/2 w-[2px] h-full bg-border z-0" />
              </>
            )}

            {/* Dot */}
            <div
              className={`
                w-3.5 h-3.5 rounded-full relative z-10 flex-shrink-0
                ring-[3px] ring-background
                ${m.colorClasses.dot}
                shadow-[0_0_0_1px] shadow-current/20
                transition-transform duration-200 ease-out
                hover:scale-125
                ${m.pulse ? "animate-pulse" : ""}
              `}
              title={`${m.label}: ${format(m.date, "PPP")}`}
            />

            {/* Label + date */}
            <div className="flex flex-col items-center mt-2 gap-px max-[480px]:flex-row max-[480px]:gap-1.5 max-[480px]:mt-0">
              <span className="text-xs font-medium whitespace-nowrap">
                {m.label}
              </span>
              <span className="text-[0.65rem] text-muted-foreground whitespace-nowrap">
                {formatMilestoneDate(m.date)}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Metadata badges */}
      <div className="flex gap-1.5 flex-wrap">
        <Badge variant="outline" className="font-mono text-xs">
          {props.roid}
        </Badge>
        <Badge variant="secondary" className="text-xs">
          {props.registrar}
        </Badge>
      </div>
    </div>
  );
}
