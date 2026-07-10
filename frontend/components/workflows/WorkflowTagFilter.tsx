'use client';

import { X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

interface WorkflowTagFilterProps {
  allTags: string[];
  selectedTags: string[];
  onTagToggle: (tag: string) => void;
  onClearAll: () => void;
  tagCounts: Record<string, number>;
}

export function WorkflowTagFilter({
  allTags,
  selectedTags,
  onTagToggle,
  onClearAll,
  tagCounts,
}: WorkflowTagFilterProps) {
  const hasSelection = selectedTags.length > 0;

  return (
    <div className="flex items-center gap-2 overflow-x-auto pb-1">
      {allTags.map((tag) => {
        const isSelected = selectedTags.includes(tag);
        return (
          <Button
            key={tag}
            variant={isSelected ? 'default' : 'outline'}
            size="sm"
            onClick={() => onTagToggle(tag)}
            className={cn(
              'shrink-0 transition-all duration-150',
              isSelected && 'shadow-sm'
            )}
          >
            {tag}
            <span className={cn(
              'ml-1 tabular-nums',
              isSelected ? 'text-primary-foreground/70' : 'text-muted-foreground'
            )}>
              ({tagCounts[tag] ?? 0})
            </span>
          </Button>
        );
      })}

      {hasSelection && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onClearAll}
          className="text-muted-foreground shrink-0 gap-1"
        >
          <X className="size-3" />
          Clear All
        </Button>
      )}
    </div>
  );
}
