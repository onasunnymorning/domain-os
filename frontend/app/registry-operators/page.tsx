'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useRegistryOperators, useDeleteRegistryOperator } from '@/lib/hooks/useRegistryOperators';
import { TLDBadges } from '@/components/registry-operators/TLDBadges';
import { Button } from '@/components/ui/button';
import { PlusIcon, Trash2, Building2 } from 'lucide-react';
import Link from 'next/link';
import { toast } from 'sonner';

import { ListPageLayout } from '@/components/shared/ListPageLayout';
import { DataTable, ColumnDef } from '@/components/shared/DataTable';
import { SearchFilter } from '@/components/shared/SearchFilter';
import { DeleteConfirmDialog } from '@/components/shared/DeleteConfirmDialog';

export default function RegistryOperatorsPage() {
  const router = useRouter();
  const [searchTerm, setSearchTerm] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  
  const { data, isLoading, error } = useRegistryOperators({
    name_like: searchTerm || undefined,
  });
  
  const deleteMutation = useDeleteRegistryOperator();

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await deleteMutation.mutateAsync(deleteId);
      toast.success('Registry operator deleted successfully');
      setDeleteId(null);
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to delete registry operator');
    }
  };

  const columns: ColumnDef<any>[] = [
    {
      header: 'RyID',
      accessor: 'RyID',
      className: 'font-mono text-sm'
    },
    {
      header: 'Name',
      accessor: 'Name',
      className: 'font-medium'
    },
    {
      header: 'Email',
      accessor: 'Email',
      className: 'text-muted-foreground'
    },
    {
      header: 'TLDs',
      cell: (operator) => <TLDBadges ryid={operator.RyID} maxDisplay={3} />
    },
    {
      header: 'URL',
      cell: (operator) => operator.URL ? (
        <a 
          href={operator.URL} 
          target="_blank" 
          rel="noopener noreferrer"
          className="text-primary hover:underline"
          onClick={(e) => e.stopPropagation()}
        >
          {operator.URL}
        </a>
      ) : (
        <span className="text-muted-foreground">-</span>
      )
    },
    {
      header: 'Actions',
      className: 'text-right',
      cell: (operator) => (
        <div className="flex justify-end gap-2">
          <Button 
            variant="ghost" 
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              setDeleteId(operator.RyID);
            }}
          >
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        </div>
      )
    }
  ];

  return (
    <ListPageLayout
      icon={Building2}
      title="Registry Operators"
      description="Manage registry operators in your system"
      actionButton={
        <Link href="/registry-operators/create">
          <Button>
            <PlusIcon className="mr-2 h-4 w-4" />
            Create Operator
          </Button>
        </Link>
      }
      filters={
        <div className="flex items-center gap-4">
          <SearchFilter 
            value={searchTerm} 
            onChange={setSearchTerm} 
            placeholder="Search by name..." 
          />
        </div>
      }
    >
      <DataTable
        title="All Registry Operators"
        description={`${data?.Data?.length || 0} registry operator(s) found`}
        columns={columns}
        data={data?.Data || []}
        keyExtractor={(row) => row.RyID}
        isLoading={isLoading}
        onRowClick={(row) => router.push(`/registry-operators/${row.RyID}`)}
        error={error ? `Error loading registry operators: ${error.message}` : undefined}
        emptyState={
          <div className="flex flex-col items-center justify-center py-12 text-center">
            <div className="rounded-full bg-muted p-3 mb-4">
              <Building2 className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold">No registry operators found</h3>
            <p className="text-sm text-muted-foreground mb-4">
              {searchTerm 
                ? 'Try adjusting your search terms'
                : 'Get started by creating your first registry operator'
              }
            </p>
            <Link href="/registry-operators/create">
              <Button>
                <PlusIcon className="mr-2 h-4 w-4" />
                Create Registry Operator
              </Button>
            </Link>
          </div>
        }
      />

      <DeleteConfirmDialog
        open={!!deleteId}
        onOpenChange={(open) => !open && setDeleteId(null)}
        description="This action cannot be undone. This will permanently delete the registry operator."
        onConfirm={handleDelete}
        isDeleting={deleteMutation.isPending}
      />
    </ListPageLayout>
  );
}
