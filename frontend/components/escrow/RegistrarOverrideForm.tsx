'use client';

import { useState, useEffect } from 'react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { ChevronDown, Plus, Trash2, Loader2, List, Code, AlertCircle } from 'lucide-react';
import { useRegistrars } from '@/lib/hooks/useRegistrars';
import { useDebounce } from '@/lib/hooks/useDebounce';
import { RegistrarListItem } from '@/lib/types/registrar';
import { Alert, AlertDescription } from '@/components/ui/alert';

interface RegistrarOverrideFormProps {
    value: Record<string, string>;
    onChange: (value: Record<string, string>) => void;
    disabled?: boolean;
}

interface OverrideRow {
    id: string; // internal unique id for React keys
    escrowName: string;
    systemClID: string;
}

export function RegistrarOverrideForm({ value, onChange, disabled }: RegistrarOverrideFormProps) {
    const [view, setView] = useState<'list' | 'json'>('list');
    const [jsonInput, setJsonInput] = useState(() => JSON.stringify(value, null, 2));
    const [jsonError, setJsonError] = useState<string | null>(null);

    // Convert Record to Array for internal state
    const [rows, setRows] = useState<OverrideRow[]>(() => {
        return Object.entries(value).map(([escrowName, systemClID], index) => ({
            id: `init-${index}-${Date.now()}`,
            escrowName,
            systemClID,
        }));
    });

    // Sync rows -> parent value and JSON input (when in list view)
    useEffect(() => {
        const nextValue: Record<string, string> = {};
        rows.forEach((row) => {
            if (row.escrowName.trim() && row.systemClID.trim()) {
                nextValue[row.escrowName.trim()] = row.systemClID.trim();
            }
        });

        // Emit to parent
        onChange(nextValue);

        // Update JSON input when rows change, but only if we are in list view
        // to avoid overwriting user edits in JSON view before they are finished.
        if (view === 'list') {
            setJsonInput(JSON.stringify(nextValue, null, 2));
            setJsonError(null);
        }
    }, [rows, onChange, view]);

    function addRow() {
        setRows([...rows, { id: `new-${Date.now()}`, escrowName: '', systemClID: '' }]);
    }

    function removeRow(id: string) {
        setRows(rows.filter((r) => r.id !== id));
    }

    function updateRow(id: string, field: 'escrowName' | 'systemClID', val: string) {
        setRows(
            rows.map((r) => {
                if (r.id === id) return { ...r, [field]: val };
                return r;
            })
        );
    }

    function handleJsonChange(val: string) {
        setJsonInput(val);
        try {
            if (!val.trim()) {
                setRows([]);
                setJsonError(null);
                return;
            }
            const parsed = JSON.parse(val);
            if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
                throw new Error('Input must be a JSON object { "escrowName": "systemClID" }');
            }

            // Valid JSON, convert to rows
            const newRows: OverrideRow[] = Object.entries(parsed).map(([k, v], index) => ({
                id: `json-${index}-${Date.now()}`,
                escrowName: k,
                systemClID: String(v),
            }));
            setRows(newRows);
            setJsonError(null);
        } catch (e: any) {
            setJsonError(e.message);
        }
    }

    return (
        <div className="space-y-4">
            <Tabs value={view} onValueChange={(v) => setView(v as 'list' | 'json')} className="w-full">
                <div className="flex items-center justify-between mb-2">
                    <TabsList>
                        <TabsTrigger value="list" className="flex items-center gap-2">
                            <List className="h-4 w-4" /> List View
                        </TabsTrigger>
                        <TabsTrigger value="json" className="flex items-center gap-2">
                            <Code className="h-4 w-4" /> JSON View
                        </TabsTrigger>
                    </TabsList>
                </div>

                <TabsContent value="list" className="space-y-4 mt-0">
                    <div className="rounded-md border">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead className="w-[45%]">Escrow Registrar Name</TableHead>
                                    <TableHead className="w-[45%]">System Registrar (ClID)</TableHead>
                                    <TableHead className="w-[10%]"></TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {rows.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={3} className="text-center text-muted-foreground">
                                            No overrides defined.
                                        </TableCell>
                                    </TableRow>
                                ) : (
                                    rows.map((row) => (
                                        <TableRow key={row.id}>
                                            <TableCell>
                                                <Input
                                                    placeholder="e.g. GoDaddy, LLC"
                                                    value={row.escrowName}
                                                    onChange={(e) => updateRow(row.id, 'escrowName', e.target.value)}
                                                    disabled={disabled}
                                                />
                                            </TableCell>
                                            <TableCell>
                                                <RegistrarCombobox
                                                    value={row.systemClID}
                                                    onChange={(val) => updateRow(row.id, 'systemClID', val)}
                                                    disabled={disabled}
                                                />
                                            </TableCell>
                                            <TableCell>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    onClick={() => removeRow(row.id)}
                                                    disabled={disabled}
                                                    className="text-red-500 hover:text-red-600 hover:bg-red-50"
                                                >
                                                    <Trash2 className="h-4 w-4" />
                                                </Button>
                                            </TableCell>
                                        </TableRow>
                                    ))
                                )}
                            </TableBody>
                        </Table>
                    </div>
                    <Button variant="outline" size="sm" onClick={addRow} disabled={disabled}>
                        <Plus className="mr-2 h-4 w-4" /> Add Override
                    </Button>
                </TabsContent>

                <TabsContent value="json" className="mt-0 space-y-2">
                    <Textarea
                        placeholder='{ "Escrow Name": "system-clid" }'
                        className="font-mono min-h-[300px] text-sm"
                        value={jsonInput}
                        onChange={(e) => handleJsonChange(e.target.value)}
                        disabled={disabled}
                    />
                    {jsonError && (
                        <Alert variant="destructive" className="py-2">
                            <AlertCircle className="h-4 w-4" />
                            <AlertDescription className="text-xs">
                                Invalid JSON: {jsonError}
                            </AlertDescription>
                        </Alert>
                    )}
                    <p className="text-xs text-muted-foreground">
                        Paste a JSON object where keys are the names in the escrow file and values are the system Registrar ClIDs.
                    </p>
                </TabsContent>
            </Tabs>
        </div>
    );
}

function RegistrarCombobox({
    value,
    onChange,
    disabled,
}: {
    value: string;
    onChange: (val: string) => void;
    disabled?: boolean;
}) {
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState('');
    const debounced = useDebounce(query, 300);

    // If value is set, we might want to resolve it to a name for better display?
    // For now just show the ClID (value) or allow search.
    // The user requirement says "drop down field with incorporated search to select a cl_id".

    const { data, isLoading } = useRegistrars(
        debounced ? { name_like: debounced, pagesize: 50 } : { pagesize: 50 }
    );

    const items = data?.Data || [];

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    className="w-full justify-between font-normal"
                    disabled={disabled}
                >
                    {value || 'Select registrar...'}
                    <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[300px] p-0" align="start">
                <div className="p-2 border-b">
                    <Input
                        placeholder="Search registrar..."
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        className="h-8"
                    />
                </div>
                <ScrollArea className="h-[200px]">
                    {isLoading ? (
                        <div className="flex items-center justify-center p-4 text-muted-foreground">
                            <Loader2 className="h-4 w-4 animate-spin mr-2" /> Loading...
                        </div>
                    ) : items.length === 0 ? (
                        <div className="p-4 text-sm text-center text-muted-foreground">
                            No registrars found.
                        </div>
                    ) : (
                        <div className="p-1">
                            {items.map((r: RegistrarListItem) => (
                                <Button
                                    key={r.ClID}
                                    variant="ghost"
                                    className="w-full justify-start text-left h-auto py-2"
                                    onClick={() => {
                                        onChange(r.ClID);
                                        setOpen(false);
                                    }}
                                >
                                    <div className="flex flex-col items-start gap-1 overflow-hidden">
                                        <span className="font-medium truncate w-full">{r.Name}</span>
                                        <span className="text-xs text-muted-foreground truncate w-full">{r.ClID}</span>
                                    </div>
                                </Button>
                            ))}
                        </div>
                    )}
                </ScrollArea>
            </PopoverContent>
        </Popover>
    );
}
