"use client";

import { useDomainDNS } from "@/lib/hooks/useDomains";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useState } from "react";
import { Terminal } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { CopyButton } from "@/components/ui/copy-button";

interface DnsLookupModalProps {
  domainName: string;
  trigger?: React.ReactNode;
}

export function DnsLookupModal({ domainName, trigger }: DnsLookupModalProps) {
  const [open, setOpen] = useState(false);
  // Only fetch data when the modal is open
  const { data: hosts, isLoading, error } = useDomainDNS(domainName, open);



  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger || (
          <Button variant="outline" size="sm">
            <Terminal className="w-4 h-4 mr-2" />
            DNS Lookup
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="font-mono text-lg flex items-center gap-2">
            <Terminal className="w-5 h-5 text-muted-foreground" />
            dig ns {domainName}
          </DialogTitle>
          <DialogDescription className="sr-only">Live NS records for {domainName}</DialogDescription>
        </DialogHeader>

        <div className="relative rounded-md overflow-hidden border mt-2">
          {isLoading && (
            <div className="p-6 space-y-3 bg-muted/30">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-4 w-5/6" />
            </div>
          )}

          {error && (
            <div className="p-6 text-sm text-red-500 bg-red-500/10">
              Failed to resolve DNS records. Please try again.
            </div>
          )}

          {!isLoading && !error && hosts !== undefined && (
            <div className="bg-muted text-xs p-4 pt-10 overflow-x-auto group min-h-[140px]">
              <CopyButton
                value={hosts.join("\n")}
                variant="none"
                className="absolute top-2 right-2 p-1.5 z-10 text-muted-foreground hover:text-foreground bg-background/50 hover:bg-background rounded-md transition-colors opacity-0 group-hover:opacity-100"
                tooltip="Copy text"
                iconClassName="h-4 w-4"
              />
              {hosts.length === 0 ? (
                <div className="text-muted-foreground italic h-full flex items-center justify-center">No NS records found</div>
              ) : (
                <pre><code>{hosts.join("\n")}</code></pre>
              )}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
