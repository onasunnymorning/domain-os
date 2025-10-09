'use client';

import { use } from 'react';
import { useRouter } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useRegistryOperator, useDeleteRegistryOperator } from '@/lib/hooks/useRegistryOperators';
import { useTLDsByRyID } from '@/lib/hooks/useTLDs';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { ArrowLeft, Pencil, Trash2, Mail, Phone, Globe, FileText, Server, Building2 } from 'lucide-react';
import Link from 'next/link';
import { toast } from 'sonner';
import { useState } from 'react';

interface Props {
  params: Promise<{ ryid: string }>;
}

export default function RegistryOperatorDetailPage({ params }: Props) {
  const { ryid } = use(params);
  const router = useRouter();
  const { data: operator, isLoading, error } = useRegistryOperator(ryid);
  const { data: tldsData, isLoading: tldsLoading } = useTLDsByRyID(ryid);
  const deleteMutation = useDeleteRegistryOperator();
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(ryid);
      toast.success('Registry operator deleted successfully');
      router.push('/registry-operators');
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to delete registry operator');
    }
  };

  if (error) {
    return (
      <DashboardLayout>
        <div className="space-y-6">
          <div className="flex items-center gap-4">
            <Link href="/registry-operators">
              <Button variant="ghost" size="sm">
                <ArrowLeft className="h-4 w-4 mr-2" />
                Back
              </Button>
            </Link>
          </div>
          <div className="rounded-md bg-destructive/15 p-4 text-destructive">
            Error loading registry operator: {error.message}
          </div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="max-w-5xl space-y-8">
        {/* Back Button and Actions */}
        <div className="flex items-center justify-between">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/registry-operators">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back
            </Link>
          </Button>
          
          {!isLoading && operator && (
            <div className="flex gap-2">
              <Button variant="outline" size="sm" asChild>
                <Link href={`/registry-operators/${ryid}/edit`}>
                  <Pencil className="h-4 w-4 mr-2" />
                  Edit
                </Link>
              </Button>
              <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    <Trash2 className="h-4 w-4 mr-2" />
                    Delete
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Are you sure?</AlertDialogTitle>
                    <AlertDialogDescription>
                      This action cannot be undone. This will permanently delete the registry operator{' '}
                      <strong>{operator.Name}</strong>.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={handleDelete}
                      className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    >
                      Delete
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          )}
        </div>

        {isLoading ? (
          <div className="space-y-8">
            <div className="space-y-2">
              <Skeleton className="h-12 w-64" />
              <Skeleton className="h-6 w-32 ml-[52px]" />
            </div>
            <Card>
              <CardHeader>
                <Skeleton className="h-6 w-40" />
              </CardHeader>
              <CardContent className="space-y-6">
                <Skeleton className="h-20 w-full" />
                <Skeleton className="h-20 w-full" />
              </CardContent>
            </Card>
          </div>
        ) : operator ? (
          <>
            {/* Hero Section */}
            <div className="space-y-2">
              <div className="flex items-baseline gap-3">
                <Building2 className="h-10 w-10 text-muted-foreground" />
                <h1 className="text-5xl font-bold tracking-tight">{operator.Name}</h1>
              </div>
              <p className="text-sm text-muted-foreground ml-[52px]">Registry Operator</p>
            </div>

            {/* Contact Information */}
            <Card>
              <CardHeader>
                <CardTitle>Contact Information</CardTitle>
                <CardDescription>How to reach this registry operator</CardDescription>
              </CardHeader>
              <CardContent className="space-y-8">
                {/* Email */}
                <div className="space-y-2">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-2">
                    <Mail className="h-3 w-3" />
                    Email
                  </p>
                  <a 
                    href={`mailto:${operator.Email}`}
                    className="text-lg font-medium text-primary hover:underline inline-block"
                  >
                    {operator.Email}
                  </a>
                </div>

                {/* Optional fields in a grid */}
                <div className="grid gap-6 md:grid-cols-2">
                  {operator.URL && (
                    <div className="space-y-2">
                      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-2">
                        <Globe className="h-3 w-3" />
                        Website
                      </p>
                      <a 
                        href={operator.URL}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-base font-medium text-primary hover:underline inline-block break-all"
                      >
                        {operator.URL}
                      </a>
                    </div>
                  )}

                  {operator.Voice && (
                    <div className="space-y-2">
                      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-2">
                        <Phone className="h-3 w-3" />
                        Phone
                      </p>
                      <p className="text-base font-medium">{operator.Voice}</p>
                    </div>
                  )}

                  {operator.Fax && (
                    <div className="space-y-2">
                      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-2">
                        <FileText className="h-3 w-3" />
                        Fax
                      </p>
                      <p className="text-base font-medium">{operator.Fax}</p>
                    </div>
                  )}
                </div>

                {/* Metadata */}
                {(operator.CreatedAt || operator.UpdatedAt) && (
                  <div className="pt-6 border-t space-y-3">
                    {operator.CreatedAt && (
                      <div className="text-sm text-muted-foreground">
                        Created {new Date(operator.CreatedAt).toLocaleString()}
                      </div>
                    )}
                    {operator.UpdatedAt && (
                      <div className="text-sm text-muted-foreground">
                        Last updated {new Date(operator.UpdatedAt).toLocaleString()}
                      </div>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* TLDs Section */}
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="flex items-center gap-2">
                      <Server className="h-5 w-5" />
                      Top-Level Domains
                    </CardTitle>
                    <CardDescription>
                      {tldsLoading 
                        ? 'Loading...' 
                        : `${tldsData?.Data?.length || 0} TLD(s) managed by this operator`
                      }
                    </CardDescription>
                  </div>
                  <Link href={`/tlds?ryid_equals=${ryid}`}>
                    <Button variant="outline" size="sm">
                      View All
                    </Button>
                  </Link>
                </div>
              </CardHeader>
              <CardContent>
                {tldsLoading ? (
                  <div className="flex gap-2 flex-wrap">
                    {[...Array(5)].map((_, i) => (
                      <Skeleton key={i} className="h-6 w-16" />
                    ))}
                  </div>
                ) : tldsData?.Data && tldsData.Data.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {tldsData.Data.map((tld) => (
                      <Link key={tld.Name} href={`/tlds/${tld.Name}`}>
                        <Badge 
                          variant="secondary"
                          className="text-sm hover:bg-primary/20 cursor-pointer"
                        >
                          .{tld.Name}
                          <span className="ml-2 text-xs opacity-70">
                            ({tld.Type === 'generic' ? 'gTLD' : tld.Type === 'country-code' ? 'ccTLD' : 'SLD'})
                          </span>
                        </Badge>
                      </Link>
                    ))}
                  </div>
                ) : (
                  <div className="text-center py-8">
                    <Server className="h-12 w-12 mx-auto text-muted-foreground/50 mb-3" />
                    <p className="text-sm text-muted-foreground">
                      No TLDs assigned to this operator yet
                    </p>
                    <Link href="/tlds/create">
                      <Button variant="outline" size="sm" className="mt-4">
                        Create First TLD
                      </Button>
                    </Link>
                  </div>
                )}
              </CardContent>
            </Card>
          </>
        ) : null}
      </div>
    </DashboardLayout>
  );
}
