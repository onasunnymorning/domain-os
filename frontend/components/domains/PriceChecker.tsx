"use client";

import { useMemo, useState } from "react";
import { DomainDetail } from "@/lib/types/domain";
import { useDomainQuotes } from "@/lib/hooks/useDomains";
import { useTLDRegistrars } from "@/lib/hooks/useAccreditations";
import { Repeat, Copy, ChevronDown, Check, Search } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogDescription } from "@/components/ui/dialog";

interface Props {
  domain: DomainDetail;
  trigger?: React.ReactNode;
}

function formatMoney(moneyObj: any) {
  if (!moneyObj) return "-";
  const amt = typeof moneyObj.amount === 'number' ? moneyObj.amount / 100 : 0;
  const cur = moneyObj.currency?.code || moneyObj.currency || "USD";
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: cur }).format(amt);
}

const TRANSACTIONS = ["registration", "renewal", "transfer", "restore", "auto_renewal"];

export function PriceChecker({ domain, trigger }: Props) {
  const [open, setOpen] = useState(false);
  const [showRaw, setShowRaw] = useState(false);
  const [selectedClID, setSelectedClID] = useState(domain.ClID);
  const [popoverOpen, setPopoverOpen] = useState(false);
  const [search, setSearch] = useState("");

  const { data: registrarsData } = useTLDRegistrars(open ? (domain.TLDName || "") : "", { pagesize: 1000 });
  const registrars = registrarsData?.Data || [];

  const filteredRegistrars = useMemo(() => {
    return registrars.filter(r => 
      r.ClID.toLowerCase().includes(search.toLowerCase()) || 
      r.Name.toLowerCase().includes(search.toLowerCase())
    );
  }, [registrars, search]);

  const payloads = useMemo(() => {
    if (!domain?.Name || !selectedClID || !open) return [];
    return TRANSACTIONS.map(t => ({
      DomainName: domain.Name,
      TransactionType: t,
      Currency: "USD",
      Years: 1,
      ClID: selectedClID,
    }));
  }, [domain.Name, selectedClID, open]);

  const results = useDomainQuotes(payloads);
  const isLoading = results.some(r => r.isLoading);
  
  // Exclude errors and undefined data
  const quotes = results.map(r => r.data).filter(Boolean);

  const simplifiedQuotes = useMemo(() => {
    return quotes.reduce((acc, q) => {
      if (q && q.TransactionType) {
        acc[q.TransactionType] = { 
          Price: q.Price, 
          Class: q.Class, 
          Fees: q.Fees || [] 
        };
      }
      return acc;
    }, {} as Record<string, any>);
  }, [quotes]);

  const selectedRegistrar = registrars.find(r => r.ClID === selectedClID);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger}
      </DialogTrigger>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>Price Checker</DialogTitle>
          <DialogDescription className="text-xs">Select registrar to view pricing</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 pt-2 max-w-[340px] mx-auto w-full">
          <Popover open={popoverOpen} onOpenChange={setPopoverOpen}>
            <PopoverTrigger asChild>
              <Button variant="outline" className="w-full justify-between font-normal text-sm" aria-label="Select Registrar">
                <span className="truncate">
                  {selectedRegistrar ? `${selectedRegistrar.Name} (${selectedRegistrar.ClID})` : selectedClID}
                </span>
                <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[300px] p-0" align="start">
              <div className="flex items-center border-b px-3">
                <Search className="mr-2 h-4 w-4 shrink-0 opacity-50" />
                <input
                  className="flex h-10 w-full rounded-md bg-transparent py-3 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50"
                  placeholder="Search registrar..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <div className="max-h-[300px] overflow-y-auto p-1">
                {filteredRegistrars.length === 0 ? (
                  <div className="py-6 text-center text-sm text-muted-foreground">No registrars found.</div>
                ) : (
                  filteredRegistrars.map((r) => (
                    <div
                      key={r.ClID}
                      className={cn(
                        "relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground",
                        selectedClID === r.ClID ? "bg-accent text-accent-foreground" : ""
                      )}
                      onClick={() => {
                        setSelectedClID(r.ClID);
                        setPopoverOpen(false);
                        setSearch("");
                      }}
                    >
                      <Check
                        className={cn(
                          "mr-2 h-4 w-4",
                          selectedClID === r.ClID ? "opacity-100" : "opacity-0"
                        )}
                      />
                      <span className="truncate">{r.Name} <span className="text-muted-foreground text-xs">({r.ClID})</span></span>
                    </div>
                  ))
                )}
              </div>
            </PopoverContent>
          </Popover>

          <div className="rounded-md border overflow-hidden relative min-h-[200px] flex flex-col">
            <button
              onClick={() => setShowRaw(!showRaw)}
              className="absolute top-2 right-2 p-1.5 z-10 text-muted-foreground hover:text-foreground bg-background/50 hover:bg-background rounded-md transition-colors"
              title={showRaw ? "Show Visual Dashboard" : "Show Raw JSON"}
              aria-label="Toggle raw JSON"
            >
              <Repeat className="w-4 h-4" />
            </button>

            {isLoading ? (
              <div className="flex-1 flex justify-center items-center text-muted-foreground text-sm py-8">
                Loading quotes...
              </div>
            ) : !quotes || quotes.length === 0 ? (
              <div className="flex-1 flex justify-center items-center text-muted-foreground text-sm py-8">
                No quotes available for this registrar.
              </div>
            ) : showRaw ? (
              <div className="bg-muted text-xs p-4 pt-10 overflow-x-auto group min-h-[140px] relative">
                <button
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(JSON.stringify(simplifiedQuotes, null, 2));
                    } catch {}
                  }}
                  className="absolute top-2 right-10 p-1.5 z-10 text-muted-foreground hover:text-foreground bg-background/50 hover:bg-background rounded-md transition-colors opacity-0 group-hover:opacity-100"
                  title="Copy JSON array"
                  aria-label="Copy JSON"
                >
                  <Copy className="h-4 w-4" />
                </button>
                <pre className="h-full"><code>{JSON.stringify(simplifiedQuotes, null, 2)}</code></pre>
              </div>
            ) : (
              <div className="overflow-x-auto flex-1">
                <table className="w-full text-sm text-left whitespace-nowrap">
                  <thead className="bg-muted/50 border-b">
                    <tr>
                      <th className="px-3 py-2 font-medium text-center">Tx</th>
                      <th className="px-3 py-2 font-medium text-center">Price</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    {quotes.map((quote, idx) => (
                      <tr key={quote?.TransactionType || idx}>
                        <td className="px-3 py-2 font-medium bg-muted/10 capitalize truncate text-center">
                          {quote?.TransactionType?.replace(/_/g, ' ')}
                        </td>
                        <td className="px-3 py-2 bg-muted/10 font-medium text-center">
                          <div className="flex items-center justify-center gap-1.5">
                            {quote?.Class && quote.Class !== "standard" && (
                              <span className="text-[10px] bg-amber-100 text-amber-800 border border-amber-200 px-1 rounded truncate max-w-[60px]" title={quote.Class}>
                                {quote.Class}
                              </span>
                            )}
                            <span>{formatMoney(quote?.Price)}</span>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
