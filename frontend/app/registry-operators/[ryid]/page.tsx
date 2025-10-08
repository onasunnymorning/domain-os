'use client';

import { use } from 'react';
import { useRouter } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useRegistryOperator, useDeleteRegistryOperator } from '@/lib/hooks/useRegistryOperators';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { ArrowLeft, Pencil, Trash2, Mail, Phone, Globe, FileText } from 'lucide-react';
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
      <div className="max-w-4xl space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Link href="/registry-operators">
              <Button variant="ghost" size="sm">
                <ArrowLeft className="h-4 w-4 mr-2" />
                Back
              </Button>
            </Link>
          </div>
          
          {!isLoading && operator && (
            <div className="flex gap-2">
              <Link href={`/registry-operators/${ryid}/edit`}>
                <Button variant="outline">
                  <Pencil className="h-4 w-4 mr-2" />
                  Edit
                </Button>
              </Link>
              <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive">
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
          <Card>
            <CardHeader>
              <Skeleton className="h-8 w-64" />
              <Skeleton className="h-4 w-32 mt-2" />
            </CardHeader>
            <CardContent className="space-y-4">
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-20 w-full" />
            </CardContent>
          </Card>
        ) : operator ? (
          <>
            {/* Main Info */}
            <Card>
              <CardHeader>
                <div className="flex items-start justify-between">
                  <div>
                    <CardTitle className="text-2xl">{operator.Name}</CardTitle>
                    <CardDescription className="mt-2">
                      <Badge variant="secondary" className="font-mono">
                        {operator.RyID}
                      </Badge>
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* Contact Information */}
                <div>
                  <h3 className="text-sm font-semibold mb-4">Contact Information</h3>
                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="flex items-start gap-3">
                      <Mail className="h-5 w-5 text-muted-foreground mt-0.5" />
                      <div>
                        <p className="text-sm font-medium">Email</p>
                        <a 
                          href={`mailto:${operator.Email}`}
                          className="text-sm text-primary hover:underline"
                        >
                          {operator.Email}
                        </a>
                      </div>
                    </div>

                    {operator.URL && (
                      <div className="flex items-start gap-3">
                        <Globe className="h-5 w-5 text-muted-foreground mt-0.5" />
                        <div>
                          <p className="text-sm font-medium">Website</p>
                          <a 
                            href={operator.URL}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-sm text-primary hover:underline"
                          >
                            {operator.URL}
                          </a>
                        </div>
                      </div>
                    )}

                    {operator.Voice && (
                      <div className="flex items-start gap-3">
                        <Phone className="h-5 w-5 text-muted-foreground mt-0.5" />
                        <div>
                          <p className="text-sm font-medium">Phone</p>
                          <p className="text-sm text-muted-foreground">{operator.Voice}</p>
                        </div>
                      </div>
                    )}

                    {operator.Fax && (
                      <div className="flex items-start gap-3">
                        <FileText className="h-5 w-5 text-muted-foreground mt-0.5" />
                        <div>
                          <p className="text-sm font-medium">Fax</p>
                          <p className="text-sm text-muted-foreground">{operator.Fax}</p>
                        </div>
                      </div>
                    )}
                  </div>
                </div>

                {/* Metadata */}
                {(operator.CreatedAt || operator.UpdatedAt) && (
                  <div className="border-t pt-6">
                    <h3 className="text-sm font-semibold mb-4">Metadata</h3>
                    <div className="grid gap-4 md:grid-cols-2 text-sm">
                      {operator.CreatedAt && (
                        <div>
                          <p className="font-medium">Created</p>
                          <p className="text-muted-foreground">
                            {new Date(operator.CreatedAt).toLocaleString()}
                          </p>
                        </div>
                      )}
                      {operator.UpdatedAt && (
                        <div>
                          <p className="font-medium">Last Updated</p>
                          <p className="text-muted-foreground">
                            {new Date(operator.UpdatedAt).toLocaleString()}
                          </p>
                        </div>
                      )}
                    </div>
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
