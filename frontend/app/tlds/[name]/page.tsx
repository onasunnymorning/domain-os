'use client';

import { use, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useTLD, useDeleteTLD } from '@/lib/hooks/useTLDs';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { 
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { ArrowLeft, Globe, Trash2, CheckCircle, XCircle, Calendar, Building2 } from 'lucide-react';
import Link from 'next/link';
import { format } from 'date-fns';
import { PhaseTimeline } from '@/components/phases/PhaseTimeline';

interface Props {
  params: Promise<{ name: string }>;
}

export default function TLDDetailPage({ params }: Props) {
  const { name } = use(params);
  const router = useRouter();
  const searchParams = useSearchParams();
  const phaseName = searchParams.get('phase');
  const { data: tld, isLoading, error } = useTLD(decodeURIComponent(name));
  const { mutate: deleteTLD, isPending: isDeleting } = useDeleteTLD();

  // Scroll to top when the page loads
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [name]);

  const handleDelete = () => {
    deleteTLD(name, {
      onSuccess: () => {
        router.push('/tlds');
      },
    });
  };

  const getTypeBadge = (type: string) => {
    switch (type) {
      case 'generic':
        return <Badge variant="default">Generic TLD (gTLD)</Badge>;
      case 'country-code':
        return <Badge variant="secondary">Country-Code TLD (ccTLD)</Badge>;
      case 'second-level':
        return <Badge variant="outline">Second-Level Domain (SLD)</Badge>;
      default:
        return <Badge variant="outline">{type}</Badge>;
    }
  };

  if (error) {
    return (
      <DashboardLayout>
        <div className="space-y-6">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="icon" asChild>
              <Link href="/tlds">
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div>
              <h1 className="text-3xl font-bold tracking-tight">TLD Not Found</h1>
            </div>
          </div>
          <Card>
            <CardContent className="pt-6">
              <p className="text-muted-foreground">
                The TLD &quot;{name}&quot; could not be found.
              </p>
              <Button asChild className="mt-4">
                <Link href="/tlds">Back to TLDs</Link>
              </Button>
            </CardContent>
          </Card>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="space-y-8">
        {/* Back Button */}
        <div className="flex items-center justify-between">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/tlds">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back
            </Link>
          </Button>
          {!isLoading && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" size="sm" disabled={isDeleting}>
                  <Trash2 className="mr-2 h-4 w-4" />
                  Delete TLD
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Are you sure?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will permanently delete the TLD &quot;{tld?.Name}&quot; and all associated data.
                    This action cannot be undone.
                    <br /><br />
                    <strong>Note:</strong> TLDs with active phases cannot be deleted.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground">
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </div>

        {/* Hero Section */}
        <div className="space-y-2">
          <div className="flex items-baseline gap-3">
            <Globe className="h-10 w-10 text-muted-foreground" />
            {isLoading ? (
              <Skeleton className="h-12 w-48" />
            ) : (
              <h1 className="text-5xl font-bold tracking-tight">{tld?.Name}</h1>
            )}
          </div>
          <p className="text-sm text-muted-foreground ml-[52px]">TLD Details</p>
        </div>

        {/* TLD Information Card */}
        <Card>
          <CardContent className="pt-6">
            {isLoading ? (
              <div className="space-y-6">
                {[1, 2, 3, 4].map((i) => (
                  <div key={i} className="space-y-2">
                    <Skeleton className="h-3 w-20" />
                    <Skeleton className="h-6 w-full max-w-md" />
                  </div>
                ))}
              </div>
            ) : (
              <div className="space-y-8">
                {/* Type */}
                <div className="space-y-2">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Type</p>
                  <div>{tld && getTypeBadge(tld.Type)}</div>
                </div>

                {/* Registry Operator */}
                <div className="space-y-2">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-2">
                    <Building2 className="h-3 w-3" />
                    Registry Operator
                  </p>
                  <Link 
                    href={`/registry-operators/${tld?.RyID}`} 
                    className="text-lg font-medium text-primary hover:underline inline-block"
                  >
                    {tld?.RyID}
                  </Link>
                </div>

                {/* Status Grid */}
                <div className="grid gap-6 md:grid-cols-2">
                  <div className="space-y-2">
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">DNS Enabled</p>
                    <div>
                      {tld?.EnableDNS ? (
                        <Badge variant="secondary" className="bg-green-100 text-green-800">
                          <CheckCircle className="mr-1 h-3 w-3" />
                          Yes
                        </Badge>
                      ) : (
                        <Badge variant="outline">
                          <XCircle className="mr-1 h-3 w-3" />
                          No
                        </Badge>
                      )}
                    </div>
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Escrow Import</p>
                    <div>
                      {tld?.AllowEscrowImport ? (
                        <Badge variant="secondary">Allowed</Badge>
                      ) : (
                        <Badge variant="outline">Not Allowed</Badge>
                      )}
                    </div>
                  </div>
                </div>

                {/* Metadata */}
                <div className="pt-6 border-t space-y-4">
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Calendar className="h-4 w-4" />
                    <span>Created {tld && format(new Date(tld.CreatedAt), 'PPpp')}</span>
                  </div>
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Calendar className="h-4 w-4" />
                    <span>Updated {tld && format(new Date(tld.UpdatedAt), 'PPpp')}</span>
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Phase Timeline */}
        {!isLoading && tld && (
          <PhaseTimeline 
            tldName={tld.Name} 
            initialPhaseName={phaseName || undefined}
          />
        )}
      </div>
    </DashboardLayout>
  );
}
