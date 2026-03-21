import React from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';

export interface ColumnDef<T> {
  header: React.ReactNode;
  accessor?: keyof T;
  cell?: (row: T) => React.ReactNode;
  className?: string; // For alignment on table headers/cells
}

export interface DataTableProps<T> {
  title: string;
  description?: string;
  columns: ColumnDef<T>[];
  data: T[];
  keyExtractor: (row: T) => string | number;
  isLoading?: boolean;
  onRowClick?: (row: T) => void;
  emptyState?: React.ReactNode;
  error?: React.ReactNode;
}

export function DataTable<T>({
  title,
  description,
  columns,
  data,
  keyExtractor,
  isLoading,
  onRowClick,
  emptyState,
  error
}: DataTableProps<T>) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>
        {error && (
          <div className="rounded-md bg-destructive/15 p-4 text-sm text-destructive mb-6">
            {error}
          </div>
        )}

        {isLoading ? (
          <div className="space-y-2">
            {[1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        ) : data.length === 0 ? (
          emptyState || (
            <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground">
              No data found.
            </div>
          )
        ) : (
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  {columns.map((col, index) => (
                    <TableHead key={index} className={col.className}>
                      {col.header}
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.map((row) => (
                  <TableRow
                    key={keyExtractor(row)}
                    className={onRowClick ? "cursor-pointer hover:bg-muted/50 transition-colors" : ""}
                    onClick={() => onRowClick?.(row)}
                  >
                    {columns.map((col, index) => (
                      <TableCell 
                        key={index} 
                        className={col.className}
                      >
                        {col.cell ? col.cell(row) : (col.accessor ? String(row[col.accessor] ?? '') : null)}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
