import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Library } from "lucide-react";

interface Props {
  count: number;
  isLoading: boolean;
  onClick: () => void;
}

export function RegistrarTLDCountWidget({ count, isLoading, onClick }: Props) {
  return (
    <Card className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={onClick}>
      <CardContent className="flex items-center gap-4 p-6">
        <div className="p-3 bg-primary/10 text-primary rounded-full">
          <Library className="h-6 w-6" />
        </div>
        <div>
          <p className="text-sm font-medium text-muted-foreground">Carrying TLDs</p>
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
