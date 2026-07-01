'use client';

import { use, useEffect, useMemo, useState } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useTLD } from '@/lib/hooks/useTLDs';
import { useAccreditForTLD, useDeaccreditForTLD } from '@/lib/hooks/useAccreditations';
import { useRegistrars } from '@/lib/hooks/useRegistrars';
import { useDomainCountsForRegistrars } from '@/lib/hooks/useDomains';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';

import { ArrowLeft, Globe, CheckCircle, XCircle, Calendar, ChevronLeft, ChevronRight, Download } from 'lucide-react';
import { formatCompactNumber } from '@/lib/utils/numberUtils';
import Link from 'next/link';
import { format } from 'date-fns';
import { useQueryClient } from '@tanstack/react-query';
import { accreditationsApi } from '@/lib/api/accreditations';
import { getDomainCount } from '@/lib/api/domains';
import { getRegistrars } from '@/lib/api/registrars';
import { RegistrarSearchFilters } from '@/components/registrars/RegistrarSearchFilters';
import { PhaseTimeline } from '@/components/phases/PhaseTimeline';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useDebounce } from '@/lib/hooks/useDebounce';
import type { RegistrarListItem, RegistrarListParams } from '@/lib/types/registrar';
import { TLDAccreditedRegistrarCountWidget } from '@/components/tlds/TLDAccreditedRegistrarCountWidget';
import { TLDDomainCountWidget } from '@/components/tlds/TLDDomainCountWidget';
import { TLDReservedInventoryWidget } from '@/components/tlds/TLDReservedInventoryWidget';
import { TLDDUMsPieChartCard } from '@/components/tlds/TLDDUMsPieChartCard';
import posthog from 'posthog-js';

interface Props {
  params: Promise<{ name: string }>;
}

export default function TLDDetailPage({ params }: Props) {
  const { name } = use(params);
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const phaseName = searchParams.get('phase');
  const tabParam = searchParams.get('tab');
  const [activeTab, setActiveTab] = useState(phaseName ? 'phases' : (tabParam || 'phases'));

  const handleTabChange = (value: string) => {
    setActiveTab(value);
    const params = new URLSearchParams(searchParams.toString());
    params.set('tab', value);
    if (value !== 'phases') params.delete('phase');
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  };
  const queryClient = useQueryClient();
  const { data: tld, isLoading, error } = useTLD(decodeURIComponent(name));
  const tldName = decodeURIComponent(name);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const pageSize = 50;

  const [exporting, setExporting] = useState(false);

  const [searchQuery, setSearchQuery] = useState('');
  const [ianaIdQuery, setIanaIdQuery] = useState('');
  const debouncedSearch = useDebounce(searchQuery, 300);
  const debouncedIana = useDebounce(ianaIdQuery, 300);

  const regAccParams = useMemo(() => {
    const params: any = {
      pagesize: pageSize,
      cursor,
      tld: tldName,
    };
    const q = (debouncedSearch || '').trim();
    if (q) {
      params.name_like = q;
    }
    const iid = (debouncedIana || '').trim();
    if (iid && /^\d+$/.test(iid)) {
      params.gurid_equals = parseInt(iid, 10);
    }
    return params;
  }, [pageSize, cursor, debouncedSearch, debouncedIana, tldName]);

  const { data: regAccData, isLoading: regAccLoading } = useRegistrars(regAccParams);
  const accreditForTLD = useAccreditForTLD(tldName);
  const deaccreditForTLD = useDeaccreditForTLD(tldName);

  // Fetch DUMs for accredited registrars
  const regAccClIDs = useMemo(() => regAccData?.Data?.map(r => r.ClID) || [], [regAccData]);
  const domainCountsQueries = useDomainCountsForRegistrars(tldName, regAccClIDs);
  
  const domainCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    regAccClIDs.forEach((clid, index) => {
      counts[clid] = domainCountsQueries[index]?.data?.Count ?? 0;
    });
    return counts;
  }, [regAccClIDs, domainCountsQueries]);

  const sortedRegistrars = useMemo(() => {
    if (!regAccData?.Data) return [];
    return [...regAccData.Data].sort((a, b) => {
      const countA = domainCounts[a.ClID] || 0;
      const countB = domainCounts[b.ClID] || 0;
      return countB - countA;
    });
  }, [regAccData, domainCounts]);

  // Add accreditation modal state (search registrars)
  const [addOpen, setAddOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [addError, setAddError] = useState<string | null>(null);
  const debounced = useDebounce(search, 300);
  const regParams: RegistrarListParams = useMemo(() => {
    const p: RegistrarListParams = { pagesize: 20 };
    const q = (debounced || '').trim();
    if (q) {
      // Filter by name only as requested to avoid overly restrictive AND filtering in backend
      p.name_like = q;
    }
    return p;
  }, [debounced]);
  const { data: registrarSearch, isLoading: regSearchLoading } = useRegistrars(regParams);

  // De-accredit modal state
  const [deaccOpen, setDeaccOpen] = useState(false);
  const [selectedRegistrar, setSelectedRegistrar] = useState<RegistrarListItem | null>(null);
  const [confirmText, setConfirmText] = useState('');
  const [deaccError, setDeaccError] = useState<string | null>(null);

  // Scroll to top and reset pagination when the page loads
  useEffect(() => {
    window.scrollTo(0, 0);
    setCursor(undefined);
    setCursorStack([]);
  }, [name]);

  // Reset pagination when search queries change
  useEffect(() => {
    setCursor(undefined);
    setCursorStack([]);
  }, [debouncedSearch, debouncedIana]);

  const getTypeBadge = (type: string) => {
    switch (type) {
      case 'generic':
        return <Badge variant="default">Generic TLD (gTLD)</Badge>;
      case 'country-code':
        return <Badge variant="secondary">Country-Code TLD (ccTLD)</Badge>;
      case 'second-level':
        return <Badge variant="outline">Second-Level Domain (SLD)</Badge>;
      default:
        return <Badge variant="outline">{type}</Badge>;
    }
  };

  if (error) {
    return (
      <DashboardLayout>
        <div className="space-y-6">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="icon" asChild>
              <Link href="/tlds">
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div>
              <h1 className="text-3xl font-bold tracking-tight">TLD Not Found</h1>
            </div>
          </div>
          <Card>
            <CardContent className="pt-6">
              <p className="text-muted-foreground">
                The TLD &quot;{name}&quot; could not be found.
              </p>
              <Button asChild className="mt-4">
                <Link href="/tlds">Back to TLDs</Link>
              </Button>
            </CardContent>
          </Card>
        </div>
      </DashboardLayout>
    );
  }

  const handleExportCSV = async () => {
    if (!tldName) return;
    setExporting(true);
    try {
      // 1. Fetch filtered accredited registrars (large page size, matching current filters)
      const exportParams = {
        pagesize: 1000,
        tld: tldName,
        name_like: regAccParams.name_like,
        gurid_equals: regAccParams.gurid_equals,
      };
      const res = await getRegistrars(exportParams);
      const registrars = res.Data || [];
      
      // 2. Fetch TLD-specific domain counts for each registrar in parallel
      const counts = await Promise.all(
        registrars.map(async (r) => {
          try {
            const countRes = await getDomainCount({ tld_equals: tldName, clid_equals: r.ClID });
            return { clid: r.ClID, count: countRes?.Count ?? 0 };
          } catch {
            return { clid: r.ClID, count: 0 };
          }
        })
      );
      
      const countsMap = new Map(counts.map(c => [c.clid, c.count]));

      // 3. Generate CSV content
      const headers = ['Client ID', 'Name', 'IANA ID', 'Status', 'Auto-renew', 'DUMs'];
      const rows = registrars.map(r => [
        r.ClID,
        r.Name,
        r.GurID || '',
        r.Status,
        r.Autorenew ? 'Enabled' : 'Disabled',
        countsMap.get(r.ClID) || 0
      ]);

      const csvContent = [
        headers.join(','),
        ...rows.map(row => row.map(val => `"${String(val).replace(/"/g, '""')}"`).join(','))
      ].join('\n');

      // 4. Trigger download
      const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.setAttribute('href', url);
      link.setAttribute('download', `${tldName}_accredited_registrars.csv`);
      link.style.visibility = 'hidden';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (err) {
      console.error('Failed to export CSV:', err);
    } finally {
      setExporting(false);
    }
  };

  return (
    <DashboardLayout>
      <div className="space-y-8">
        {/* Back Button */}
        <div className="flex items-center justify-between">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/tlds">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back
            </Link>
          </Button>
          {!isLoading && tld && (
            <Button size="sm" asChild className="ml-2">
              <Link href={`/tlds/${encodeURIComponent(tld.Name)}/edit`}>Edit</Link>
            </Button>
          )}
        </div>

        {/* Hero Section */}
        <div className="space-y-2">
          <div className="flex items-baseline gap-3">
            <Globe className="h-10 w-10 text-muted-foreground" />
            {isLoading ? (
              <Skeleton className="h-12 w-48" />
            ) : (
              <h1 className="text-5xl font-bold tracking-tight">{tld?.Name}</h1>
            )}
          </div>
          {tld?.RyID && (
            <div className="ml-[52px]">
              <Link href={`/registry-operators/${tld.RyID}`}>
                <Badge variant="outline" className="font-mono mt-1 text-sm cursor-pointer hover:bg-primary/10 hover:border-primary/30 transition-colors">{tld.RyID}</Badge>
              </Link>
            </div>
          )}
        </div>

        {/* Count Widgets */}
        {tld && (
          <div className="grid gap-6 md:grid-cols-4">
            <TLDDomainCountWidget tldName={tld.Name} />
            <TLDReservedInventoryWidget tldName={tld.Name} />
            <TLDAccreditedRegistrarCountWidget 
              count={tld?.RegistrarCount ?? 0} 
              isLoading={isLoading} 
              onClick={() => handleTabChange('registrars')} 
            />
            <TLDDUMsPieChartCard 
              data={sortedRegistrars.map(r => ({ name: r.Name, clid: r.ClID, value: domainCounts[r.ClID] || 0 }))} 
              onClick={() => handleTabChange('registrars')}
            />
          </div>
        )}

        {/* Tabbed Content */}
        {!isLoading && tld && (
          <Tabs value={activeTab} onValueChange={handleTabChange}>
            <TabsList>
              <TabsTrigger value="phases">Phases</TabsTrigger>
              <TabsTrigger value="registrars">
                Registrars
                {tld?.RegistrarCount !== undefined && (
                  <Badge variant="secondary" className="ml-1.5 text-[10px] px-1.5 py-0 h-4 min-w-[1.25rem] rounded-full">
                    {tld.RegistrarCount}
                  </Badge>
                )}
              </TabsTrigger>
              <TabsTrigger value="details">Details</TabsTrigger>
            </TabsList>

            <TabsContent value="phases" className="mt-6" forceMount>
              <div className={activeTab !== 'phases' ? 'hidden' : undefined}>
              <PhaseTimeline
                tldName={tld.Name}
                initialPhaseName={phaseName || undefined}
              />
              </div>
            </TabsContent>

            <TabsContent value="registrars" className="mt-6" forceMount>
              <div className={activeTab !== 'registrars' ? 'hidden' : undefined}>
              <Card>
                <CardHeader className="flex flex-row items-start justify-between gap-4">
                  <div>
                    <CardTitle>Accredited Registrars</CardTitle>
                    <CardDescription>
                      {isLoading ? 'Loading accredited registrars…' : `${tld?.RegistrarCount ?? 0} registrar${(tld?.RegistrarCount ?? 0) !== 1 ? 's' : ''} accredited`}
                    </CardDescription>
                  </div>
                  <div className="pt-1 flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleExportCSV}
                      disabled={exporting || (regAccData?.Data?.length ?? 0) === 0}
                      className="shrink-0 font-medium h-9"
                      title={
                        (regAccData?.Data?.length ?? 0) === 0
                          ? "No registrars to export"
                          : exporting
                          ? "Exporting to CSV..."
                          : "Export filtered registrar list to CSV"
                      }
                    >
                      <Download className="h-4 w-4 mr-2" />
                      {exporting ? 'Exporting...' : 'Export CSV'}
                    </Button>
                    <Button size="sm" onClick={() => setAddOpen(true)} className="h-9">Accredit registrar</Button>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex flex-wrap items-center justify-between gap-4 pb-2">
                    <RegistrarSearchFilters
                      searchQuery={searchQuery}
                      setSearchQuery={setSearchQuery}
                      ianaIdQuery={ianaIdQuery}
                      setIanaIdQuery={setIanaIdQuery}
                      className="flex-1"
                    />
                  </div>

                  {regAccLoading ? (
                    <div className="space-y-2">
                      {[1, 2, 3, 4].map(i => (
                        <Skeleton key={i} className="h-10 w-full" />
                      ))}
                    </div>
                  ) : (regAccData?.Data?.length ?? 0) === 0 ? (
                    <div className="text-center py-8 text-muted-foreground">
                      {searchQuery || ianaIdQuery
                        ? "No accredited registrars match your search query."
                        : "No registrars accredited for this TLD."}
                    </div>
                  ) : (
                    <div className="space-y-4">
                      <div className="rounded-md border overflow-x-auto">
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead className="text-right cursor-help" title="Domains Under Management (active domains under this TLD)">DUMs</TableHead>
                              <TableHead className="cursor-help" title="Client ID / Registrar ID (unique identifier)">ClID</TableHead>
                              <TableHead>Name</TableHead>
                              <TableHead>Status</TableHead>
                              <TableHead>Auto-renew</TableHead>
                              <TableHead className="w-[140px]"></TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {sortedRegistrars.map((r: RegistrarListItem) => {
                              const clidIndex = regAccClIDs.indexOf(r.ClID);
                              const isCountLoading = clidIndex >= 0 ? domainCountsQueries[clidIndex]?.isLoading : false;
                              const count = domainCounts[r.ClID] || 0;
                              return (
                              <TableRow key={r.ClID}>
                                <TableCell className="text-right whitespace-nowrap font-mono text-muted-foreground" title={count.toLocaleString()}>
                                  {isCountLoading ? <Skeleton className="h-4 w-8 inline-block" /> : formatCompactNumber(count)}
                                </TableCell>
                                <TableCell className="font-mono">
                                  <Link href={`/registrars/${encodeURIComponent(r.ClID)}`} className="text-primary hover:underline">{r.ClID}</Link>
                                </TableCell>
                                <TableCell>
                                  <Link href={`/registrars/${encodeURIComponent(r.ClID)}`} className="text-primary hover:underline">{r.Name}</Link>
                                </TableCell>
                                <TableCell>
                                  <Badge variant={r.Status === 'ok' ? 'default' : r.Status === 'terminated' ? 'destructive' : 'secondary'}>
                                    {r.Status}
                                  </Badge>
                                </TableCell>
                                <TableCell>
                                  <Badge variant={r.Autorenew ? 'default' : 'outline'}>
                                    {r.Autorenew ? 'Enabled' : 'Disabled'}
                                  </Badge>
                                </TableCell>
                                <TableCell className="text-right">
                                  <Button
                                    size="sm"
                                    variant="destructive"
                                    onClick={() => {
                                      setSelectedRegistrar(r);
                                      setConfirmText('');
                                      setDeaccError(null);
                                      setDeaccOpen(true);
                                    }}
                                  >
                                    De-accredit
                                  </Button>
                                </TableCell>
                              </TableRow>
                            );})
                            }
                          </TableBody>
                        </Table>
                      </div>

                      {regAccData?.Data && regAccData.Data.length > 0 && (
                        <div className="flex items-center justify-between pt-2">
                          <p className="text-sm text-muted-foreground">
                            Showing page {cursorStack.length + 1}
                          </p>
                          <div className="flex items-center gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                if (cursorStack.length > 0) {
                                  const prev = [...cursorStack];
                                  const c = prev.pop();
                                  setCursorStack(prev);
                                  setCursor(c);
                                }
                              }}
                              disabled={regAccLoading || cursorStack.length === 0}
                            >
                              <ChevronLeft className="h-4 w-4" /> Previous
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                const nextCursor = regAccData?.Meta?.PageCursor;
                                if (nextCursor) {
                                  setCursorStack((s) => (cursor ? [...s, cursor] : s));
                                  setCursor(nextCursor);
                                }
                              }}
                              disabled={regAccLoading || !regAccData?.Meta?.PageCursor}
                            >
                              Next <ChevronRight className="h-4 w-4 ml-1" />
                            </Button>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
              </div>
            </TabsContent>

            <TabsContent value="details" className="mt-6" forceMount>
              <div className={activeTab !== 'details' ? 'hidden' : undefined}>
              <Card>
                <CardHeader>
                  <CardTitle>Details</CardTitle>
                </CardHeader>
                <CardContent>
                  {isLoading ? (
                    <div className="space-y-6">
                      {[1, 2, 3, 4].map((i) => (
                        <div key={i} className="space-y-2">
                          <Skeleton className="h-3 w-20" />
                          <Skeleton className="h-6 w-full max-w-md" />
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="space-y-8">
                      {/* Type */}
                      <div className="space-y-2">
                        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Type</p>
                        <div>{tld && getTypeBadge(tld.Type)}</div>
                      </div>

                      {/* Status Grid */}
                      <div className="grid gap-6 md:grid-cols-2">
                        <div className="space-y-2">
                          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">DNS</p>
                          <div>
                            {tld?.EnableDNS ? (
                              <Badge variant="secondary" className="bg-green-100 text-green-800">
                                <CheckCircle className="mr-1 h-3 w-3" /> Enabled
                              </Badge>
                            ) : (
                              <Badge variant="outline">
                                <XCircle className="mr-1 h-3 w-3" /> Disabled
                              </Badge>
                            )}
                          </div>
                        </div>

                        <div className="space-y-2">
                          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Escrow Import</p>
                          <div>
                            {tld?.AllowEscrowImport ? (
                              <Badge variant="secondary" className="bg-green-100 text-green-800">
                                <CheckCircle className="mr-1 h-3 w-3" /> Enabled
                              </Badge>
                            ) : (
                              <Badge variant="outline">
                                <XCircle className="mr-1 h-3 w-3" /> Disabled
                              </Badge>
                            )}
                          </div>
                        </div>
                      </div>

                      {/* Metadata */}
                      <div className="pt-6 border-t">
                        <div className="flex flex-col sm:flex-row sm:items-center gap-4 sm:gap-8 text-sm text-muted-foreground">
                          <div className="flex items-center gap-2">
                            <Calendar className="h-4 w-4" />
                            <span>Created {tld && format(new Date(tld.CreatedAt), 'PPpp')}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <Calendar className="h-4 w-4" />
                            <span>Updated {tld && format(new Date(tld.UpdatedAt), 'PPpp')}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>
              </div>
            </TabsContent>
          </Tabs>
        )}

        {/* Add accreditation dialog */}
        <Dialog open={addOpen} onOpenChange={(v) => { setAddOpen(v); if (!v) { setAddError(null); setSearch(''); } }}>
          <DialogContent className="sm:max-w-3xl w-[min(95vw,1100px)]">
            <DialogHeader>
              <DialogTitle>Accredit registrar to .{tld?.Name}</DialogTitle>
              <DialogDescription>Search for a registrar to accredit for this TLD.</DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <Input placeholder="Search registrars by name or ClID…" value={search} onChange={(e) => setSearch(e.target.value)} />
              <p className="text-xs text-muted-foreground">Only registrars with status <span className="font-medium">ok</span> can be accredited.</p>
              {addError && (
                <div className="text-sm text-red-600">{addError}</div>
              )}
              <div className="rounded-md border max-h-80 overflow-y-auto overflow-x-auto">
                {regSearchLoading ? (
                  <div className="p-4 text-sm text-muted-foreground">Searching…</div>
                ) : (registrarSearch?.Data?.length ?? 0) === 0 ? (
                  <div className="p-4 text-sm text-muted-foreground">No registrars found</div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>ClID</TableHead>
                        <TableHead>Name</TableHead>
                        <TableHead className="w-[120px] text-center">Status</TableHead>
                        <TableHead className="w-[140px]"></TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {registrarSearch!.Data!.map((r: RegistrarListItem) => (
                        <TableRow key={r.ClID}>
                          <TableCell className="font-mono">{r.ClID}</TableCell>
                          <TableCell>{r.Name}</TableCell>
                          <TableCell className="text-center">
                            <Badge variant={r.Status === 'ok' ? 'default' : r.Status === 'terminated' ? 'destructive' : 'secondary'}>
                              {r.Status}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              size="sm"
                              disabled={accreditForTLD.isPending || r.Status !== 'ok'}
                              title={r.Status !== 'ok' ? `Registrar status is ${r.Status}` : undefined}
                              onClick={async () => {
                                setAddError(null);
                                try {
                                  await accreditForTLD.mutateAsync(r.ClID);
                                  posthog.capture('registrar_accredited_to_tld', {
                                    tld_name: tldName,
                                    registrar_clid: r.ClID,
                                    registrar_name: r.Name,
                                  });
                                  queryClient.invalidateQueries({ queryKey: ['registrars'] });
                                  queryClient.invalidateQueries({ queryKey: ['tlds', tldName] });
                                  setAddOpen(false);
                                  setSearch('');
                                } catch (err) {
                                  const e = err as { response?: { data?: { error?: string } } };
                                  setAddError(e?.response?.data?.error || 'Failed to accredit');
                                }
                              }}
                            >
                              {r.Status !== 'ok' ? 'Not eligible' : (accreditForTLD.isPending ? 'Adding…' : 'Accredit')}
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </div>
            </div>
          </DialogContent>
        </Dialog>

        {/* De-accredit dialog */}
        <Dialog open={deaccOpen} onOpenChange={(v) => { setDeaccOpen(v); if (!v) { setDeaccError(null); setConfirmText(''); } }}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>De-accredit {selectedRegistrar?.Name}</DialogTitle>
              <DialogDescription>
                This will remove this registrar’s accreditation for .{tld?.Name}.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                To confirm, type: <span className="font-mono">{"delete "}{selectedRegistrar?.ClID}</span>
              </p>
              <Input
                autoFocus
                placeholder={`delete ${selectedRegistrar?.ClID ?? ''}`}
                value={confirmText}
                onChange={(e) => setConfirmText(e.target.value)}
              />
              {deaccError && (
                <div className="text-sm text-red-600">{deaccError}</div>
              )}
              <div className="flex items-center justify-end gap-2 pt-2">
                <Button variant="outline" onClick={() => setDeaccOpen(false)} disabled={deaccreditForTLD.isPending}>Cancel</Button>
                <Button
                  variant="destructive"
                  disabled={
                    deaccreditForTLD.isPending || !selectedRegistrar || confirmText.trim() !== `delete ${selectedRegistrar?.ClID}`
                  }
                  onClick={async () => {
                    if (!selectedRegistrar) return;
                    setDeaccError(null);
                    try {
                      await deaccreditForTLD.mutateAsync(selectedRegistrar.ClID);
                      posthog.capture('registrar_deaccredited_from_tld', {
                        tld_name: tldName,
                        registrar_clid: selectedRegistrar.ClID,
                        registrar_name: selectedRegistrar.Name,
                      });
                      queryClient.invalidateQueries({ queryKey: ['registrars'] });
                      queryClient.invalidateQueries({ queryKey: ['tlds', tldName] });
                      setDeaccOpen(false);
                      setConfirmText('');
                      setSelectedRegistrar(null);
                    } catch (err) {
                      const e = err as { response?: { data?: { error?: string } } };
                      setDeaccError(e?.response?.data?.error || 'Failed to de-accredit');
                    }
                  }}
                >
                  {deaccreditForTLD.isPending ? 'Removing…' : 'Confirm de-accredit'}
                </Button>
              </div>
            </div>
          </DialogContent>
        </Dialog>

      </div>
    </DashboardLayout>
  );
}
