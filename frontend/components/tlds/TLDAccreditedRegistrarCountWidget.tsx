import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Building2 } from "lucide-react";

interface Props {
  count: number;
  isLoading: boolean;
  onClick?: () => void;
}

export function TLDAccreditedRegistrarCountWidget({ count, isLoading, onClick }: Props) {
  return (
    <Card className={onClick ? "cursor-pointer hover:bg-muted/50 transition-colors" : ""} onClick={onClick}>
      <CardContent className="flex items-center gap-4 p-6">
        <div className="p-3 bg-primary/10 text-primary rounded-full">
          <Building2 className="h-6 w-6" />
        </div>
        <div>
          <p className="text-sm font-medium text-muted-foreground">Accredited Registrars</p>
          {isLoading ? (
            <Skeleton className="h-8 w-16 mt-1" />
          ) : (
            <h3 className="text-2xl font-bold">{count.toLocaleString()}</h3>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
