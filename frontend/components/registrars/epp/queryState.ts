import { eppErrorMessage } from "@/lib/hooks/useEPP";

/** One sentence for a query that could not be read: what, and the API's reason. */
export function eppQueryErrorText(what: string, error: unknown): string {
  return `Could not read ${what}: ${eppErrorMessage(error, "the request failed")}`;
}
