import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Library } from "lucide-react";
import { Badge } from "@/components/ui/badge";

interface Props {
  tlds: string[];
  isLoading: boolean;
  onClick: () => void;
}

export function RegistrarTLDCountWidget({ tlds, isLoading, onClick }: Props) {
  return (
    <Card className="cursor-pointer hover:bg-muted/50 transition-colors flex flex-col justify-center h-full" onClick={onClick}>
      <CardContent className="flex items-center gap-4 p-6">
        <div className="p-3 bg-primary/10 text-primary rounded-full shrink-0">
          <Library className="h-6 w-6" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-muted-foreground mb-1">Carrying</p>
          {isLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : tlds.length === 0 ? (
            <h3 className="text-2xl font-bold">0</h3>
          ) : (
            <div className="flex flex-wrap gap-1">
              {tlds.slice(0, 6).map((tld) => (
                <Badge key={tld} variant="secondary" className="font-mono">
                  .{tld}
                </Badge>
              ))}
              {tlds.length > 6 && (
                <Badge variant="outline" className="text-muted-foreground">
                  +{tlds.length - 6}
                </Badge>
              )}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
