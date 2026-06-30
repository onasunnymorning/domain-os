"use client";

import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Search, X } from "lucide-react";
import { RegistrarStatus } from "@/lib/types/registrar";

interface TLDConfig {
  Name: string;
}

interface RegistrarSearchFiltersProps {
  searchQuery: string;
  setSearchQuery: (val: string) => void;
  ianaIdQuery: string;
  setIanaIdQuery: (val: string) => void;
  statusFilter?: string;
  setStatusFilter?: (val: string) => void;
  tldFilter?: string;
  setTldFilter?: (val: string) => void;
  tlds?: TLDConfig[];
  sortBy?: string;
  setSortBy?: (val: string) => void;
  className?: string;
  placeholder?: string;
}

export function RegistrarSearchFilters({
  searchQuery,
  setSearchQuery,
  ianaIdQuery,
  setIanaIdQuery,
  statusFilter,
  setStatusFilter,
  tldFilter,
  setTldFilter,
  tlds,
  sortBy,
  setSortBy,
  className = "",
  placeholder = "Search registrars..."
}: RegistrarSearchFiltersProps) {
  const hasActiveFilters =
    searchQuery !== "" ||
    ianaIdQuery !== "" ||
    (statusFilter !== undefined && statusFilter !== "all") ||
    (tldFilter !== undefined && tldFilter !== "all");

  const handleReset = () => {
    setSearchQuery("");
    setIanaIdQuery("");
    if (setStatusFilter) setStatusFilter("all");
    if (setTldFilter) setTldFilter("all");
    if (setSortBy) setSortBy("domains_desc");
  };

  return (
    <div className={`flex flex-wrap items-center gap-3 ${className}`}>
      <div className="flex-1 min-w-[200px]">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={placeholder}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10 pr-9 h-9"
            title="Search registrars by name or Client ID (ClID)"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery("")}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      <div className="w-28 relative">
        <Input
          placeholder="IANA ID"
          inputMode="numeric"
          value={ianaIdQuery}
          onChange={(e) => setIanaIdQuery(e.target.value)}
          className="h-9 pr-8"
          title="Filter by IANA Registrar ID (numerical GURID)"
        />
        {ianaIdQuery && (
          <button
            onClick={() => setIanaIdQuery("")}
            className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            type="button"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {statusFilter !== undefined && setStatusFilter && (
        <div className="w-36">
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="h-9">
              <SelectValue placeholder="All Statuses" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Statuses</SelectItem>
              <SelectItem value={RegistrarStatus.OK}>OK</SelectItem>
              <SelectItem value={RegistrarStatus.Readonly}>Readonly</SelectItem>
              <SelectItem value={RegistrarStatus.Terminated}>Terminated</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}

      {tldFilter !== undefined && setTldFilter && (
        <div className="w-36">
          <Select value={tldFilter} onValueChange={setTldFilter}>
            <SelectTrigger className="h-9">
              <SelectValue placeholder="All TLDs" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All TLDs</SelectItem>
              {tlds?.map((tld) => (
                <SelectItem key={tld.Name} value={tld.Name}>
                  .{tld.Name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {sortBy !== undefined && setSortBy && (
        <div className="w-44">
          <Select value={sortBy} onValueChange={setSortBy}>
            <SelectTrigger className="h-9">
              <SelectValue placeholder="Sort By" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="clid_asc">Client ID (A-Z)</SelectItem>
              <SelectItem value="domains_desc">Domains (High to Low)</SelectItem>
              <SelectItem value="domains_asc">Domains (Low to High)</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}

      {hasActiveFilters && (
        <Button
          variant="ghost"
          size="sm"
          onClick={handleReset}
          className="h-9 px-3 text-muted-foreground hover:text-foreground shrink-0 gap-1.5"
          type="button"
        >
          <X className="h-4 w-4" />
          Clear filters
        </Button>
      )}
    </div>
  );
}
