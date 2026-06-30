import React from 'react';
import { Search, X } from 'lucide-react';
import { Input } from '@/components/ui/input';

export interface SearchFilterProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string; // allow overriding max-w etc
}

export function SearchFilter({ 
  value, 
  onChange, 
  placeholder = "Search...", 
  className = "relative flex-1 max-w-sm" 
}: SearchFilterProps) {
  return (
    <div className={className}>
      <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="pl-9 pr-8"
      />
      {value && (
        <button
          onClick={() => onChange("")}
          className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          type="button"
        >
          <X className="h-4 w-4" />
        </button>
      )}
    </div>
  );
}
