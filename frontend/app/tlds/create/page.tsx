'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useRouter } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useCreateTLD } from '@/lib/hooks/useTLDs';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ArrowLeft, Globe } from 'lucide-react';
import Link from 'next/link';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';

const formSchema = z.object({
  Name: z
    .string()
    .min(1, 'TLD Name is required')
    .max(63, 'TLD Name must not exceed 63 characters')
    .regex(/^[a-z0-9]([a-z0-9.-]{0,61}[a-z0-9])?$/i, 'Invalid TLD name format')
    .refine((val) => !val.startsWith('-') && !val.endsWith('-'), 'TLD name cannot start or end with hyphen'),
  RyID: z.string().min(1, 'Registry Operator is required'),
  CreateOperatorRegistrars: z.boolean(),
  AllowEscrowImport: z.boolean(),
});

type FormValues = z.infer<typeof formSchema>;

export default function CreateTLDPage() {
  const router = useRouter();
  const { mutate, isPending } = useCreateTLD();
  const { data: operatorsData, isLoading: isLoadingOperators } = useRegistryOperators();
  
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Name: '',
      RyID: '',
      CreateOperatorRegistrars: true,
      AllowEscrowImport: true,
    },
  });

  const onSubmit = (values: FormValues) => {
    mutate(values, {
      onSuccess: () => {
        router.push('/tlds');
      },
    });
  };

  // Detect TLD type based on name
  const detectTLDType = (name: string) => {
    if (!name) return null;
    if (name.length === 2) return 'country-code';
    if (name.includes('.')) return 'second-level';
    return 'generic';
  };

  const tldName = form.watch('Name');
  const tldType = detectTLDType(tldName);

  const operators = operatorsData?.Data || [];

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" asChild>
            <Link href="/tlds">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <Globe className="h-8 w-8" />
              Create TLD
            </h1>
            <p className="text-muted-foreground mt-2">
              Add a new top-level domain to the system
            </p>
          </div>
        </div>

        {/* Form */}
        <Card>
          <CardHeader>
            <CardTitle>TLD Details</CardTitle>
            <CardDescription>
              Enter the information for the new TLD. Fields marked with * are required.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                <FormField
                  control={form.control}
                  name="Name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>TLD Name *</FormLabel>
                      <FormControl>
                        <Input placeholder="com" {...field} />
                      </FormControl>
                      <FormDescription>
                        Enter the TLD name (e.g., com, org, uk, co.uk)
                      </FormDescription>
                      {tldType && (
                        <div className="mt-2 text-sm text-muted-foreground">
                          <span>Detected type: </span>
                          <Badge variant="outline" className="ml-2">
                            {tldType === 'generic' && 'Generic TLD (gTLD)'}
                            {tldType === 'country-code' && 'Country-Code TLD (ccTLD)'}
                            {tldType === 'second-level' && 'Second-Level Domain (SLD)'}
                          </Badge>
                        </div>
                      )}
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="RyID"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Registry Operator *</FormLabel>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="Select a registry operator" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {isLoadingOperators ? (
                            <SelectItem value="loading" disabled>Loading operators...</SelectItem>
                          ) : operators.length === 0 ? (
                            <SelectItem value="none" disabled>No operators available</SelectItem>
                          ) : (
                            operators.map((op) => (
                              <SelectItem key={op.RyID} value={op.RyID}>
                                {op.RyID} - {op.Name}
                              </SelectItem>
                            ))
                          )}
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        Select the registry operator managing this TLD
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="CreateOperatorRegistrars"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                      <FormControl>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <div className="space-y-1 leading-none">
                        <FormLabel>Create Operator Registrar Accounts</FormLabel>
                        <FormDescription>
                          Automatically creates the ICANN-reserved registrar accounts (9998/9999) for this TLD. These are required for transactions where the Registry Operator acts as Registrar.
                        </FormDescription>
                      </div>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="AllowEscrowImport"
                  render={({ field }) => (
                    <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                      <FormControl>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <div className="space-y-1 leading-none">
                        <FormLabel>Allow Escrow Import</FormLabel>
                        <FormDescription>
                          Permits importing domain data from escrow deposits into this TLD. Turn off if you want to prevent bulk data imports.
                        </FormDescription>
                      </div>
                    </FormItem>
                  )}
                />

                <div className="flex gap-4">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => router.push('/tlds')}
                  >
                    Cancel
                  </Button>
                  <Button type="submit" disabled={isPending}>
                    {isPending ? 'Creating...' : 'Create TLD'}
                  </Button>
                </div>
              </form>
            </Form>
          </CardContent>
        </Card>

        {/* Info Card */}
        <Card>
          <CardHeader>
            <CardTitle>TLD Type Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <h4 className="font-semibold flex items-center gap-2">
                <Badge variant="default">gTLD</Badge>
                Generic TLD
              </h4>
              <p className="text-sm text-muted-foreground mt-1">
                More than 2 characters, no dots (e.g., com, org, tech)
              </p>
            </div>
            <div>
              <h4 className="font-semibold flex items-center gap-2">
                <Badge variant="secondary">ccTLD</Badge>
                Country-Code TLD
              </h4>
              <p className="text-sm text-muted-foreground mt-1">
                Exactly 2 characters (e.g., uk, jp, us)
              </p>
            </div>
            <div>
              <h4 className="font-semibold flex items-center gap-2">
                <Badge variant="outline">SLD</Badge>
                Second-Level Domain
              </h4>
              <p className="text-sm text-muted-foreground mt-1">
                Contains a dot (e.g., co.uk, com.au)
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
