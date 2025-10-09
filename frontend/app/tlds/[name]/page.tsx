'use client';

import { use } from 'react';
import { useRouter } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useTLD, useDeleteTLD } from '@/lib/hooks/useTLDs';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
  const { data: tld, isLoading, error } = useTLD(decodeURIComponent(name));
  const { mutate: deleteTLD, isPending: isDeleting } = useDeleteTLD();

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
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="icon" asChild>
              <Link href="/tlds">
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div>
              <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
                <Globe className="h-8 w-8" />
                {isLoading ? <Skeleton className="h-8 w-32" /> : tld?.Name}
              </h1>
              <p className="text-muted-foreground mt-2">
                TLD Details
              </p>
            </div>
          </div>
          {!isLoading && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" disabled={isDeleting}>
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

        {/* TLD Information Card */}
        <Card>
          <CardHeader>
            <CardTitle>TLD Information</CardTitle>
            <CardDescription>Basic details about this top-level domain</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="space-y-4">
                {[1, 2, 3, 4, 5].map((i) => (
                  <div key={i} className="flex items-center gap-4">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-4 w-48" />
                  </div>
                ))}
              </div>
            ) : (
              <div className="grid gap-4">
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="font-semibold">Name:</span>
                  <span className="col-span-2 font-mono text-lg">{tld?.Name}</span>
                </div>
                
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="font-semibold">Type:</span>
                  <span className="col-span-2">{tld && getTypeBadge(tld.Type)}</span>
                </div>

                {tld?.UName && (
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="font-semibold">Unicode Name:</span>
                    <span className="col-span-2 font-mono">{tld.UName}</span>
                  </div>
                )}

                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="font-semibold flex items-center gap-2">
                    <Building2 className="h-4 w-4" />
                    Registry Operator:
                  </span>
                  <span className="col-span-2">
                    <Link href={`/registry-operators/${tld?.RyID}`} className="text-primary hover:underline">
                      {tld?.RyID}
                    </Link>
                  </span>
                </div>

                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="font-semibold">DNS Enabled:</span>
                  <span className="col-span-2">
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
                  </span>
                </div>

                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="font-semibold">Escrow Import:</span>
                  <span className="col-span-2">
                    {tld?.AllowEscrowImport ? (
                      <Badge variant="secondary">Allowed</Badge>
                    ) : (
                      <Badge variant="outline">Not Allowed</Badge>
                    )}
                  </span>
                </div>

                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="font-semibold flex items-center gap-2">
                    <Calendar className="h-4 w-4" />
                    Created:
                  </span>
                  <span className="col-span-2 text-muted-foreground">
                    {tld && format(new Date(tld.CreatedAt), 'PPpp')}
                  </span>
                </div>

                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="font-semibold flex items-center gap-2">
                    <Calendar className="h-4 w-4" />
                    Updated:
                  </span>
                  <span className="col-span-2 text-muted-foreground">
                    {tld && format(new Date(tld.UpdatedAt), 'PPpp')}
                  </span>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Phase Timeline */}
        {!isLoading && tld && <PhaseTimeline tldName={tld.Name} />}

        {/* Actions */}
        <div className="flex gap-4">
          <Button variant="outline" asChild>
            <Link href="/tlds">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to List
            </Link>
          </Button>
        </div>
      </div>
    </DashboardLayout>
  );
}
