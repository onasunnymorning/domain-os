import { apiClient } from './client';
import type { DomainListItem } from '@/lib/types/domain';
import type { TLD } from './tlds';
import type { RegistrarListItem } from '@/lib/types/registrar';
import type { NNDN } from './nndns';
import type { RegistryOperator } from './types';
import type { WorkflowMeta } from './workflows';
import type { DomainTombstone } from './tombstones';

export interface DocSearchResult {
  workflowKey: string;
  workflowName: string;
  heading: string;
  snippet: string;
}

export interface SearchResults {
  domains: DomainListItem[];
  tlds: TLD[];
  registrars: RegistrarListItem[];
  nndns: NNDN[];
  tombstones: DomainTombstone[];
  registryOperators: RegistryOperator[];
  workflows: WorkflowMeta[];
  documentation: DocSearchResult[];
}

// Cache the workflow registry since it's small and static
let workflowRegistryCache: WorkflowMeta[] | null = null;

async function getWorkflowRegistryForSearch(): Promise<WorkflowMeta[]> {
  if (workflowRegistryCache) return workflowRegistryCache;
  try {
    const { data } = await apiClient.get('/workflows/registry');
    workflowRegistryCache = data?.items ?? [];
    return workflowRegistryCache!;
  } catch {
    return [];
  }
}

function filterWorkflows(workflows: WorkflowMeta[], query: string): WorkflowMeta[] {
  const q = query.toLowerCase();
  return workflows.filter(
    (wf) =>
      wf.name.toLowerCase().includes(q) ||
      wf.description.toLowerCase().includes(q) ||
      wf.key.toLowerCase().includes(q) ||
      wf.tags.some((tag) => tag.toLowerCase().includes(q)) ||
      wf.category.toLowerCase().includes(q)
  ).slice(0, 5);
}

/**
 * Extract searchable sections from workflow doc markdown.
 * Splits on headings and pairs each heading with a content snippet.
 */
function extractDocSections(workflows: WorkflowMeta[]): DocSearchResult[] {
  const sections: DocSearchResult[] = [];

  for (const wf of workflows) {
    if (!wf.docMarkdown) continue;

    const lines = wf.docMarkdown.split('\n');
    let currentHeading = '';
    let currentContent: string[] = [];

    const flush = () => {
      if (currentHeading && currentContent.length > 0) {
        // Take first ~150 chars of content as snippet
        const text = currentContent
          .join(' ')
          .replace(/[#*`|\-_>]/g, '')
          .replace(/\s+/g, ' ')
          .trim();
        if (text) {
          sections.push({
            workflowKey: wf.key,
            workflowName: wf.name,
            heading: currentHeading,
            snippet: text.length > 150 ? text.slice(0, 147) + '...' : text,
          });
        }
      }
    };

    for (const line of lines) {
      const headingMatch = line.match(/^#{1,3}\s+(.+)/);
      if (headingMatch) {
        flush();
        currentHeading = headingMatch[1].trim();
        currentContent = [];
      } else if (line.trim() && !line.startsWith('```')) {
        currentContent.push(line.trim());
      }
    }
    flush();
  }

  return sections;
}

import { CONTACT_DATA_POLICY_DOC_MARKDOWN } from '../constants/contactDataPolicyDoc';
import { DATABASE_INDEX_STRATEGY_DOC_MARKDOWN } from '../constants/databaseIndexStrategyDoc';
import { DOMAIN_ARCHIVAL_DOC_MARKDOWN } from '../constants/domainArchivalDoc';
import { EVENT_CONSUMER_DOC_MARKDOWN } from '../constants/eventConsumerDoc';
import { POSTHOG_ANALYTICS_DOC_MARKDOWN } from '../constants/posthogAnalyticsDoc';
import { WORKER_QUEUE_ARCHITECTURE_DOC_MARKDOWN } from '../constants/workerQueueArchitectureDoc';

const STATIC_DOCS: WorkflowMeta[] = [
  {
    key: 'contact-data-policy',
    name: 'Contact Data Policy',
    description: 'Configuration, enforcement rules, and compliance standards for domain contacts',
    queue: '',
    category: 'policy',
    tags: ['policy', 'contact', 'registrant', 'tech', 'admin', 'billing'],
    hasSignal: false,
    scheduled: false,
    steps: [],
    docMarkdown: CONTACT_DATA_POLICY_DOC_MARKDOWN,
  },
  {
    key: 'database-index-strategy',
    name: 'Database Index Strategy',
    description: 'PostgreSQL indexing for 80M+ domain scale, storage budgets, query optimization, and event lifecycle',
    queue: '',
    category: 'infrastructure',
    tags: ['database', 'postgresql', 'index', 'performance', 'scale', 'domains', 'events', 'optimization'],
    hasSignal: false,
    scheduled: false,
    steps: [],
    docMarkdown: DATABASE_INDEX_STRATEGY_DOC_MARKDOWN,
  },
  {
    key: 'event-consumer',
    name: 'Event Consumer Cloud',
    description: 'Tiered event lifecycle with automated relay to S3, pruning, and long-term archival',
    queue: '',
    category: 'infrastructure',
    tags: ['events', 'consumer', 'archive', 's3', 'minio', 'temporal', 'prune', 'relay', 'lifecycle'],
    hasSignal: false,
    scheduled: false,
    steps: [],
    docMarkdown: EVENT_CONSUMER_DOC_MARKDOWN,
  },
  {
    key: 'posthog-analytics',
    name: 'PostHog Analytics',
    description: 'Event tracking, session recordings, error capture, and behavioral analytics for the registry UI',
    queue: '',
    category: 'infrastructure',
    tags: ['posthog', 'analytics', 'tracking', 'events', 'session', 'recording', 'autocapture', 'error'],
    hasSignal: false,
    scheduled: false,
    steps: [],
    docMarkdown: POSTHOG_ANALYTICS_DOC_MARKDOWN,
  },
  {
    key: 'domain-archival',
    name: 'Domain Archival',
    description: 'Tombstone architecture, ROID-based linking, and lifecycle history for purged domains',
    queue: '',
    category: 'infrastructure',
    tags: ['domain', 'archival', 'tombstone', 'roid', 'purge', 'lifecycle', 'history', 'archive'],
    hasSignal: false,
    scheduled: false,
    steps: [],
    docMarkdown: DOMAIN_ARCHIVAL_DOC_MARKDOWN,
  },
  {
    key: 'worker-queue-architecture',
    name: 'Worker Queue Architecture',
    description: 'Queue taxonomy, Temporal worker configuration, poller tuning, and deployment topology',
    queue: '',
    category: 'infrastructure',
    tags: ['temporal', 'worker', 'queue', 'poller', 'architecture', 'scaling', 'tuning', 'deployment'],
    hasSignal: false,
    scheduled: false,
    steps: [],
    docMarkdown: WORKER_QUEUE_ARCHITECTURE_DOC_MARKDOWN,
  },
];

function filterDocumentation(workflows: WorkflowMeta[], query: string): DocSearchResult[] {
  const q = query.toLowerCase();
  const allDocs = [...workflows, ...STATIC_DOCS];
  const sections = extractDocSections(allDocs);
  return sections
    .filter(
      (s) =>
        s.heading.toLowerCase().includes(q) ||
        s.snippet.toLowerCase().includes(q) ||
        s.workflowName.toLowerCase().includes(q)
    )
    .slice(0, 5);
}

/**
 * Search across all entity types in parallel.
 * Uses the existing `_like` filter params on each list endpoint,
 * capped at 5 results per category for speed.
 *
 * Uses Promise.allSettled so a failure in one category
 * doesn't block results from others.
 */
export async function searchAll(query: string): Promise<SearchResults> {
  const pagesize = 5;

  const [
    domainsResult,
    tldsResult,
    registrarsResult,
    nndnsResult,
    tombstonesResult,
    registryOperatorsResult,
    workflowsResult,
    docsResult,
  ] = await Promise.allSettled([
    apiClient.get('/domains', { params: { name_like: query, pagesize } }),
    apiClient.get('/tlds', { params: { name_like: query, pagesize } }),
    apiClient.get('/registrars', { params: { name_like: query, pagesize } }),
    apiClient.get('/nndns', { params: { name_like: query, pagesize } }),
    apiClient.get('/tombstones', { params: { name_like: query, pagesize } }),
    apiClient.get('/registry-operators', { params: { name_like: query, pagesize } }),
    getWorkflowRegistryForSearch().then((all) => filterWorkflows(all, query)),
    getWorkflowRegistryForSearch().then((all) => filterDocumentation(all, query)),
  ]);

  return {
    domains:
      domainsResult.status === 'fulfilled'
        ? (domainsResult.value.data?.Data ?? [])
        : [],
    tlds:
      tldsResult.status === 'fulfilled'
        ? (tldsResult.value.data?.Data ?? [])
        : [],
    registrars:
      registrarsResult.status === 'fulfilled'
        ? (registrarsResult.value.data?.Data ?? [])
        : [],
    nndns:
      nndnsResult.status === 'fulfilled'
        ? (nndnsResult.value.data?.Data ?? [])
        : [],
    tombstones:
      tombstonesResult.status === 'fulfilled'
        ? (tombstonesResult.value.data?.Data ?? [])
        : [],
    registryOperators:
      registryOperatorsResult.status === 'fulfilled'
        ? (registryOperatorsResult.value.data?.Data ?? [])
        : [],
    workflows:
      workflowsResult.status === 'fulfilled'
        ? workflowsResult.value
        : [],
    documentation:
      docsResult.status === 'fulfilled'
        ? docsResult.value
        : [],
  };
}
