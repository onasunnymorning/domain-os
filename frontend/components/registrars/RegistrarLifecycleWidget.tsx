import { Registrar } from "@/lib/types/registrar";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { format, formatDistanceToNowStrict } from "date-fns";

interface Props {
  data: Registrar;
}

export function RegistrarLifecycleWidget({ data }: Props) {
  return (
    <Card>
      <CardContent className="space-y-6 pt-6">
        <div className="flex items-center gap-2 pb-2 flex-wrap">
          {data.CreatedAt && (
            <>
              <div className="text-lg font-semibold text-primary">
                created {formatDistanceToNowStrict(new Date(data.CreatedAt))} ago
              </div>
              <div className="text-sm text-muted-foreground mt-0.5">
                ({format(new Date(data.CreatedAt), "MMM d, yyyy")})
              </div>
            </>
          )}

          {data.CreatedAt && data.UpdatedAt && (
            <div className="text-muted-foreground text-sm mx-1">•</div>
          )}

          {data.UpdatedAt && (
            <>
              <div className="text-sm font-medium">
                last updated {formatDistanceToNowStrict(new Date(data.UpdatedAt))} ago
              </div>
              <div className="text-sm text-muted-foreground mt-0.5">
                ({format(new Date(data.UpdatedAt), "MMM d, yyyy")})
              </div>
            </>
          )}

          <div className="flex-1" />
          
          <div className="flex items-center gap-2">
            {data.Status && <Badge>{data.Status}</Badge>}
            {data.IANAStatus && (
              <Badge variant="secondary">{`IANA ${data.IANAStatus}`}</Badge>
            )}
            {typeof data.Autorenew === "boolean" && (
              <Badge variant={data.Autorenew ? "default" : "outline"}>
                {data.Autorenew ? "Autorenew on" : "Autorenew off"}
              </Badge>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
