'use client';

import { DashboardLayout } from '@/components/layout/DashboardLayout';
import {
  Building2,
  Globe,
  Users,
  Server,
  Contact,
  HardDrive,
  Activity,
  Database,
  ExternalLink,
} from 'lucide-react';
import Link from 'next/link';
import { useRegistryOperatorsCount } from '@/lib/hooks/useRegistryOperators';
import { useTLDsCount } from '@/lib/hooks/useTLDs';
import { useRegistrarCount } from '@/lib/hooks/useRegistrars';
import { useDomainCount } from '@/lib/hooks/useDomains';
import { useContactCount } from '@/lib/hooks/useContacts';
import { useHostCount } from '@/lib/hooks/useHosts';
import { format } from 'date-fns';
import { TEMPORAL_UI_URL, STORAGE_UI_URL } from '@/lib/constants/external-urls';
import { EventFeed } from '@/components/dashboard/EventFeed';
import { QuickActions } from '@/components/dashboard/QuickActions';
import { WelcomeToast } from '@/components/dashboard/WelcomeToast';
import { Skeleton } from '@/components/ui/skeleton';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 18) return 'Good afternoon';
  return 'Good evening';
}

function formatCount(n: number | undefined): string {
  if (n === undefined) return '—';
  return n.toLocaleString();
}

// ---------------------------------------------------------------------------
// Stat pill config
// ---------------------------------------------------------------------------

const STAT_PILLS = [
  {
    name: 'Registry Operators',
    short: 'ROs',
    icon: Building2,
    href: '/registry-operators',
    color: 'text-orange-600 dark:text-orange-400',
  },
  {
    name: 'TLDs',
    short: 'TLDs',
    icon: Globe,
    href: '/tlds',
    color: 'text-blue-600 dark:text-blue-400',
  },
  {
    name: 'Registrars',
    short: 'Registrars',
    icon: Users,
    href: '/registrars',
    color: 'text-emerald-600 dark:text-emerald-400',
  },
  {
    name: 'Domains',
    short: 'Domains',
    icon: Server,
    href: '/domains',
    color: 'text-violet-600 dark:text-violet-400',
  },
  {
    name: 'Contacts',
    short: 'Contacts',
    icon: Contact,
    href: '/contacts',
    color: 'text-amber-600 dark:text-amber-400',
  },
  {
    name: 'Hosts',
    short: 'Hosts',
    icon: HardDrive,
    href: '/hosts',
    color: 'text-rose-600 dark:text-rose-400',
  },
] as const;

// Resources (external links)
const resources = [
  {
    name: 'Temporal',
    href: TEMPORAL_UI_URL,
    icon: Activity,
  },
  {
    name: 'Storage',
    href: STORAGE_UI_URL,
    icon: Database,
  },
].filter((r) => r.href);

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function Home() {
  const { data: roCount, isLoading: loadingRO } = useRegistryOperatorsCount();
  const { data: tldCount, isLoading: loadingTLD } = useTLDsCount();
  const { data: registrarCount, isLoading: loadingRegistrar } = useRegistrarCount();
  const { data: domainCount, isLoading: loadingDomain } = useDomainCount();
  const { data: contactCount, isLoading: loadingContact } = useContactCount();
  const { data: hostCount, isLoading: loadingHost } = useHostCount();

  const counts = [
    roCount?.Count,
    tldCount?.Count,
    registrarCount?.Count,
    domainCount?.Count,
    contactCount?.Count,
    hostCount?.Count,
  ];

  const loadings = [
    loadingRO,
    loadingTLD,
    loadingRegistrar,
    loadingDomain,
    loadingContact,
    loadingHost,
  ];

  return (
    <DashboardLayout>
      <WelcomeToast />

      <div className="space-y-8">
        {/* ---------------------------------------------------------------- */}
        {/* Greeting — minimal, one line                                      */}
        {/* ---------------------------------------------------------------- */}
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {getGreeting()}
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            {format(new Date(), 'EEEE, MMMM do, yyyy')}
          </p>
        </div>

        {/* ---------------------------------------------------------------- */}
        {/* Stat pills — compact horizontal row                               */}
        {/* ---------------------------------------------------------------- */}
        <div className="flex flex-wrap gap-3">
          {STAT_PILLS.map((stat, i) => (
            <Link
              key={stat.name}
              href={stat.href}
              className="group flex items-center gap-2 rounded-full border bg-card px-4 py-2 transition-all duration-200 hover:border-primary/30 hover:shadow-sm"
              title={stat.name}
            >
              <stat.icon className={`h-3.5 w-3.5 ${stat.color}`} />
              <span className="text-xs font-medium text-muted-foreground">
                {stat.short}
              </span>
              {loadings[i] ? (
                <Skeleton className="h-4 w-8 rounded" />
              ) : (
                <span className="text-sm font-semibold tabular-nums">
                  {formatCount(counts[i])}
                </span>
              )}
            </Link>
          ))}
        </div>

        {/* ---------------------------------------------------------------- */}
        {/* Main content: Events feed + Quick Actions sidebar                 */}
        {/* ---------------------------------------------------------------- */}
        <div className="grid gap-6 lg:grid-cols-3">
          {/* Event feed — takes 2 cols */}
          <div className="lg:col-span-2">
            <EventFeed />
          </div>

          {/* Right sidebar — Quick Actions + Resources */}
          <div className="space-y-6">
            <QuickActions />

            {/* Resources — minimal */}
            {resources.length > 0 && (
              <div className="space-y-2">
                <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                  Resources
                </h3>
                <div className="space-y-1">
                  {resources.map((res) => (
                    <a
                      key={res.name}
                      href={res.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="group flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors hover:bg-accent/50"
                    >
                      <res.icon className="h-3.5 w-3.5 text-muted-foreground" />
                      <span className="flex-1 text-foreground/80">{res.name}</span>
                      <ExternalLink className="h-3 w-3 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
                    </a>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}
