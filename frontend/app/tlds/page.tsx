'use client';

import { Suspense, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useTLDs, useDeleteTLD } from '@/lib/hooks/useTLDs';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { formatCompactNumber } from '@/lib/utils/numberUtils';
import { TLDActivePhases } from '@/components/tlds/TLDActivePhases';
import { TLDCreateDialog } from '@/components/tlds/TLDCreateDialog';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

import { 
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Globe, Plus, X, Download } from 'lucide-react';
import { WorkflowShortcuts } from '@/components/shared/WorkflowShortcuts';
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

function TLDsPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [searchTerm, setSearchTerm] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [ryidFilter, setRyidFilter] = useState<string>(searchParams.get('ryid_equals') || '');
  const [tldToDelete, setTldToDelete] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  
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
      setTldToDelete(null);
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

  // Sort by domain count descending
  const tlds = [...(data?.Data || [])].sort(
    (a, b) => (b.DomainCount ?? 0) - (a.DomainCount ?? 0)
  );

  const handleExportCSV = () => {
    const headers = ['TLD Name', 'Domains Count', 'DNS Status', 'Escrow Status', 'Registry Operator ID'];
    const rows = tlds.map(t => [
      t.Name,
      t.DomainCount ?? 0,
      t.EnableDNS ? 'Enabled' : 'Disabled',
      t.AllowEscrowImport ? 'Enabled' : 'Disabled',
      t.RyID || ''
    ]);

    const csvContent = [
      headers.join(','),
      ...rows.map(row => row.map(val => `"${String(val).replace(/"/g, '""')}"`).join(','))
    ].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.setAttribute('href', url);
    link.setAttribute('download', `tlds_export.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const columns: ColumnDef<any>[] = [
    {
      header: 'Name',
      accessor: 'Name',
      className: 'font-semibold text-base',
      cell: (tld) => <span title={tld.UName || undefined}>{tld.Name}</span>
    },
    {
      header: 'Domains',
      cell: (tld) => (
        <span className="cursor-help border-b border-dotted border-muted-foreground/50 pb-0.5"
              title={typeof tld.DomainCount === 'number' ? tld.DomainCount.toLocaleString() : undefined}>
          {typeof tld.DomainCount === 'number' ? formatCompactNumber(tld.DomainCount) : '—'}
        </span>
      )
    },
    {
      header: 'Registrars',
      cell: (tld) => <span>{tld.RegistrarCount ?? 0}</span>
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
      header: 'RO',
      accessor: 'RyID'
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
      header: 'DNS',
      cell: (tld) => tld.EnableDNS ? (
        <Badge variant="secondary" className="bg-green-100 text-green-800">Enabled</Badge>
      ) : (
        <Badge variant="outline">Disabled</Badge>
      )
    },
    {
      header: 'Escrow',
      cell: (tld) => tld.AllowEscrowImport ? (
        <Badge variant="secondary" className="bg-green-100 text-green-800">Enabled</Badge>
      ) : (
        <Badge variant="outline">Disabled</Badge>
      )
    },
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
      headerActions={
        <WorkflowShortcuts workflowKeys={['escrow-import', 'tld-cleanup']} />
      }
      actionButton={
        <Button onClick={() => setCreateOpen(true)} className="h-9">
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
        headerActions={
          <Button
            variant="outline"
            size="sm"
            onClick={handleExportCSV}
            disabled={tlds.length === 0}
            className="h-9 font-medium"
          >
            <Download className="h-4 w-4 mr-2" />
            Export CSV
          </Button>
        }
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
              <Button onClick={() => setCreateOpen(true)} className="mt-4">
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

      {/* Create dialog */}
      <TLDCreateDialog open={createOpen} onOpenChange={setCreateOpen} />
    </ListPageLayout>
  );
}
