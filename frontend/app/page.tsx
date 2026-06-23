'use client';

import { useMemo } from 'react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Building2,
  Globe,
  Users,
  Server,
  Contact,
  HardDrive,
  Plus,
  ArrowRight,
  ExternalLink,
  Clock,
  Database,
  BarChart,
} from 'lucide-react';
import Link from 'next/link';
import { useRegistryOperatorsCount } from '@/lib/hooks/useRegistryOperators';
import { useTLDsCount } from '@/lib/hooks/useTLDs';
import { useRegistrarCount } from '@/lib/hooks/useRegistrars';
import { useDomainCount } from '@/lib/hooks/useDomains';
import { useContactCount } from '@/lib/hooks/useContacts';
import { useHostCount } from '@/lib/hooks/useHosts';
import { format } from 'date-fns';
import { TEMPORAL_UI_URL, STORAGE_UI_URL, GRAFANA_URL } from '@/lib/constants/external-urls';
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
} from 'recharts';

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
  if (n === undefined) return '0';
  return n.toLocaleString();
}

// Colors for the stat card icon backgrounds (light / dark friendly via oklch)
const STAT_COLORS = [
  { bg: 'bg-orange-100 dark:bg-orange-950/40', text: 'text-orange-600 dark:text-orange-400' },
  { bg: 'bg-blue-100 dark:bg-blue-950/40', text: 'text-blue-600 dark:text-blue-400' },
  { bg: 'bg-emerald-100 dark:bg-emerald-950/40', text: 'text-emerald-600 dark:text-emerald-400' },
  { bg: 'bg-violet-100 dark:bg-violet-950/40', text: 'text-violet-600 dark:text-violet-400' },
  { bg: 'bg-amber-100 dark:bg-amber-950/40', text: 'text-amber-600 dark:text-amber-400' },
  { bg: 'bg-rose-100 dark:bg-rose-950/40', text: 'text-rose-600 dark:text-rose-400' },
];

// Chart segment fill colors (hex for recharts)
const CHART_COLORS = [
  '#ea580c', // orange-600
  '#2563eb', // blue-600
  '#059669', // emerald-600
  '#7c3aed', // violet-600
  '#d97706', // amber-600
  '#e11d48', // rose-600
];

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

  const isAnyLoading =
    loadingRO || loadingTLD || loadingRegistrar || loadingDomain || loadingContact || loadingHost;

  const stats = [
    {
      name: 'Registry Operators',
      value: roCount?.Count,
      loading: loadingRO,
      icon: Building2,
      href: '/registry-operators',
      description: 'Manage registry operators',
    },
    {
      name: 'TLDs',
      value: tldCount?.Count,
      loading: loadingTLD,
      icon: Globe,
      href: '/tlds',
      description: 'Top-level domains',
    },
    {
      name: 'Registrars',
      value: registrarCount?.Count,
      loading: loadingRegistrar,
      icon: Users,
      href: '/registrars',
      description: 'Domain registrars',
    },
    {
      name: 'Domains',
      value: domainCount?.Count,
      loading: loadingDomain,
      icon: Server,
      href: '/domains',
      description: 'Registered domains',
    },
    {
      name: 'Contacts',
      value: contactCount?.Count,
      loading: loadingContact,
      icon: Contact,
      href: '/contacts',
      description: 'Registered contacts',
    },
    {
      name: 'Hosts',
      value: hostCount?.Count,
      loading: loadingHost,
      icon: HardDrive,
      href: '/hosts',
      description: 'Registered hosts',
    },
  ];

  // Build chart data from the same counts (skip zeros for cleaner chart)
  const chartData = useMemo(() => {
    return stats
      .map((s) => ({ name: s.name, value: s.value ?? 0 }))
      .filter((d) => d.value > 0);
  }, [
    roCount?.Count,
    tldCount?.Count,
    registrarCount?.Count,
    domainCount?.Count,
    contactCount?.Count,
    hostCount?.Count,
  ]);

  const totalEntities = useMemo(
    () => chartData.reduce((sum, d) => sum + d.value, 0),
    [chartData],
  );

  const quickActions = [
    { label: 'Create Domain', href: '/domains/create' },
    { label: 'Create TLD', href: '/tlds/create' },
    { label: 'Create Registrar', href: '/registrars/create' },
    { label: 'Create Registry Operator', href: '/registry-operators/create' },
  ];

  const resources = [
    {
      name: 'Temporal UI',
      href: TEMPORAL_UI_URL,
      icon: Clock,
      description: 'Workflow orchestration',
    },
    {
      name: 'Object Storage',
      href: STORAGE_UI_URL,
      icon: Database,
      description: 'MinIO console',
    },
    {
      name: 'Analytics',
      href: GRAFANA_URL,
      icon: BarChart,
      description: 'Grafana dashboards',
    },
  ].filter((r) => r.href);

  return (
    <DashboardLayout>
      <div className="space-y-8">
        {/* ---------------------------------------------------------------- */}
        {/* Hero greeting                                                     */}
        {/* ---------------------------------------------------------------- */}
        <div className="relative overflow-hidden rounded-xl bg-gradient-to-br from-primary/10 via-primary/5 to-transparent border px-6 py-8 sm:px-8 sm:py-10">
          {/* Decorative blurred circle */}
          <div className="pointer-events-none absolute -right-16 -top-16 h-56 w-56 rounded-full bg-primary/10 blur-3xl" />
          <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">
            {getGreeting()} 👋
          </h1>
          <p className="mt-1 text-muted-foreground">
            {format(new Date(), 'EEEE, MMMM do, yyyy')} — Alpaca Names Registry Admin
          </p>
        </div>

        {/* ---------------------------------------------------------------- */}
        {/* Stat cards                                                        */}
        {/* ---------------------------------------------------------------- */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {stats.map((stat, i) => (
            <Link key={stat.name} href={stat.href}>
              <Card className="group relative hover:border-primary/40 hover:shadow-md transition-all duration-200 cursor-pointer">
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    {stat.name}
                  </CardTitle>
                  <div
                    className={`flex h-9 w-9 items-center justify-center rounded-lg ${STAT_COLORS[i].bg}`}
                  >
                    <stat.icon className={`h-4.5 w-4.5 ${STAT_COLORS[i].text}`} />
                  </div>
                </CardHeader>
                <CardContent>
                  {stat.loading ? (
                    <Skeleton className="h-8 w-20" />
                  ) : (
                    <div className="text-3xl font-bold tracking-tight">
                      {formatCount(stat.value)}
                    </div>
                  )}
                  <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                    {stat.description}
                    <ArrowRight className="h-3 w-3 opacity-0 -translate-x-1 transition-all group-hover:opacity-100 group-hover:translate-x-0" />
                  </p>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>

        {/* ---------------------------------------------------------------- */}
        {/* Middle row: Quick Actions + Entity Distribution Chart             */}
        {/* ---------------------------------------------------------------- */}
        <div className="grid gap-4 lg:grid-cols-5">
          {/* Quick Actions — takes 3 cols */}
          <Card className="lg:col-span-3">
            <CardHeader>
              <CardTitle>Quick Actions</CardTitle>
              <CardDescription>Create new records</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 sm:grid-cols-2">
                {quickActions.map((action) => (
                  <Button
                    key={action.href}
                    variant="outline"
                    className="justify-start gap-2 h-11"
                    asChild
                  >
                    <Link href={action.href}>
                      <Plus className="h-4 w-4 text-primary" />
                      {action.label}
                    </Link>
                  </Button>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Entity distribution chart — takes 2 cols */}
          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle>Entity Distribution</CardTitle>
              <CardDescription>
                {isAnyLoading
                  ? 'Loading…'
                  : `${totalEntities.toLocaleString()} total entities`}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isAnyLoading ? (
                <div className="flex items-center justify-center h-[180px]">
                  <Skeleton className="h-[140px] w-[140px] rounded-full" />
                </div>
              ) : chartData.length === 0 ? (
                <div className="flex items-center justify-center h-[180px] text-sm text-muted-foreground">
                  No entities yet
                </div>
              ) : (
                <div className="h-[180px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={chartData}
                        cx="50%"
                        cy="50%"
                        innerRadius={45}
                        outerRadius={75}
                        paddingAngle={3}
                        dataKey="value"
                        strokeWidth={0}
                      >
                        {chartData.map((entry, index) => (
                          <Cell
                            key={entry.name}
                            fill={CHART_COLORS[index % CHART_COLORS.length]}
                            className="transition-opacity hover:opacity-80"
                          />
                        ))}
                      </Pie>
                      <RechartsTooltip
                        content={({ active, payload }) => {
                          if (!active || !payload?.length) return null;
                          const item = payload[0];
                          return (
                            <div className="rounded-lg border bg-popover px-3 py-2 text-sm shadow-md">
                              <p className="font-medium">{item.name}</p>
                              <p className="text-muted-foreground">
                                {Number(item.value).toLocaleString()} entities
                              </p>
                            </div>
                          );
                        }}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                </div>
              )}

              {/* Legend */}
              {!isAnyLoading && chartData.length > 0 && (
                <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1.5">
                  {chartData.map((d, i) => (
                    <div key={d.name} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                      <span
                        className="inline-block h-2.5 w-2.5 rounded-sm shrink-0"
                        style={{ backgroundColor: CHART_COLORS[i % CHART_COLORS.length] }}
                      />
                      {d.name}
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* ---------------------------------------------------------------- */}
        {/* Resources                                                         */}
        {/* ---------------------------------------------------------------- */}
        {resources.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Resources</CardTitle>
              <CardDescription>External tools &amp; services</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {resources.map((res) => (
                  <a
                    key={res.name}
                    href={res.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="group flex items-center gap-3 rounded-lg border bg-card px-4 py-3 transition-colors hover:bg-accent"
                  >
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                      <res.icon className="h-4 w-4 text-muted-foreground" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium leading-tight">{res.name}</p>
                      <p className="text-xs text-muted-foreground">{res.description}</p>
                    </div>
                    <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                  </a>
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </DashboardLayout>
  );
}
