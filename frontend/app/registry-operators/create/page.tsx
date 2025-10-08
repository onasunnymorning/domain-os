'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useRouter } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useCreateRegistryOperator } from '@/lib/hooks/useRegistryOperators';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { ArrowLeft } from 'lucide-react';
import Link from 'next/link';
import { toast } from 'sonner';

const formSchema = z.object({
  RyID: z
    .string()
    .min(3, 'RyID must be at least 3 characters')
    .max(16, 'RyID must not exceed 16 characters')
    .regex(/^[\x20-\x7E]+$/, 'RyID must contain only ASCII characters')
    .refine((val) => val.trim() === val, 'RyID cannot start or end with whitespace'),
  Name: z.string().min(1, 'Name is required').min(3, 'Name must be at least 3 characters'),
  Email: z.string().email('Invalid email address'),
  URL: z.string().url('Invalid URL').optional().or(z.literal('')),
  Voice: z.string().optional(),
  Fax: z.string().optional(),
});

type FormValues = z.infer<typeof formSchema>;

export default function CreateRegistryOperatorPage() {
  const router = useRouter();
  const { mutate, isPending } = useCreateRegistryOperator();
  
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      RyID: '',
      Name: '',
      Email: '',
      URL: '',
      Voice: '',
      Fax: '',
    },
  });

  const onSubmit = (data: FormValues) => {
    mutate(data, {
      onSuccess: () => {
        toast.success('Registry operator created successfully');
        router.push('/registry-operators');
      },
      onError: (error: any) => {
        toast.error(error.response?.data?.error || 'Failed to create registry operator');
      },
    });
  };

  return (
    <DashboardLayout>
      <div className="max-w-2xl space-y-6">
        {/* Header */}
        <div className="flex items-center gap-4">
          <Link href="/registry-operators">
            <Button variant="ghost" size="sm">
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back
            </Button>
          </Link>
        </div>

        <div>
          <h1 className="text-3xl font-bold tracking-tight">Create Registry Operator</h1>
          <p className="text-muted-foreground">
            Add a new registry operator to the system
          </p>
        </div>

        {/* Form */}
        <Card>
          <CardHeader>
            <CardTitle>Registry Operator Details</CardTitle>
            <CardDescription>
              Enter the information for the new registry operator. Fields marked with * are required.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                <FormField
                  control={form.control}
                  name="RyID"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>RyID *</FormLabel>
                      <FormControl>
                        <Input placeholder="RY-001" {...field} />
                      </FormControl>
                      <FormDescription>
                        Unique identifier for the registry operator (e.g., RY-001)
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="Name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Name *</FormLabel>
                      <FormControl>
                        <Input placeholder="Example Registry Inc." {...field} />
                      </FormControl>
                      <FormDescription>
                        Full name of the registry operator
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="Email"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Email *</FormLabel>
                      <FormControl>
                        <Input type="email" placeholder="contact@registry.com" {...field} />
                      </FormControl>
                      <FormDescription>
                        Primary contact email address
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="URL"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>URL</FormLabel>
                      <FormControl>
                        <Input placeholder="https://registry.com" {...field} />
                      </FormControl>
                      <FormDescription>
                        Website URL (optional)
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="Voice"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Phone</FormLabel>
                      <FormControl>
                        <Input placeholder="+1.1234567890" {...field} />
                      </FormControl>
                      <FormDescription>
                        Phone number in E.164 format (optional)
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="Fax"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Fax</FormLabel>
                      <FormControl>
                        <Input placeholder="+1.1234567890" {...field} />
                      </FormControl>
                      <FormDescription>
                        Fax number in E.164 format (optional)
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <div className="flex gap-4">
                  <Button type="submit" disabled={isPending}>
                    {isPending ? 'Creating...' : 'Create Registry Operator'}
                  </Button>
                  <Button 
                    type="button" 
                    variant="outline" 
                    onClick={() => router.back()}
                    disabled={isPending}
                  >
                    Cancel
                  </Button>
                </div>
              </form>
            </Form>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
