'use client';

import { useEffect, useMemo, useState } from 'react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { ChevronDown, Loader2, CheckCircle2, XCircle, Info } from 'lucide-react';
import { presignUpload, startEscrowImport, listEscrowImports, startEscrowIngestion, EscrowRunItem } from '@/lib/api/escrow';
import { apiClient } from '@/lib/api/client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { tldsApi, TLD } from '@/lib/api/tlds';
import { useDebounce } from '@/lib/hooks/useDebounce';
import { useTLD } from '@/lib/hooks/useTLDs';
import { useRegistrar, useCreateRegistrar } from '@/lib/hooks/useRegistrars';
import { useRegistryOperator } from '@/lib/hooks/useRegistryOperators';
import { toast } from 'sonner';
import { getRegistrarByClID } from '@/lib/api/registrars';

import { RegistrarOverrideForm } from '@/components/escrow/RegistrarOverrideForm';

import { Switch } from '@/components/ui/switch';

export default function EscrowPage() {
  // Upload & start state
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [objectKey, setObjectKey] = useState<string>('');
  const [tld, setTld] = useState<string>('');
  const [registrarOverrides, setRegistrarOverrides] = useState<Record<string, string>>({});
  const [importMode, setImportMode] = useState<'api' | 'direct_db'>('api');
  const [startStatus, setStartStatus] = useState<string>('');
  const [uploadStatus, setUploadStatus] = useState<string>('');
  const [workflowLink, setWorkflowLink] = useState<{ id: string; url?: string } | null>(null);
  const [tldOpen, setTldOpen] = useState(false);
  const [tldQuery, setTldQuery] = useState('');

  // Runs filter state (no URL syncing to avoid confusion between tabs)
  const [filterTld, setFilterTld] = useState<string>('');
  const [runsTldOpen, setRunsTldOpen] = useState(false);
  const [runsTldQuery, setRunsTldQuery] = useState('');
  const [runs, setRuns] = useState<EscrowRunItem[]>([]);
  const [loadingRuns, setLoadingRuns] = useState(false);
  const [error, setError] = useState<string>('');

  // Fetch runs whenever filterTld changes
  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoadingRuns(true);
      setError('');
      try {
        const res = await listEscrowImports(filterTld, 20);
        if (!cancelled) setRuns(res.items || []);
      } catch (e: any) {
        if (!cancelled) setError(e?.message || 'Failed to load runs');
      } finally {
        if (!cancelled) setLoadingRuns(false);
      }
    }
    if (filterTld.trim()) load();
    return () => {
      cancelled = true;
    };
  }, [filterTld]);

  // Intentionally no URL syncing for runs filter to keep page URL stable across tabs.

  // (optional) objectKey validation could go here if needed

  // Debounced TLD search
  useEffect(() => {
    const id = setTimeout(() => {
      setTldQuery((q) => q); // trigger rerender
    }, 250);
    return () => clearTimeout(id);
  }, [tldQuery]);

  async function handleUpload() {
    if (!file) return;
    setUploading(true);
    setUploadStatus('');
    // Prepare options
    const options = {
      mapRegistrars: true, // Legacy flag, mostly ignored by backend now but harmless
      registrarOverrides,
      importMode,
    };

    try {
      const presign = await presignUpload(file.name);
      const putRes = await fetch(presign.url, {
        method: 'PUT',
        body: file,
        headers: {
          'Content-Type': 'application/octet-stream',
        },
      });
      if (!putRes.ok) throw new Error(`Upload failed with status ${putRes.status}`);
      setObjectKey(presign.objectKey);
      // Auto start workflow on success
      try {
        const res = await startEscrowImport({ tld, objectKey: presign.objectKey, options });
        setWorkflowLink({ id: res.workflowId, url: res.url });
        setStartStatus(`Started workflow ${res.workflowId}`);
      } catch (e: any) {
        setStartStatus(e?.message || 'Failed to start workflow');
      }
    } catch (e: any) {
      // Fallback: proxy upload via backend to avoid MinIO CORS issues in dev
      try {
        const fd = new FormData();
        fd.append('file', file);
        const { data } = await apiClient.post('/escrow/uploads', fd, {
          headers: { 'Content-Type': 'multipart/form-data' },
        });
        if (typeof data.objectKey === 'string' && data.objectKey.startsWith('escrow/')) {
          setObjectKey(data.objectKey);
          setUploadStatus('Uploaded via proxy');
          try {
            const res = await startEscrowImport({ tld, objectKey: data.objectKey, options });
            setWorkflowLink({ id: res.workflowId, url: res.url });
            setStartStatus(`Started workflow ${res.workflowId}`);
          } catch (werr: any) {
            setStartStatus(werr?.message || 'Failed to start workflow');
          }
        } else {
          setUploadStatus('Upload stored locally; object not in escrow bucket');
        }
      } catch (proxyErr: any) {
        setUploadStatus(proxyErr?.message || e?.message || 'Upload failed');
      }
    } finally {
      setUploading(false);
    }
  }

  // No manual start; workflow auto-starts after upload completes
  // Readiness checks for Upload & Start: require escrow imports enabled and two registrars
  const { data: tldInfo, isLoading: tldInfoLoading } = useTLD(tld || '');
  const clid9999 = tld ? `9999-${tld}` : '';
  const clid9998 = tld ? `9998-${tld}` : '';
  const { data: reg9999, isLoading: reg9999Loading, isError: reg9999Error } = useRegistrar(clid9999, !!tld);
  const { data: reg9998, isLoading: reg9998Loading, isError: reg9998Error } = useRegistrar(clid9998, !!tld);
  const escrowEnabled = !!tld && !!tldInfo?.AllowEscrowImport;
  const has9999 = !!tld && !!reg9999 && !reg9999Error && reg9999?.ClID === clid9999;
  const has9998 = !!tld && !!reg9998 && !reg9998Error && reg9998?.ClID === clid9998;
  const checksLoading = !!tld && (tldInfoLoading || reg9999Loading || reg9998Loading);
  const readyToUpload = !!tld && escrowEnabled && has9999 && has9998 && !checksLoading;

  // Registry Operator for selected TLD (for email)
  const ryid = tldInfo?.RyID || '';
  const { data: registryOperator, isLoading: roLoading } = useRegistryOperator(ryid);
  const { mutateAsync: createRegistrarMutate, isPending: creatingRegistrar } = useCreateRegistrar();
  const queryClient = useQueryClient();

  async function handleCreateMissingRegistrars() {
    if (!tld) return;
    const email = registryOperator?.Email;
    if (!email) {
      toast.error('Registry Operator email not available');
      return;
    }
    const payloads: any[] = [];
    if (!has9999) {
      payloads.push({
        ClID: clid9999,
        Name: `${tld.toUpperCase()} NoBill RO Registrar`,
        NickName: `${tld.toUpperCase()} NoBill RO Registrar`,
        GurID: 9999,
        Email: email,
        Status: 'ok',
        IANAStatus: 'Reserved',
        PostalInfo: [
          {
            Type: 'int',
            Address: { City: 'Mancora', CC: 'PE' },
          },
        ],
      });
    }
    if (!has9998) {
      payloads.push({
        ClID: clid9998,
        Name: `${tld.toUpperCase()} Bill RO Registrar`,
        NickName: `${tld.toUpperCase()} Bill RO Registrar`,
        GurID: 9998,
        Email: email,
        Status: 'ok',
        IANAStatus: 'Reserved',
        PostalInfo: [
          {
            Type: 'int',
            Address: { City: 'Mancora', CC: 'PE' },
          },
        ],
      });
    }
    if (payloads.length === 0) return;

    try {
      for (const p of payloads) {
        await createRegistrarMutate(p);
      }
      // Explicitly verify creations (up to 3 quick retries for eventual consistency)
      const verify = async (clid: string) => {
        for (let i = 0; i < 3; i++) {
          try {
            const r = await getRegistrarByClID(clid);
            if (r?.ClID === clid) return true;
          } catch { }
          await new Promise((res) => setTimeout(res, 250));
        }
        return false;
      };

      let ok9999 = true;
      let ok9998 = true;
      if (!has9999) ok9999 = await verify(clid9999);
      if (!has9998) ok9998 = await verify(clid9998);

      // Refresh per-registrar queries used by readiness checks
      if (!has9999) {
        await queryClient.invalidateQueries({ queryKey: ['registrar', clid9999] });
        await queryClient.refetchQueries({ queryKey: ['registrar', clid9999] });
      }
      if (!has9998) {
        await queryClient.invalidateQueries({ queryKey: ['registrar', clid9998] });
        await queryClient.refetchQueries({ queryKey: ['registrar', clid9998] });
      }

      if (ok9999 && ok9998) {
        toast.success('Created missing registrars');
      } else {
        const missing: string[] = [];
        if (!ok9999) missing.push(clid9999);
        if (!ok9998) missing.push(clid9998);
        toast.error(`Creation verification failed for: ${missing.join(', ')}`);
      }
    } catch (e: any) {
      const msg = e?.response?.data?.error || e?.message || 'Failed to create registrars';
      toast.error(msg);
    }
  }

  return (
    <DashboardLayout>
      <div className="space-y-8">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Escrow Imports</h1>
          <p className="text-muted-foreground">Upload escrow files, start imports, and view results</p>
        </div>

        <Tabs defaultValue="start" className="w-full">
          <TabsList>
            <TabsTrigger value="start">Upload & Start</TabsTrigger>
            <TabsTrigger value="runs">Runs</TabsTrigger>
          </TabsList>

          <TabsContent value="start">
            <div className="grid gap-6 md:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>1) Select TLD</CardTitle>
                  <CardDescription>Search and select the TLD to import into</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <Popover open={tldOpen} onOpenChange={setTldOpen}>
                    <PopoverTrigger asChild>
                      <Button variant="outline" role="combobox" className="w-full justify-between">
                        {tld || 'Select TLD'}
                        <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-96" align="start">
                      <div className="space-y-2">
                        <Input placeholder="Search tld..." value={tldQuery} onChange={(e) => setTldQuery(e.target.value)} />
                        <ScrollArea className="h-60">
                          <div className="space-y-1">
                            {/* Simple client-side hit: we call API when query changes via apiClient directly */}
                            <TLDSearchList query={tldQuery} onSelect={(name) => { setTld(name); setTldOpen(false); }} />
                          </div>
                        </ScrollArea>
                      </div>
                    </PopoverContent>
                  </Popover>
                  {/* Readiness checks appear once a TLD is selected */}
                  {tld && (
                    <div className="mt-2 space-y-2">
                      <div className="text-sm text-muted-foreground">Readiness checks</div>
                      <ul className="space-y-1">
                        <li className="flex items-center gap-2 text-sm">
                          {checksLoading ? (
                            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                          ) : escrowEnabled ? (
                            <CheckCircle2 className="h-4 w-4 text-green-600" />
                          ) : (
                            <XCircle className="h-4 w-4 text-red-600" />
                          )}
                          <span>Escrow import enabled for .{tld}</span>
                        </li>
                        <li className="flex items-center gap-2 text-sm">
                          {checksLoading ? (
                            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                          ) : has9999 ? (
                            <CheckCircle2 className="h-4 w-4 text-green-600" />
                          ) : (
                            <XCircle className="h-4 w-4 text-red-600" />
                          )}
                          <span>Registrar {clid9999} exists</span>
                        </li>
                        <li className="flex items-center gap-2 text-sm">
                          {checksLoading ? (
                            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                          ) : has9998 ? (
                            <CheckCircle2 className="h-4 w-4 text-green-600" />
                          ) : (
                            <XCircle className="h-4 w-4 text-red-600" />
                          )}
                          <span>Registrar {clid9998} exists</span>
                        </li>
                      </ul>
                      {!readyToUpload && !checksLoading && (
                        <div className="flex items-start gap-2 text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2">
                          <Info className="h-4 w-4 mt-0.5" />
                          <div>
                            Upload is disabled until all checks pass. Ensure escrow import is enabled and both system registrars exist.
                          </div>
                        </div>
                      )}
                      {/* Helper to create missing registrars */}
                      {!!tld && (!has9999 || !has9998) && (
                        <div className="pt-1">
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={handleCreateMissingRegistrars}
                            disabled={creatingRegistrar || roLoading}
                          >
                            {creatingRegistrar || roLoading ? (
                              <span className="inline-flex items-center gap-2"><Loader2 className="h-4 w-4 animate-spin" /> Creating missing registrars…</span>
                            ) : (
                              `Create missing registrars`
                            )}
                          </Button>
                        </div>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card className="md:col-span-2">
                <CardHeader>
                  <CardTitle>Optional: Registrar Overrides</CardTitle>
                  <CardDescription>
                    Map unknown registrars from the escrow file to system registrars manually.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <RegistrarOverrideForm value={registrarOverrides} onChange={setRegistrarOverrides} disabled={uploading || !readyToUpload} />
                </CardContent>
              </Card>

              <Card className="md:col-span-2">
                <CardHeader>
                  <CardTitle>2) Upload Escrow File</CardTitle>
                  <CardDescription>Upload the escrow artifact; we’ll start the workflow automatically</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex items-center justify-between p-2 border rounded-md bg-slate-50 dark:bg-slate-900/50">
                    <div className="flex items-center space-x-2">
                      <Switch id="import-mode" checked={importMode === 'direct_db'} onCheckedChange={(c) => setImportMode(c ? 'direct_db' : 'api')} />
                      <Label htmlFor="import-mode" className="cursor-pointer">Direct DB Import</Label>
                    </div>
                    <Popover>
                      <PopoverTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-6 w-6">
                          <Info className="h-4 w-4 text-muted-foreground" />
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className="w-80" align="end">
                        <div className="space-y-3">
                          <h4 className="font-medium leading-none">Import Mode Selection</h4>
                          <div className="text-sm text-muted-foreground space-y-2">
                            <p>
                              <span className="font-semibold text-foreground">Direct DB:</span> High-speed import that streams data directly to the database using bulk operations. Recommended for large initial migrations.
                            </p>
                            <p>
                              <span className="font-semibold text-foreground">API (Standard):</span> Imports via the Admin API. Slower (approx 50x) but strictly validates all logic through the application layer.
                            </p>
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>

                  <Input type="file" onChange={(e) => setFile(e.target.files?.[0] || null)} disabled={!tld || !readyToUpload} />
                  <div className="flex items-center gap-3">
                    <Button onClick={handleUpload} disabled={!file || uploading || !tld || !readyToUpload}>
                      {uploading ? (
                        <span className="inline-flex items-center gap-2"><Loader2 className="h-4 w-4 animate-spin" /> Uploading…</span>
                      ) : (
                        'Upload & Start'
                      )}
                    </Button>
                    {objectKey && (
                      <Badge variant="secondary">Object Key ready</Badge>
                    )}
                  </div>
                  {uploadStatus && (
                    <p className="text-xs text-red-500">{uploadStatus}</p>
                  )}
                  {startStatus && (
                    <p className="text-xs text-muted-foreground">{startStatus} {workflowLink?.url && (<a className="text-primary underline ml-2" href={workflowLink.url} target="_blank" rel="noreferrer">Open Workflow</a>)}</p>
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="runs">
            <Card>
              <CardHeader>
                <CardTitle>Recent Runs</CardTitle>
                <CardDescription>Filtered by TLD</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-2">
                  <Label>TLD</Label>
                  <Popover open={runsTldOpen} onOpenChange={setRunsTldOpen}>
                    <PopoverTrigger asChild>
                      <Button variant="outline" role="combobox" className="w-72 justify-between">
                        {filterTld || 'Select TLD'}
                        <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-96" align="start">
                      <div className="space-y-2">
                        <Input placeholder="Search tld..." value={runsTldQuery} onChange={(e) => setRunsTldQuery(e.target.value)} />
                        <ScrollArea className="h-60">
                          <div className="space-y-1">
                            <TLDSearchList query={runsTldQuery} onSelect={(name) => { setFilterTld(name); setRunsTldOpen(false); }} />
                          </div>
                        </ScrollArea>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>

                {error && <p className="text-sm text-red-600">{error}</p>}
                <div className="overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Date</TableHead>
                        <TableHead>Workflow ID</TableHead>
                        <TableHead>Stage</TableHead>
                        <TableHead>Analysis</TableHead>
                        <TableHead>DBs</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {loadingRuns ? (
                        <TableRow>
                          <TableCell colSpan={5}>Loading…</TableCell>
                        </TableRow>
                      ) : !filterTld.trim() ? (
                        null
                      ) : runs.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={5}>No runs</TableCell>
                        </TableRow>
                      ) : (
                        runs.map((r) => {
                          // Calculate Stage
                          let stage = 0;
                          if (r.importEventsUrl) stage = 5;
                          else if (r.stagedDbUrl) stage = 4;
                          else if (r.registrarMappingJsonUrl) stage = 3;
                          else if (r.sqliteDbUrl) stage = 2;
                          else if (r.analysisUrl) stage = 1;

                          return (
                            <TableRow key={r.runPrefix}>
                              <TableCell className="whitespace-nowrap text-xs">{r.date}</TableCell>
                              <TableCell className="whitespace-nowrap text-xs font-mono">
                                <a className="text-primary underline" href={r.url} target="_blank" rel="noreferrer">
                                  {r.workflowId}
                                </a>
                              </TableCell>
                              <TableCell>
                                <div className="flex items-center gap-2">
                                  <Badge variant={stage === 5 ? "default" : "outline"} className={stage === 5 ? "bg-green-600 hover:bg-green-700" : ""}>
                                    {stage === 5 ? "Ingested (5/5)" : stage === 4 ? "Staged (4/5)" : `Stage ${stage}/5`}
                                  </Badge>
                                  {stage === 4 && r.stagedDbKey && (
                                    <Button
                                      size="sm"
                                      variant="default"
                                      className="h-6 text-xs"
                                      onClick={async () => {
                                        try {
                                          toast.info('Triggering ingestion...');
                                          const res = await startEscrowIngestion(r.tld, r.stagedDbKey!);
                                          toast.success('Ingestion started', {
                                            action: {
                                              label: 'View Workflow',
                                              onClick: () => window.open(res.url, '_blank'),
                                            },
                                            duration: 10000,
                                          });
                                          // Optionally trigger refresh
                                          // load(); // requires un-scoping load function or relying on S3 eventual consistency (delay needed)
                                        } catch (e: any) {
                                          toast.error(e?.message || 'Failed to start ingestion');
                                        }
                                      }}
                                    >
                                      Run Ingestion
                                    </Button>
                                  )}
                                </div>
                              </TableCell>
                              <TableCell>
                                {r.analysisUrl ? (
                                  <a className="text-primary underline text-xs" href={r.analysisUrl} target="_blank" rel="noreferrer">Analysis</a>
                                ) : (
                                  <span className="text-muted-foreground text-xs">—</span>
                                )}
                              </TableCell>
                              <TableCell className="space-x-2">
                                {r.sqliteDbUrl ? (
                                  <a className="text-primary underline text-xs" href={r.sqliteDbUrl} target="_blank" rel="noreferrer">Ryde</a>
                                ) : null}
                                {r.stagedDbUrl ? (
                                  <a className="text-primary underline text-xs" href={r.stagedDbUrl} target="_blank" rel="noreferrer">Staged</a>
                                ) : null}
                                {!r.sqliteDbUrl && !r.stagedDbUrl && <span className="text-muted-foreground text-xs">—</span>}
                              </TableCell>
                            </TableRow>
                          );
                        })
                      )}
                    </TableBody>
                  </Table>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </DashboardLayout>
  );
}

function TLDSearchList({ query, onSelect }: { query: string; onSelect: (name: string) => void }) {
  const [recent, setRecent] = useState<string[]>([]);
  const debounced = useDebounce(query.trim(), 300);

  useEffect(() => {
    try {
      const raw = localStorage.getItem('recent_tlds');
      if (raw) setRecent(JSON.parse(raw));
    } catch { }
  }, []);

  const { data, isLoading, isError } = useQuery({
    queryKey: ['tld-search', debounced],
    queryFn: () => tldsApi.list({ name_like: debounced, pagesize: 20 }),
    enabled: debounced.length > 0,
  });

  function pick(name: string) {
    try {
      const next = [name, ...recent.filter((r) => r !== name)].slice(0, 10);
      setRecent(next);
      localStorage.setItem('recent_tlds', JSON.stringify(next));
    } catch { }
    onSelect(name);
  }

  const apiNames = (data?.Data || []).map((t: TLD) => t.Name);
  const results = Array.from(new Set([...(debounced ? [debounced] : []), ...apiNames]));
  const showRecent = !debounced && recent.length > 0;

  if (isLoading) return <div className="text-sm text-muted-foreground px-1 py-2">Searching…</div>;
  if (isError) return <div className="text-sm text-red-600 px-1 py-2">Failed to load TLDs</div>;

  if (showRecent) {
    return (
      <div className="flex flex-col gap-1">
        <div className="text-xs text-muted-foreground px-1">Recent</div>
        {recent.map((name) => (
          <Button key={name} variant="ghost" className="justify-start" onClick={() => pick(name)}>
            {name}
          </Button>
        ))}
      </div>
    );
  }

  if (results.length === 0) {
    return <div className="text-sm text-muted-foreground px-1 py-2">No matches. Type to enter a TLD</div>;
  }

  return (
    <div className="flex flex-col gap-1">
      {results.map((name) => (
        <Button key={name} variant="ghost" className="justify-start" onClick={() => pick(name)}>
          {name}
        </Button>
      ))}
    </div>
  );
}
