'use client';

import { use, useEffect, useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useTLD, useDeleteTLD } from '@/lib/hooks/useTLDs';
import { useTLDRegistrars, useAccreditForTLD, useDeaccreditForTLD } from '@/lib/hooks/useAccreditations';
import { useRegistrars } from '@/lib/hooks/useRegistrars';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { ArrowLeft, Globe, Trash2, CheckCircle, XCircle, Calendar, Building2 } from 'lucide-react';
import Link from 'next/link';
import { format } from 'date-fns';
import { PhaseTimeline } from '@/components/phases/PhaseTimeline';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useDebounce } from '@/lib/hooks/useDebounce';
import type { RegistrarListItem, RegistrarListParams } from '@/lib/types/registrar';

interface Props {
  params: Promise<{ name: string }>;
}

export default function TLDDetailPage({ params }: Props) {
  const { name } = use(params);
  const router = useRouter();
  const searchParams = useSearchParams();
  const phaseName = searchParams.get('phase');
  const { data: tld, isLoading, error } = useTLD(decodeURIComponent(name));
  const { mutate: deleteTLD, isPending: isDeleting } = useDeleteTLD();
  const tldName = decodeURIComponent(name);
  const { data: regAccData, isLoading: regAccLoading } = useTLDRegistrars(tldName, { pagesize: 100 });
  const accreditForTLD = useAccreditForTLD(tldName);
  const deaccreditForTLD = useDeaccreditForTLD(tldName);

  // Add accreditation modal state (search registrars)
  const [addOpen, setAddOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [addError, setAddError] = useState<string | null>(null);
  const debounced = useDebounce(search, 300);
  const regParams: RegistrarListParams = useMemo(() => {
    const p: RegistrarListParams = { pagesize: 20 };
    const q = (debounced || '').trim();
    if (q) {
      // Always search by name and ClID for a consistent fuzzy experience
      p.name_like = q;
      p.clid_like = q;
      // Avoid switching to gurid_equals-only on numeric input.
    }
    return p;
  }, [debounced]);
  const { data: registrarSearch, isLoading: regSearchLoading } = useRegistrars(regParams);

  // De-accredit modal state
  const [deaccOpen, setDeaccOpen] = useState(false);
  const [selectedRegistrar, setSelectedRegistrar] = useState<RegistrarListItem | null>(null);
  const [confirmText, setConfirmText] = useState('');
  const [deaccError, setDeaccError] = useState<string | null>(null);

  // Scroll to top when the page loads
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [name]);

  const handleDelete = () => {
    deleteTLD(name, {
      onSuccess: () => {
        router.push('/tlds');
      },
    });
  };

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
          {!isLoading && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" size="sm" disabled={isDeleting}>
                  <Trash2 className="mr-2 h-4 w-4" />
                  Delete TLD
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Are you sure?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will permanently delete the TLD &quot;{tld?.Name}&quot; and all associated data.
                    This action cannot be undone.
                    <br /><br />
                    <strong>Note:</strong> TLDs with active phases cannot be deleted.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground">
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
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
          <p className="text-sm text-muted-foreground ml-[52px]">TLD Details</p>
        </div>

        {/* TLD Information Card */}
        <Card>
          <CardContent className="pt-6">
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

                {/* Registry Operator */}
                <div className="space-y-2">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-2">
                    <Building2 className="h-3 w-3" />
                    Registry Operator
                  </p>
                  <Link 
                    href={`/registry-operators/${tld?.RyID}`} 
                    className="text-lg font-medium text-primary hover:underline inline-block"
                  >
                    {tld?.RyID}
                  </Link>
                </div>

                {/* Status Grid */}
                <div className="grid gap-6 md:grid-cols-2">
                  <div className="space-y-2">
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">DNS Enabled</p>
                    <div>
                      {tld?.EnableDNS ? (
                        <Badge variant="secondary" className="bg-green-100 text-green-800">
                          <CheckCircle className="mr-1 h-3 w-3" />
                          Yes
                        </Badge>
                      ) : (
                        <Badge variant="outline">
                          <XCircle className="mr-1 h-3 w-3" />
                          No
                        </Badge>
                      )}
                    </div>
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Escrow Import</p>
                    <div>
                      {tld?.AllowEscrowImport ? (
                        <Badge variant="secondary">Allowed</Badge>
                      ) : (
                        <Badge variant="outline">Not Allowed</Badge>
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

        {/* Registrars Accredited for this TLD */}
        <Card>
          <CardHeader className="flex flex-row items-start justify-between gap-4">
            <div>
              <CardTitle>Accredited Registrars</CardTitle>
              <CardDescription>
                {regAccLoading ? 'Loading accredited registrars…' : `${regAccData?.Data?.length ?? 0} registrar${(regAccData?.Data?.length ?? 0) !== 1 ? 's' : ''} accredited`}
              </CardDescription>
            </div>
            <div className="pt-1">
              <Button size="sm" onClick={() => setAddOpen(true)}>Accredit registrar</Button>
            </div>
          </CardHeader>
          <CardContent>
            {regAccLoading ? (
              <div className="space-y-2">
                {[1,2,3,4].map(i => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : (regAccData?.Data?.length ?? 0) === 0 ? (
              <div className="text-center py-8 text-muted-foreground">No registrars accredited for this TLD</div>
            ) : (
              <div className="rounded-md border overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>ClID</TableHead>
                      <TableHead>Name</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="w-[140px]"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {regAccData!.Data!.map((r: RegistrarListItem) => (
                      <TableRow key={r.ClID}>
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
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>

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

        {/* Phase Timeline */}
        {!isLoading && tld && (
          <PhaseTimeline 
            tldName={tld.Name} 
            initialPhaseName={phaseName || undefined}
          />
        )}
      </div>
    </DashboardLayout>
  );
}
