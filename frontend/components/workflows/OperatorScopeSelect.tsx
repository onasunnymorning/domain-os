'use client';

import { useState } from 'react';
import { Check, ChevronsUpDown } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { cn } from '@/lib/utils';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';

interface OperatorScopeSelectProps {
  /** Selected registry operator RyID, sent as the X-Tenant-ID header. */
  value: string;
  onChange: (ryid: string) => void;
  disabled?: boolean;
}

/**
 * Picks the operator scope for a tenant-scoped workflow launch.
 *
 * The scope is a header, never a body parameter: the server authorises the
 * launch against it and resolves TLD ownership from it (ADR-0006).
 */
export function OperatorScopeSelect({ value, onChange, disabled }: OperatorScopeSelectProps) {
  const [open, setOpen] = useState(false);
  const { data, isLoading } = useRegistryOperators({ pagesize: 100 });
  const operators = data?.Data ?? [];

  const selected = operators.find((o) => o.RyID === value);
  const label = selected ? `${selected.RyID} — ${selected.Name}` : value;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          disabled={disabled}
          className={cn('w-full justify-between font-normal', !value && 'text-muted-foreground')}
        >
          {value ? label : 'Select registry operator…'}
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
        <Command>
          <CommandInput placeholder="Search by RyID or name…" />
          <CommandList>
            <CommandEmpty>{isLoading ? 'Loading…' : 'No operators found.'}</CommandEmpty>
            <CommandGroup>
              {operators.map((op) => (
                <CommandItem
                  key={op.RyID}
                  value={`${op.RyID} ${op.Name}`}
                  onSelect={() => {
                    onChange(op.RyID);
                    setOpen(false);
                  }}
                >
                  <Check
                    className={cn(
                      'mr-2 h-4 w-4',
                      value === op.RyID ? 'opacity-100' : 'opacity-0'
                    )}
                  />
                  <span className="truncate">
                    {op.RyID} — {op.Name}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
