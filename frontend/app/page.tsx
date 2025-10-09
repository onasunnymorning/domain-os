'use client';

import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Building2, Globe, Users, Server } from 'lucide-react';
import Link from 'next/link';
import { useRegistryOperatorsCount } from '@/lib/hooks/useRegistryOperators';
import { useTLDsCount } from '@/lib/hooks/useTLDs';

export default function Home() {
  const { data: countData, isLoading: isLoadingCount } = useRegistryOperatorsCount();
  const { data: tldCountData, isLoading: isLoadingTldCount } = useTLDsCount();

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
      value: '0',
      icon: Users,
      href: '/registrars',
      description: 'Domain registrars'
    },
    {
      name: 'Domains',
      value: '0',
      icon: Server,
      href: '/domains',
      description: 'Registered domains'
    },
  ];

  return (
    <DashboardLayout>
      <div className="space-y-8">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-muted-foreground">
            Welcome to Domain OS Registry Administration
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
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
              Get started by creating your first registry operator
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col gap-4">
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
