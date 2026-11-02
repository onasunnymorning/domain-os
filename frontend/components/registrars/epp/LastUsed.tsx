import { formatDistanceToNow } from "date-fns";

/** "3 hours ago from 203.0.113.9", or "never". */
export function LastUsed({ at, from }: { at?: string; from?: string }) {
  if (!at) return <span className="text-muted-foreground">never</span>;
  return (
    <span title={new Date(at).toLocaleString()}>
      {formatDistanceToNow(new Date(at))} ago
      {from && <span className="text-muted-foreground"> from {from}</span>}
    </span>
  );
}
