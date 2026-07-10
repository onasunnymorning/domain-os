'use client';

import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { getTemporalUiUrl, getStorageUiUrl } from '@/lib/env';
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
  Variable,
  BarChart3,
} from 'lucide-react';

const getServices = () => [
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
    url: getTemporalUiUrl() || 'https://cloud.temporal.io/',
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
    name: 'Cloudflare R2',
    description: 'Object storage — escrow files and backups',
    icon: HardDrive,
    url: getStorageUiUrl() || 'https://dash.cloudflare.com/',
    color: 'text-amber-400',
    bgColor: 'bg-amber-400/10',
    items: ['Escrow deposits', 'TLD backups'],
  },
  {
    name: 'PostHog',
    description: 'Product analytics — event tracking, session recordings, error capture',
    icon: BarChart3,
    url: 'https://us.posthog.com/project/487655',
    color: 'text-rose-400',
    bgColor: 'bg-rose-400/10',
    items: ['Autocapture', 'Session recordings', 'Custom events', 'Error tracking'],
  },
];

type EnvVar = {
  key: string;
  description: string;
  secret: boolean;
  services: string[];
};

const envVarsByCategory: { category: string; icon: React.ElementType; color: string; vars: EnvVar[] }[] = [
  {
    category: 'Database (Neon)',
    icon: Database,
    color: 'text-green-400',
    vars: [
      { key: 'DATABASE_URL', description: 'Neon Postgres connection string', secret: true, services: ['API', 'Worker'] },
      { key: 'AUTO_MIGRATE', description: 'Run GORM auto-migration on startup', secret: false, services: ['API'] },
    ],
  },
  {
    category: 'API Server',
    icon: Rocket,
    color: 'text-emerald-500',
    vars: [
      { key: 'API_PORT', description: 'HTTP listen port', secret: false, services: ['API'] },
      { key: 'API_HOST', description: 'HTTP bind address', secret: false, services: ['API'] },
      { key: 'API_NAME', description: 'API display name', secret: false, services: ['API'] },
      { key: 'API_VERSION', description: 'API version string', secret: false, services: ['API'] },
      { key: 'API_URL', description: 'Full API base URL (worker → API calls)', secret: false, services: ['Worker'] },
      { key: 'GIN_MODE', description: 'Gin framework mode (debug/release)', secret: false, services: ['API'] },
      { key: 'CORS_ALLOWED_ORIGINS', description: 'Allowed CORS origins (frontend URL)', secret: false, services: ['API'] },
    ],
  },
  {
    category: 'Auth0',
    icon: Shield,
    color: 'text-orange-400',
    vars: [
      { key: 'AUTH0_ENABLED', description: 'Enable Auth0 JWT validation', secret: false, services: ['API', 'Worker'] },
      { key: 'AUTH0_DOMAIN', description: 'Auth0 tenant domain', secret: false, services: ['API', 'Worker'] },
      { key: 'AUTH0_AUDIENCE', description: 'Auth0 API audience identifier', secret: false, services: ['API', 'Worker'] },
      { key: 'AUTH0_WORKER_CLIENT_ID', description: 'M2M application client ID (worker → API)', secret: true, services: ['Worker'] },
      { key: 'AUTH0_WORKER_CLIENT_SECRET', description: 'M2M application client secret', secret: true, services: ['Worker'] },
      { key: 'ADMIN_TOKEN', description: 'Static admin token fallback (when Auth0 disabled)', secret: true, services: ['API'] },
      { key: 'NEXT_PUBLIC_AUTH0_ENABLED', description: 'Enable Auth0 in the frontend', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_AUTH0_DOMAIN', description: 'Auth0 tenant domain (frontend)', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_AUTH0_CLIENT_ID', description: 'Auth0 SPA client ID', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_AUTH0_AUDIENCE', description: 'Auth0 API audience (frontend)', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_API_TOKEN', description: 'Static API token fallback — served to every browser, so it must be low-privilege', secret: false, services: ['Frontend'] },
    ],
  },
  {
    category: 'Temporal Cloud',
    icon: Activity,
    color: 'text-indigo-400',
    vars: [
      { key: 'TEMPORAL_HOST_PORT', description: 'Temporal Cloud gRPC endpoint', secret: false, services: ['API', 'Worker'] },
      { key: 'TEMPORAL_NAMESPACE', description: 'Temporal namespace (with account suffix)', secret: false, services: ['API', 'Worker'] },
      { key: 'TEMPORAL_API_KEY', description: 'Temporal Cloud API key', secret: true, services: ['API', 'Worker'] },
      { key: 'TEMPORAL_CLIENT_CERT', description: 'mTLS client certificate (PEM)', secret: true, services: ['API', 'Worker'] },
      { key: 'TEMPORAL_CLIENT_KEY', description: 'mTLS private key (PEM)', secret: true, services: ['API', 'Worker'] },
    ],
  },
  {
    category: 'Storage (R2)',
    icon: HardDrive,
    color: 'text-amber-400',
    vars: [
      { key: 'MINIO_ENDPOINT', description: 'S3-compatible endpoint (hostname only, no https://)', secret: false, services: ['API', 'Worker'] },
      { key: 'MINIO_ACCESS_KEY', description: 'S3 access key ID', secret: true, services: ['API', 'Worker'] },
      { key: 'MINIO_SECRET_KEY', description: 'S3 secret access key', secret: true, services: ['API', 'Worker'] },
      { key: 'MINIO_USE_SSL', description: 'Enable TLS for S3 connections', secret: false, services: ['API', 'Worker'] },
      { key: 'MINIO_PUBLIC_ENDPOINT', description: 'Public presign endpoint (if different)', secret: false, services: ['API'] },
      { key: 'STORAGE_ESCROW_BUCKET', description: 'Bucket for RDE/BRDA escrow deposits (PII)', secret: false, services: ['API', 'Worker'] },
      { key: 'STORAGE_EVENT_LOGS_BUCKET', description: 'Bucket for gzip JSONL event archives', secret: false, services: ['API', 'Worker'] },
      { key: 'STORAGE_REPORTS_BUCKET', description: 'Bucket for Spec 5 compliance sweep reports', secret: false, services: ['API', 'Worker'] },
      { key: 'STORAGE_TEMP_BUCKET', description: 'Bucket for workflow-scoped staging artifacts', secret: false, services: ['API', 'Worker'] },
    ],
  },
  {
    category: 'Frontend',
    icon: Rocket,
    color: 'text-emerald-500',
    vars: [
      { key: 'NEXT_PUBLIC_API_URL', description: 'API base URL (read at container start)', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_APP_VERSION', description: 'Displayed app version', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_TEMPORAL_UI_URL', description: 'Link to Temporal Cloud UI', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_TEMPORAL_NAMESPACE', description: 'Namespace used for schedule deep-links', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_STORAGE_UI_URL', description: 'Link to storage dashboard', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_GRAFANA_URL', description: 'Link to Grafana dashboard', secret: false, services: ['Frontend'] },
    ],
  },
  {
    category: 'Observability',
    icon: Activity,
    color: 'text-indigo-400',
    vars: [
      { key: 'NEW_RELIC_ENABLED', description: 'Enable New Relic APM', secret: false, services: ['API'] },
      { key: 'PROMETHEUS_ENABLED', description: 'Enable Prometheus metrics endpoint', secret: false, services: ['API'] },
    ],
  },
  {
    category: 'Analytics (PostHog)',
    icon: BarChart3,
    color: 'text-rose-400',
    vars: [
      { key: 'NEXT_PUBLIC_POSTHOG_PROJECT_TOKEN', description: 'PostHog project API key (write-only ingest key)', secret: false, services: ['Frontend'] },
      { key: 'NEXT_PUBLIC_POSTHOG_HOST', description: 'PostHog ingest endpoint (proxied via /ingest)', secret: false, services: ['Frontend'] },
    ],
  },
  {
    category: 'AI / Agent',
    icon: Activity,
    color: 'text-violet-400',
    vars: [
      { key: 'ANTHROPIC_API_KEY', description: 'Anthropic API key — enables Agent Alpaca', secret: true, services: ['API'] },
      { key: 'ANTHROPIC_BASE_URL', description: 'Optional Anthropic API base URL override', secret: false, services: ['API'] },
      { key: 'ASKG_MODEL', description: 'Claude model for tool-use reasoning (default: claude-sonnet-4-6)', secret: false, services: ['API'] },
      { key: 'ASKG_CLASSIFIER_MODEL', description: 'Cheaper model for intent classification (future)', secret: false, services: ['API'] },
      { key: 'MCP_TRANSPORT', description: 'MCP server transport: stdio or http (default: stdio)', secret: false, services: ['MCP'] },
      { key: 'MCP_PORT', description: 'MCP HTTP listen port (default: 3001)', secret: false, services: ['MCP'] },
    ],
  },
];

const serviceColors: Record<string, string> = {
  API: 'bg-emerald-500/15 text-emerald-400',
  Frontend: 'bg-blue-500/15 text-blue-400',
  Worker: 'bg-amber-500/15 text-amber-400',
  MCP: 'bg-violet-500/15 text-violet-400',
};

export default function CloudPage() {
  const services = getServices();

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

        {/* Environment Variables Reference */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Variable className="h-5 w-5" />
              Environment Variables Reference
            </CardTitle>
            <CardDescription>
              Complete list of environment variables across all services. Variables marked with 🔒 contain secrets and should be set via Doppler or Render&apos;s secret management.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Tabs defaultValue={envVarsByCategory[0].category} className="w-full">
              <TabsList className="flex flex-wrap h-auto gap-1 bg-transparent p-0 mb-4">
                {envVarsByCategory.map((cat) => (
                  <TabsTrigger
                    key={cat.category}
                    value={cat.category}
                    className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground rounded-md px-3 py-1.5 text-xs"
                  >
                    <cat.icon className={`h-3.5 w-3.5 mr-1.5 ${cat.color}`} />
                    {cat.category}
                  </TabsTrigger>
                ))}
              </TabsList>
              {envVarsByCategory.map((cat) => (
                <TabsContent key={cat.category} value={cat.category}>
                  <div className="rounded-lg border">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b bg-muted/50">
                          <th className="text-left font-medium p-3">Variable</th>
                          <th className="text-left font-medium p-3 hidden md:table-cell">Description</th>
                          <th className="text-left font-medium p-3">Used by</th>
                        </tr>
                      </thead>
                      <tbody>
                        {cat.vars.map((v, i) => (
                          <tr key={v.key} className={i < cat.vars.length - 1 ? 'border-b' : ''}>
                            <td className="p-3 font-mono text-xs">
                              {v.secret && <span title="Secret — do not hardcode">🔒 </span>}
                              {v.key}
                            </td>
                            <td className="p-3 text-muted-foreground hidden md:table-cell">{v.description}</td>
                            <td className="p-3">
                              <div className="flex flex-wrap gap-1">
                                {v.services.map((s) => (
                                  <span
                                    key={s}
                                    className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium ${serviceColors[s] || 'bg-muted text-muted-foreground'}`}
                                  >
                                    {s}
                                  </span>
                                ))}
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </TabsContent>
              ))}
            </Tabs>
          </CardContent>
        </Card>

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
