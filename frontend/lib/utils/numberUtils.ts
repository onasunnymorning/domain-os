export function formatCompactNumber(num: number | string | null | undefined): string {
  if (num === null || num === undefined) return "";
  
  const parsed = typeof num === "string" ? parseFloat(num) : num;
  if (isNaN(parsed)) return String(num);

  return new Intl.NumberFormat("en-US", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(parsed);
}
