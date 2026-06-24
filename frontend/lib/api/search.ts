import { apiClient } from './client';
import type { DomainListItem } from '@/lib/types/domain';
import type { TLD } from './tlds';
import type { RegistrarListItem } from '@/lib/types/registrar';
import type { NNDN } from './nndns';
import type { RegistryOperator } from './types';
import type { WorkflowMeta } from './workflows';

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

function filterDocumentation(workflows: WorkflowMeta[], query: string): DocSearchResult[] {
  const q = query.toLowerCase();
  const sections = extractDocSections(workflows);
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
    registryOperatorsResult,
    workflowsResult,
    docsResult,
  ] = await Promise.allSettled([
    apiClient.get('/domains', { params: { name_like: query, pagesize } }),
    apiClient.get('/tlds', { params: { name_like: query, pagesize } }),
    apiClient.get('/registrars', { params: { name_like: query, pagesize } }),
    apiClient.get('/nndns', { params: { name_like: query, pagesize } }),
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
