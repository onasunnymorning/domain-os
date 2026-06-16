'use client';

import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Building2, Globe, Users, Server, Contact, HardDrive } from 'lucide-react';
import Link from 'next/link';
import { useRegistryOperatorsCount } from '@/lib/hooks/useRegistryOperators';
import { useTLDsCount } from '@/lib/hooks/useTLDs';
import { useRegistrarCount } from '@/lib/hooks/useRegistrars';
import { useDomainCount } from '@/lib/hooks/useDomains';
import { useContactCount } from '@/lib/hooks/useContacts';
import { useHostCount } from '@/lib/hooks/useHosts';

export default function Home() {
  const { data: countData, isLoading: isLoadingCount } = useRegistryOperatorsCount();
  const { data: tldCountData, isLoading: isLoadingTldCount } = useTLDsCount();
  const { data: registrarCountData, isLoading: isLoadingRegistrarCount } = useRegistrarCount();
  const { data: domainCountData, isLoading: isLoadingDomainCount } = useDomainCount();
  const { data: contactCountData, isLoading: isLoadingContactCount } = useContactCount();
  const { data: hostCountData, isLoading: isLoadingHostCount } = useHostCount();

  const stats = [
    {
      name: 'Registry Operators',
      value: isLoadingCount ? '...' : countData?.Count?.toString() ?? '0',
      icon: Building2,
      href: '/registry-operators',
      description: 'Manage registry operators'
    },
    {
      name: 'TLDs',
      value: isLoadingTldCount ? '...' : tldCountData?.Count?.toString() ?? '0',
      icon: Globe,
      href: '/tlds',
      description: 'Top-level domains'
    },
    {
      name: 'Registrars',
      value: isLoadingRegistrarCount ? '...' : registrarCountData?.Count?.toString() ?? '0',
      icon: Users,
      href: '/registrars',
      description: 'Domain registrars'
    },
    {
      name: 'Domains',
      value: isLoadingDomainCount ? '...' : domainCountData?.Count?.toString() ?? '0',
      icon: Server,
      href: '/domains',
      description: 'Registered domains'
    },
    {
      name: 'Contacts',
      value: isLoadingContactCount ? '...' : contactCountData?.Count?.toString() ?? '0',
      icon: Contact,
      href: '/contacts',
      description: 'Registered contacts'
    },
    {
      name: 'Hosts',
      value: isLoadingHostCount ? '...' : hostCountData?.Count?.toString() ?? '0',
      icon: HardDrive,
      href: '/hosts',
      description: 'Registered hosts'
    },
  ];

  return (
    <DashboardLayout>
      <div className="space-y-8">
        <div>
          <h1 className="text-4xl font-bold tracking-tight">
            Welcome to Alpaca Names
          </h1>
          <p className="text-muted-foreground">
            Registry Admin
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {stats.map((stat) => (
            <Link key={stat.name} href={stat.href}>
              <Card className="hover:bg-accent transition-colors cursor-pointer">
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">
                    {stat.name}
                  </CardTitle>
                  <stat.icon className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{stat.value}</div>
                  <p className="text-xs text-muted-foreground">
                    {stat.description}
                  </p>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Quick Actions</CardTitle>
            <CardDescription>
              Create records quickly
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col sm:flex-row gap-3">
              <Link
                href="/domains/create"
                className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
              >
                Create Domain
              </Link>
              <Link
                href="/tlds/create"
                className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
              >
                Create TLD
              </Link>
              <Link
                href="/registrars/create"
                className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
              >
                Create Registrar
              </Link>
              <Link
                href="/registry-operators/create"
                className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
              >
                Create Registry Operator
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
