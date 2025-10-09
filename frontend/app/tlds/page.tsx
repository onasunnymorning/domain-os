'use client';

import { useState, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useTLDs, useDeleteTLD } from '@/lib/hooks/useTLDs';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { TLDActivePhases } from '@/components/tlds/TLDActivePhases';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { 
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { 
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Globe, Plus, Trash2, Eye, Search, X } from 'lucide-react';
import Link from 'next/link';
import { useDebounce } from '@/lib/hooks/useDebounce';

export default function TLDsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [searchTerm, setSearchTerm] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [ryidFilter, setRyidFilter] = useState<string>(searchParams.get('ryid_equals') || '');
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [tldToDelete, setTldToDelete] = useState<string | null>(null);
  
  const debouncedSearch = useDebounce(searchTerm, 300);
  
  // Load ALL TLDs to determine which operators have TLDs
  const { data: allTLDsData } = useTLDs({ pagesize: 1000 });
  
  // Load registry operators for the filter dropdown
  const { data: operatorsData } = useRegistryOperators({ pagesize: 100 });
  
  // Filter operators to only show those that have at least one TLD
  const operatorsWithTLDs = operatorsData?.Data?.filter(op => {
    return allTLDsData?.Data?.some(tld => tld.RyID === op.RyID);
  }) || [];
  
  const { data, isLoading } = useTLDs({
    name_like: debouncedSearch || undefined,
    type_equals: typeFilter ? (typeFilter as any) : undefined,
    ryid_equals: ryidFilter || undefined,
  });
  
  const { mutate: deleteTLD, isPending: isDeleting } = useDeleteTLD();

  const handleDelete = (name: string) => {
    setTldToDelete(name);
    setDeleteDialogOpen(true);
  };

  const confirmDelete = () => {
    if (tldToDelete) {
      deleteTLD(tldToDelete);
      setDeleteDialogOpen(false);
      setTldToDelete(null);
    }
  };

  const getTypeBadgeVariant = (type: string) => {
    switch (type) {
      case 'generic':
        return 'default'; // blue
      case 'country-code':
        return 'secondary'; // green
      case 'second-level':
        return 'outline'; // purple
      default:
        return 'default';
    }
  };

  const tlds = data?.Data || [];

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <Globe className="h-8 w-8" />
              TLDs
            </h1>
            <p className="text-muted-foreground mt-2">
              Manage top-level domains in the registry
            </p>
          </div>
          <Button onClick={() => router.push('/tlds/create')}>
            <Plus className="mr-2 h-4 w-4" />
            Create TLD
          </Button>
        </div>

        {/* Filters */}
        <Card>
          <CardHeader>
            <CardTitle>Filters</CardTitle>
            <CardDescription>Search and filter TLDs</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-3">
              <div className="relative">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search by name..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="pl-8"
                />
              </div>
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
                    <SelectItem value="none" disabled>
                      No operators with TLDs
                    </SelectItem>
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
                      onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        setTypeFilter('');
                      }}
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
                      onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        setRyidFilter('');
                      }}
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </Badge>
                )}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setSearchTerm('');
                    setTypeFilter('');
                    setRyidFilter('');
                  }}
                >
                  Clear All Filters
                </Button>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Table */}
        <Card>
          <CardHeader>
            <CardTitle>TLD List</CardTitle>
            <CardDescription>
              {isLoading ? 'Loading...' : `${tlds.length} TLD${tlds.length !== 1 ? 's' : ''} found`}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="space-y-2">
                {[1, 2, 3, 4, 5].map((i) => (
                  <Skeleton key={i} className="h-12 w-full" />
                ))}
              </div>
            ) : tlds.length === 0 ? (
              <div className="text-center py-12">
                <Globe className="mx-auto h-12 w-12 text-muted-foreground" />
                <h3 className="mt-4 text-lg font-semibold">No TLDs found</h3>
                <p className="text-muted-foreground mt-2">
                  {searchTerm || typeFilter 
                    ? 'Try adjusting your filters' 
                    : 'Get started by creating your first TLD'}
                </p>
                {!searchTerm && !typeFilter && (
                  <Button onClick={() => router.push('/tlds/create')} className="mt-4">
                    <Plus className="mr-2 h-4 w-4" />
                    Create TLD
                  </Button>
                )}
              </div>
            ) : (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Type</TableHead>
                      <TableHead>Unicode Name</TableHead>
                      <TableHead>Registry Operator</TableHead>
                      <TableHead>DNS</TableHead>
                      <TableHead>Active Phases</TableHead>
                      <TableHead className="text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {tlds.map((tld) => (
                      <TableRow 
                        key={tld.Name}
                        className="cursor-pointer hover:bg-muted/50 transition-colors"
                        onClick={() => router.push(`/tlds/${tld.Name}`)}
                      >
                        <TableCell className="font-medium">{tld.Name}</TableCell>
                        <TableCell>
                          <Badge variant={getTypeBadgeVariant(tld.Type)}>
                            {tld.Type === 'generic' && 'gTLD'}
                            {tld.Type === 'country-code' && 'ccTLD'}
                            {tld.Type === 'second-level' && 'SLD'}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {tld.UName ? (
                            <span className="text-muted-foreground">{tld.UName}</span>
                          ) : (
                            <span className="text-muted-foreground italic">-</span>
                          )}
                        </TableCell>
                        <TableCell>{tld.RyID}</TableCell>
                        <TableCell>
                          {tld.EnableDNS ? (
                            <Badge variant="secondary" className="bg-green-100 text-green-800">
                              Enabled
                            </Badge>
                          ) : (
                            <Badge variant="outline">Disabled</Badge>
                          )}
                        </TableCell>
                        <TableCell onClick={(e) => e.stopPropagation()}>
                          <TLDActivePhases tldName={tld.Name} />
                        </TableCell>
                        <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                          <div className="flex justify-end gap-2">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleDelete(tld.Name)}
                              disabled={isDeleting}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                              <span className="sr-only">Delete</span>
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Are you sure?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete the TLD &quot;{tldToDelete}&quot; and all associated data.
              <br /><br />
              <strong>Note:</strong> TLDs with active phases cannot be deleted.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </DashboardLayout>
  );
}
