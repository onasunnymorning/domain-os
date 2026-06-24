'use client';

import { useCallback, useEffect, useState, useRef } from 'react';
import { useRouter } from 'next/navigation';
import {
  Building2,
  FileText,
  Globe,
  Loader2,
  Search,
  Server,
  ServerOff,
  Users,
  Zap,
} from 'lucide-react';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import { Badge } from '@/components/ui/badge';
import { searchAll, type SearchResults, type DocSearchResult } from '@/lib/api/search';

interface GlobalSearchProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function GlobalSearch({ open, onOpenChange }: GlobalSearchProps) {
  const router = useRouter();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResults | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Reset state when dialog closes
  useEffect(() => {
    if (!open) {
      setQuery('');
      setResults(null);
      setIsLoading(false);
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
    }
  }, [open]);

  // Debounced search
  const handleSearch = useCallback((value: string) => {
    setQuery(value);

    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }

    if (!value.trim()) {
      setResults(null);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);

    debounceTimerRef.current = setTimeout(async () => {
      try {
        const data = await searchAll(value.trim());
        setResults(data);
      } catch (err) {
        console.error('Search failed:', err);
        setResults(null);
      } finally {
        setIsLoading(false);
      }
    }, 300);
  }, []);

  const handleSelect = useCallback(
    (path: string) => {
      onOpenChange(false);
      router.push(path);
    },
    [router, onOpenChange]
  );

  const hasResults =
    results &&
    (results.domains.length > 0 ||
      results.tlds.length > 0 ||
      results.registrars.length > 0 ||
      results.nndns.length > 0 ||
      results.registryOperators.length > 0 ||
      results.workflows.length > 0 ||
      results.documentation.length > 0);

  const showEmpty = query.trim() && !isLoading && results && !hasResults;

  // Track which groups have results for separator logic
  const hasPreviousGroup = (index: number) => {
    const groups = [
      results?.workflows.length ?? 0,
      results?.documentation.length ?? 0,
      results?.domains.length ?? 0,
      results?.tlds.length ?? 0,
      results?.registrars.length ?? 0,
      results?.nndns.length ?? 0,
      results?.registryOperators.length ?? 0,
    ];
    return groups.slice(0, index).some((count) => count > 0);
  };

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Global Search"
      description="Search across domains, TLDs, registrars, NNDNs, registry operators, and workflows"
    >
      <CommandInput
        placeholder="Search domains, TLDs, registrars, workflows..."
        value={query}
        onValueChange={handleSearch}
      />
      <CommandList>
        {isLoading && (
          <div className="flex items-center justify-center gap-2 py-6 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Searching...
          </div>
        )}

        {showEmpty && (
          <CommandEmpty>
            No results found for &ldquo;{query}&rdquo;
          </CommandEmpty>
        )}

        {!isLoading && !query.trim() && (
          <div className="py-6 text-center text-sm text-muted-foreground">
            Start typing to search across all entities...
          </div>
        )}

        {/* Workflows */}
        {results && results.workflows.length > 0 && (
          <CommandGroup heading="Workflows">
            {results.workflows.map((wf) => (
              <CommandItem
                key={`workflow-${wf.key}`}
                value={`workflow-${wf.key}-${wf.name}-${wf.tags.join('-')}`}
                onSelect={() =>
                  handleSelect(`/workflows?highlight=${encodeURIComponent(wf.key)}`)
                }
              >
                <Zap className="h-4 w-4 text-muted-foreground" />
                <span className="flex-1 truncate">{wf.name}</span>
                <span className="ml-1 hidden text-xs text-muted-foreground sm:inline">
                  {wf.description}
                </span>
                {wf.scheduled && (
                  <Badge variant="secondary" className="ml-2 text-xs font-normal">
                    Scheduled
                  </Badge>
                )}
                {wf.hasSignal && (
                  <Badge variant="outline" className="ml-1 text-xs font-normal">
                    HITL
                  </Badge>
                )}
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {/* Documentation */}
        {results && results.documentation.length > 0 && (
          <>
            {hasPreviousGroup(1) && <CommandSeparator />}
            <CommandGroup heading="Documentation">
              {results.documentation.map((doc, i) => (
                <CommandItem
                  key={`doc-${doc.workflowKey}-${i}`}
                  value={`doc-${doc.workflowKey}-${doc.heading}`}
                  onSelect={() =>
                    handleSelect(`/docs/${encodeURIComponent(doc.workflowKey)}`)
                  }
                >
                  <FileText className="h-4 w-4 text-muted-foreground" />
                  <span className="flex-1 truncate">{doc.heading}</span>
                  <Badge variant="outline" className="ml-2 shrink-0 text-xs font-normal">
                    {doc.workflowName}
                  </Badge>
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}

        {/* Domains */}
        {results && results.domains.length > 0 && (
          <>
            {hasPreviousGroup(2) && <CommandSeparator />}
            <CommandGroup heading="Domains">
              {results.domains.map((domain) => (
                <CommandItem
                  key={`domain-${domain.Name}`}
                  value={`domain-${domain.Name}`}
                  onSelect={() =>
                    handleSelect(`/domains/${encodeURIComponent(domain.Name)}`)
                  }
                >
                  <Server className="h-4 w-4 text-muted-foreground" />
                  <span className="flex-1 truncate">{domain.Name}</span>
                  {domain.ClID && (
                    <Badge variant="outline" className="ml-2 text-xs font-normal">
                      {domain.ClID}
                    </Badge>
                  )}
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}

        {/* TLDs */}
        {results && results.tlds.length > 0 && (
          <>
            {hasPreviousGroup(3) && <CommandSeparator />}
            <CommandGroup heading="TLDs">
              {results.tlds.map((tld) => (
                <CommandItem
                  key={`tld-${tld.Name}`}
                  value={`tld-${tld.Name}`}
                  onSelect={() =>
                    handleSelect(`/tlds/${encodeURIComponent(tld.Name)}`)
                  }
                >
                  <Globe className="h-4 w-4 text-muted-foreground" />
                  <span className="flex-1 truncate">.{tld.Name}</span>
                  {tld.Type && (
                    <Badge
                      variant="secondary"
                      className="ml-2 text-xs font-normal"
                    >
                      {tld.Type}
                    </Badge>
                  )}
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}

        {/* Registrars */}
        {results && results.registrars.length > 0 && (
          <>
            {hasPreviousGroup(4) && <CommandSeparator />}
            <CommandGroup heading="Registrars">
              {results.registrars.map((registrar) => (
                <CommandItem
                  key={`registrar-${registrar.ClID}`}
                  value={`registrar-${registrar.ClID}-${registrar.Name}`}
                  onSelect={() =>
                    handleSelect(
                      `/registrars/${encodeURIComponent(registrar.ClID)}`
                    )
                  }
                >
                  <Users className="h-4 w-4 text-muted-foreground" />
                  <span className="flex-1 truncate">{registrar.Name}</span>
                  <Badge
                    variant="outline"
                    className="ml-2 text-xs font-normal"
                  >
                    {registrar.ClID}
                  </Badge>
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}

        {/* NNDNs */}
        {results && results.nndns.length > 0 && (
          <>
            {hasPreviousGroup(5) && <CommandSeparator />}
            <CommandGroup heading="NNDNs">
              {results.nndns.map((nndn) => (
                <CommandItem
                  key={`nndn-${nndn.Name}`}
                  value={`nndn-${nndn.Name}`}
                  onSelect={() => handleSelect('/nndns')}
                >
                  <ServerOff className="h-4 w-4 text-muted-foreground" />
                  <span className="flex-1 truncate">{nndn.Name}</span>
                  {nndn.Reason && (
                    <Badge
                      variant="secondary"
                      className="ml-2 text-xs font-normal"
                    >
                      {nndn.Reason}
                    </Badge>
                  )}
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}

        {/* Registry Operators */}
        {results && results.registryOperators.length > 0 && (
          <>
            {hasPreviousGroup(6) && <CommandSeparator />}
            <CommandGroup heading="Registry Operators">
              {results.registryOperators.map((ro) => (
                <CommandItem
                  key={`ro-${ro.RyID}`}
                  value={`ro-${ro.RyID}-${ro.Name}`}
                  onSelect={() =>
                    handleSelect(
                      `/registry-operators/${encodeURIComponent(ro.RyID)}`
                    )
                  }
                >
                  <Building2 className="h-4 w-4 text-muted-foreground" />
                  <span className="flex-1 truncate">{ro.Name}</span>
                  <Badge
                    variant="outline"
                    className="ml-2 text-xs font-normal"
                  >
                    {ro.RyID}
                  </Badge>
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}
      </CommandList>
    </CommandDialog>
  );
}
