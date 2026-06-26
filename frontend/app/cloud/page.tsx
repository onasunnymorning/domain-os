'use client';

import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Cloud,
  Database,
  Activity,
  Shield,
  HardDrive,
  Rocket,
  ExternalLink,
  KeyRound,
  Github,
} from 'lucide-react';

const services = [
  {
    name: 'GitHub',
    description: 'Source code repository and CI/CD',
    icon: Github,
    url: 'https://github.com/onasunnymorning/domain-os',
    color: 'text-gray-400',
    bgColor: 'bg-gray-400/10',
    items: ['domain-os', 'Pull requests', 'Actions'],
  },
  {
    name: 'Render',
    description: 'Compute platform — API, frontend, and worker services',
    icon: Rocket,
    url: 'https://dashboard.render.com/',
    color: 'text-emerald-500',
    bgColor: 'bg-emerald-500/10',
    items: ['alp-api-test', 'alp-ui-test', 'alp-worker-test'],
  },
  {
    name: 'Neon',
    description: 'Serverless Postgres — database hosting',
    icon: Database,
    url: 'https://console.neon.tech/',
    color: 'text-green-400',
    bgColor: 'bg-green-400/10',
    items: ['alpaca project', 'test branch'],
  },
  {
    name: 'Temporal Cloud',
    description: 'Workflow orchestration — background jobs and pipelines',
    icon: Activity,
    url: process.env.NEXT_PUBLIC_TEMPORAL_UI_URL || 'https://cloud.temporal.io/',
    color: 'text-indigo-400',
    bgColor: 'bg-indigo-400/10',
    items: ['domain-lifecycle', 'escrow-import', 'sync'],
  },
  {
    name: 'Auth0',
    description: 'Identity platform — authentication and user management',
    icon: Shield,
    url: 'https://manage.auth0.com/',
    color: 'text-orange-400',
    bgColor: 'bg-orange-400/10',
    items: ['Social login', 'M2M tokens', 'User management'],
  },
  {
    name: 'Doppler',
    description: 'Secrets management — environment variables and credentials',
    icon: KeyRound,
    url: 'https://dashboard.doppler.com/',
    color: 'text-violet-400',
    bgColor: 'bg-violet-400/10',
    items: ['dev', 'test', 'production'],
  },
  {
    name: 'Backblaze B2',
    description: 'Object storage — escrow files and backups',
    icon: HardDrive,
    url: process.env.NEXT_PUBLIC_STORAGE_UI_URL || 'https://secure.backblaze.com/b2_buckets.htm',
    color: 'text-red-400',
    bgColor: 'bg-red-400/10',
    items: ['Escrow deposits', 'TLD backups'],
  },
];

export default function CloudPage() {
  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
            <Cloud className="h-8 w-8" />
            Cloud Infrastructure
          </h1>
          <p className="text-muted-foreground mt-2">
            External services powering the AlpacaNames TEST environment
          </p>
        </div>

        {/* Service Cards */}
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {services.map((service) => (
            <a
              key={service.name}
              href={service.url}
              target="_blank"
              rel="noopener noreferrer"
              className="group block"
            >
              <Card className="h-full transition-all duration-200 hover:shadow-lg hover:border-primary/50 group-hover:-translate-y-0.5">
                <CardHeader className="pb-3">
                  <div className="flex items-center justify-between">
                    <div className={`rounded-lg p-2.5 ${service.bgColor}`}>
                      <service.icon className={`h-5 w-5 ${service.color}`} />
                    </div>
                    <ExternalLink className="h-4 w-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                  </div>
                  <CardTitle className="mt-3 text-lg">{service.name}</CardTitle>
                  <CardDescription>{service.description}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex flex-wrap gap-1.5">
                    {service.items.map((item) => (
                      <span
                        key={item}
                        className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                      >
                        {item}
                      </span>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </a>
          ))}
        </div>

        {/* Environment info */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Environment: TEST</CardTitle>
            <CardDescription>
              Pre-IaC deployment — services are managed manually via their respective dashboards.
              This page serves as a quick-access hub until infrastructure-as-code is in place.
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    </DashboardLayout>
  );
}
