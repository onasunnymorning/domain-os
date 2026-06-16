import { useDomainCount } from "@/lib/hooks/useDomains";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Globe } from "lucide-react";
import { useRouter } from "next/navigation";
import { formatCompactNumber } from "@/lib/utils/numberUtils";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";

interface Props {
  clid: string;
}

export function RegistrarDomainCountWidget({ clid }: Props) {
  const { data, isLoading, error } = useDomainCount({ clid_equals: clid });
  const router = useRouter();

  return (
    <Card 
      className="cursor-pointer hover:bg-muted/50 transition-colors h-full" 
      onClick={() => router.push(`/domains?clid_equals=${encodeURIComponent(clid)}`)}
    >
      <CardContent className="flex items-center gap-4 p-6 h-full">
        <div className="p-3 bg-primary/10 text-primary rounded-full shrink-0">
          <Globe className="h-6 w-6" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-muted-foreground">Domains</p>
          {isLoading ? (
            <Skeleton className="h-8 w-16 mt-1" />
          ) : error ? (
            <p className="text-xl font-bold text-destructive">Error</p>
          ) : (
            <TooltipProvider delayDuration={300}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <h3 className="text-2xl font-bold cursor-help w-max">{data?.Count !== undefined ? formatCompactNumber(data.Count) : 0}</h3>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{data?.Count?.toLocaleString() ?? 0}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
