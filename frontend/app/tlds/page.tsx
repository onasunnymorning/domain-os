'use client';

import { Suspense, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useTLDs, useDeleteTLD } from '@/lib/hooks/useTLDs';
import { useDomainCount } from '@/lib/hooks/useDomains';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { formatCompactNumber } from '@/lib/utils/numberUtils';
import { TLDActivePhases } from '@/components/tlds/TLDActivePhases';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { 
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Globe, Plus, Trash2, X } from 'lucide-react';
import { useDebounce } from '@/lib/hooks/useDebounce';

import { ListPageLayout } from '@/components/shared/ListPageLayout';
import { DataTable, ColumnDef } from '@/components/shared/DataTable';
import { SearchFilter } from '@/components/shared/SearchFilter';
import { DeleteConfirmDialog } from '@/components/shared/DeleteConfirmDialog';

export default function TLDsPage() {
  return (
    <Suspense fallback={<div />}> 
      <TLDsPageInner />
    </Suspense>
  );
}

// Lightweight, per-row async cell for domain count
function DomainCountCell({ tldName }: { tldName: string }) {
  const { data, isLoading, isError } = useDomainCount({ tld_equals: tldName });
  if (isLoading) return <Skeleton className="h-4 w-10 inline-block" />;
  if (isError) return <span className="text-muted-foreground">—</span>;
  const count = data?.Count;
  const formattedTime = data?.Timestamp ? `As of ${new Date(data.Timestamp).toLocaleString()}` : '';
  const exactCount = typeof count === 'number' ? count.toLocaleString() : '';
  const titleText = [exactCount, formattedTime].filter(Boolean).join(' | ');

  return (
    <span className="cursor-help border-b border-dotted border-muted-foreground/50 pb-0.5" title={titleText || undefined}>
      {typeof count === 'number' ? formatCompactNumber(count) : '—'}
    </span>
  );
}

function TLDsPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [searchTerm, setSearchTerm] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [ryidFilter, setRyidFilter] = useState<string>(searchParams.get('ryid_equals') || '');
  const [tldToDelete, setTldToDelete] = useState<string | null>(null);
  
  const debouncedSearch = useDebounce(searchTerm, 300);
  
  // Load ALL TLDs to determine which operators have TLDs
  const { data: allTLDsData } = useTLDs({ pagesize: 1000 });
  const { data: operatorsData } = useRegistryOperators({ pagesize: 100 });
  
  const operatorsWithTLDs = operatorsData?.Data?.filter(op => {
    return allTLDsData?.Data?.some(tld => tld.RyID === op.RyID);
  }) || [];
  
  const { data, isLoading } = useTLDs({
    name_like: debouncedSearch || undefined,
    type_equals: typeFilter ? (typeFilter as any) : undefined,
    ryid_equals: ryidFilter || undefined,
  });
  
  const { mutate: deleteTLD, isPending: isDeleting } = useDeleteTLD();

  const confirmDelete = () => {
    if (tldToDelete) {
      deleteTLD({ name: tldToDelete, keepTLDAndPhases: false } as any);
      setTldToDelete(null); // Dialog will implicitly close since tldToDelete becomes null
    }
  };

  const getTypeBadgeVariant = (type: string) => {
    switch (type) {
      case 'generic': return 'default';
      case 'country-code': return 'secondary';
      case 'second-level': return 'outline';
      default: return 'default';
    }
  };

  const tlds = data?.Data || [];

  const columns: ColumnDef<any>[] = [
    {
      header: 'Name',
      accessor: 'Name',
      className: 'font-medium',
      cell: (tld) => <span title={tld.UName || undefined}>{tld.Name}</span>
    },
    {
      header: 'Type',
      cell: (tld) => (
        <Badge variant={getTypeBadgeVariant(tld.Type)}>
          {tld.Type === 'generic' && 'gTLD'}
          {tld.Type === 'country-code' && 'ccTLD'}
          {tld.Type === 'second-level' && 'SLD'}
        </Badge>
      )
    },
    {
      header: 'Domains',
      cell: (tld) => <DomainCountCell tldName={tld.Name} />
    },
    {
      header: 'Registry Operator',
      accessor: 'RyID'
    },
    {
      header: 'Accredited Registrars',
      cell: (tld) => <span>{tld.RegistrarCount ?? 0}</span>
    },
    {
      header: 'DNS',
      cell: (tld) => tld.EnableDNS ? (
        <Badge variant="secondary" className="bg-green-100 text-green-800">Enabled</Badge>
      ) : (
        <Badge variant="outline">Disabled</Badge>
      )
    },
    {
      header: 'Escrow Import',
      cell: (tld) => tld.AllowEscrowImport ? (
        <Badge variant="secondary" className="bg-green-100 text-green-800">Enabled</Badge>
      ) : (
        <Badge variant="outline">Disabled</Badge>
      )
    },
    {
      header: 'Active Phases',
      cell: (tld) => (
        <div onClick={(e) => e.stopPropagation()}>
          <TLDActivePhases tldName={tld.Name} />
        </div>
      )
    },
    {
      header: 'Actions',
      className: 'text-right',
      cell: (tld) => (
        <div className="flex justify-end gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              setTldToDelete(tld.Name);
            }}
            disabled={isDeleting}
          >
            <Trash2 className="h-4 w-4 text-destructive" />
            <span className="sr-only">Delete</span>
          </Button>
        </div>
      )
    }
  ];

  const filters = (
    <>
      <div className="grid gap-4 md:grid-cols-3">
        <SearchFilter 
          value={searchTerm} 
          onChange={setSearchTerm} 
          placeholder="Search by name..." 
          className="relative"
        />
        <Select value={typeFilter} onValueChange={setTypeFilter}>
          <SelectTrigger>
            <SelectValue placeholder="Filter by type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Types</SelectItem>
            <SelectItem value="generic">Generic (gTLD)</SelectItem>
            <SelectItem value="country-code">Country Code (ccTLD)</SelectItem>
            <SelectItem value="second-level">Second Level (SLD)</SelectItem>
          </SelectContent>
        </Select>
        <Select value={ryidFilter} onValueChange={setRyidFilter}>
          <SelectTrigger>
            <SelectValue placeholder="Filter by operator" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Operators</SelectItem>
            {operatorsWithTLDs.length > 0 ? (
              operatorsWithTLDs.map((op) => {
                const tldCount = allTLDsData?.Data?.filter(tld => tld.RyID === op.RyID).length || 0;
                return (
                  <SelectItem key={op.RyID} value={op.RyID}>
                    {op.Name} ({op.RyID}) • {tldCount} TLD{tldCount !== 1 ? 's' : ''}
                  </SelectItem>
                );
              })
            ) : (
              <SelectItem value="none" disabled>No operators with TLDs</SelectItem>
            )}
          </SelectContent>
        </Select>
      </div>
      {(searchTerm || typeFilter || ryidFilter) && (
        <div className="flex items-center gap-2 mt-3">
          {typeFilter && typeFilter !== 'all' && (
            <Badge variant="secondary" className="gap-1.5 pr-1">
              Type: {typeFilter === 'generic' ? 'gTLD' : 
                     typeFilter === 'country-code' ? 'ccTLD' : 
                     typeFilter === 'second-level' ? 'SLD' : typeFilter}
              <button
                type="button"
                className="ml-1 rounded-sm hover:bg-destructive/20 p-0.5"
                onClick={(e) => { e.preventDefault(); e.stopPropagation(); setTypeFilter(''); }}
              >
                <X className="h-3 w-3" />
              </button>
            </Badge>
          )}
          {ryidFilter && ryidFilter !== 'all' && (
            <Badge variant="secondary" className="gap-1.5 pr-1">
              Operator: {operatorsWithTLDs.find(op => op.RyID === ryidFilter)?.Name || 
                         operatorsData?.Data?.find(op => op.RyID === ryidFilter)?.Name || 
                         ryidFilter}
              <button
                type="button"
                className="ml-1 rounded-sm hover:bg-destructive/20 p-0.5"
                onClick={(e) => { e.preventDefault(); e.stopPropagation(); setRyidFilter(''); }}
              >
                <X className="h-3 w-3" />
              </button>
            </Badge>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { setSearchTerm(''); setTypeFilter(''); setRyidFilter(''); }}
          >
            Clear All Filters
          </Button>
        </div>
      )}
    </>
  );

  return (
    <ListPageLayout
      icon={Globe}
      title="TLDs"
      description="Manage top-level domains in the registry"
      actionButton={
        <Button onClick={() => router.push('/tlds/create')}>
          <Plus className="mr-2 h-4 w-4" />
          Create TLD
        </Button>
      }
      filters={filters}
    >
      <DataTable
        title="TLD List"
        description={isLoading ? 'Loading...' : `${tlds.length} TLD${tlds.length !== 1 ? 's' : ''} found`}
        columns={columns}
        data={tlds}
        keyExtractor={(row) => row.Name}
        isLoading={isLoading}
        onRowClick={(row) => router.push(`/tlds/${row.Name}`)}
        emptyState={
          <div className="text-center py-12">
            <Globe className="mx-auto h-12 w-12 text-muted-foreground" />
            <h3 className="mt-4 text-lg font-semibold">No TLDs found</h3>
            <p className="text-muted-foreground mt-2">
              {searchTerm || typeFilter || ryidFilter 
                ? 'Try adjusting your filters' 
                : 'Get started by creating your first TLD'}
            </p>
            {!searchTerm && !typeFilter && !ryidFilter && (
              <Button onClick={() => router.push('/tlds/create')} className="mt-4">
                <Plus className="mr-2 h-4 w-4" />
                Create TLD
              </Button>
            )}
          </div>
        }
      />

      <DeleteConfirmDialog
        open={!!tldToDelete}
        onOpenChange={(open) => !open && setTldToDelete(null)}
        title="Are you sure?"
        description={`This will start a background Temporal cleanup workflow to safely delete the TLD "${tldToDelete}" and its associated domains, hosts, and contacts.\n\nYou will be guided to review and approve the deletion counts in the workflow control center.`}
        onConfirm={confirmDelete}
        isDeleting={isDeleting}
      />
    </ListPageLayout>
  );
}
