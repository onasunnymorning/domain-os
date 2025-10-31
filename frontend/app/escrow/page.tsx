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
import { ChevronDown, Loader2 } from 'lucide-react';
import { presignUpload, startEscrowImport, listEscrowImports, EscrowRunItem } from '@/lib/api/escrow';
import { apiClient } from '@/lib/api/client';
import { useQuery } from '@tanstack/react-query';
import { tldsApi, TLD } from '@/lib/api/tlds';
import { useDebounce } from '@/lib/hooks/useDebounce';

export default function EscrowPage() {
  // Upload & start state
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [objectKey, setObjectKey] = useState<string>('');
  const [tld, setTld] = useState<string>('');
  const [startStatus, setStartStatus] = useState<string>('');
  const [uploadStatus, setUploadStatus] = useState<string>('');
  const [workflowLink, setWorkflowLink] = useState<{ id: string; url?: string } | null>(null);
  const [tldOpen, setTldOpen] = useState(false);
  const [tldQuery, setTldQuery] = useState('');

  // Runs state
  const [filterTld, setFilterTld] = useState<string>('example');
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
        const res = await startEscrowImport({ tld, objectKey: presign.objectKey, options: { mapRegistrars: true } });
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
            const res = await startEscrowImport({ tld, objectKey: data.objectKey, options: { mapRegistrars: true } });
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
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>2) Upload Escrow File</CardTitle>
                  <CardDescription>Upload the escrow artifact; we’ll start the workflow automatically</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <Input type="file" onChange={(e) => setFile(e.target.files?.[0] || null)} disabled={!tld} />
                  <div className="flex items-center gap-3">
                    <Button onClick={handleUpload} disabled={!file || uploading || !tld}>
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
                <div className="flex items-end gap-3">
                  <div className="grid gap-2">
                    <Label htmlFor="filter-tld">TLD</Label>
                    <Input id="filter-tld" value={filterTld} onChange={(e) => setFilterTld(e.target.value)} placeholder="example" />
                  </div>
                  <Button variant="outline" onClick={() => setFilterTld(filterTld)} disabled={!filterTld.trim()}>
                    Refresh
                  </Button>
                </div>

                {error && <p className="text-sm text-red-600">{error}</p>}
                <div className="overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Date</TableHead>
                        <TableHead>Workflow ID</TableHead>
                        <TableHead>Summary</TableHead>
                        <TableHead>Run Report</TableHead>
                        <TableHead>Analysis</TableHead>
                        <TableHead>Mapping</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {loadingRuns ? (
                        <TableRow>
                          <TableCell colSpan={6}>Loading…</TableCell>
                        </TableRow>
                      ) : runs.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={6}>No runs</TableCell>
                        </TableRow>
                      ) : (
                        runs.map((r) => (
                          <TableRow key={r.runPrefix}>
                            <TableCell className="whitespace-nowrap">{r.date}</TableCell>
                            <TableCell className="whitespace-nowrap">
                              <a className="text-primary underline" href={r.url} target="_blank" rel="noreferrer">
                                {r.workflowId}
                              </a>
                            </TableCell>
                            <TableCell>
                              {r.summaryUrl ? (
                                <a className="text-primary underline" href={r.summaryUrl} target="_blank" rel="noreferrer">summary.json</a>
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </TableCell>
                            <TableCell>
                              {r.runReportUrl ? (
                                <a className="text-primary underline" href={r.runReportUrl} target="_blank" rel="noreferrer">run-report.json</a>
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </TableCell>
                            <TableCell>
                              {r.analysisUrl ? (
                                <a className="text-primary underline" href={r.analysisUrl} target="_blank" rel="noreferrer">analysis.json</a>
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </TableCell>
                            <TableCell className="whitespace-nowrap">
                              {r.registrarMappingUrl ? (
                                <a className="text-primary underline" href={r.registrarMappingUrl} target="_blank" rel="noreferrer">CSV</a>
                              ) : (
                                <span className="text-muted-foreground">CSV —</span>
                              )}
                              {' '}
                              {r.registrarMappingJsonUrl ? (
                                <a className="text-primary underline" href={r.registrarMappingJsonUrl} target="_blank" rel="noreferrer">JSON</a>
                              ) : (
                                <span className="text-muted-foreground">JSON —</span>
                              )}
                            </TableCell>
                          </TableRow>
                        ))
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
    } catch {}
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
    } catch {}
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
