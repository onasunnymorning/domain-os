'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useRouter } from 'next/navigation';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { useCreateRegistrar } from '@/lib/hooks/useRegistrars';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage, FormDescription } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import Link from 'next/link';
import { ArrowLeft, UserPlus } from 'lucide-react';
import { toast } from 'sonner';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { RegistrarStatus, IANARegistrarStatus } from '@/lib/types/registrar';
import { Switch } from '@/components/ui/switch';
import posthog from 'posthog-js';

// Minimal payload based on swagger commands.CreateRegistrarCommand
// Required: ClID, Name, Email, PostalInfo[ { Type, Address{ CC, City, ... } } ]
const formSchema = z.object({
  ClID: z
    .string()
    .min(3, 'ClID must be at least 3 characters')
    .max(32, 'ClID must not exceed 32 characters')
    .regex(/^[\x20-\x7E]+$/, 'ClID must contain only ASCII characters')
    .refine((val) => val.trim() === val, 'ClID cannot start or end with whitespace'),
  Name: z.string().min(1, 'Name is required').min(3, 'Name must be at least 3 characters'),
  NickName: z.string().optional(),
  Email: z.string().email('Invalid email address'),
  GurIDString: z
    .string()
    .regex(/^\d+$/, 'GurID must be numeric')
    .optional()
    .or(z.literal('')),
  URL: z.string().url('Invalid URL').optional().or(z.literal('')),
  Voice: z.string().optional(),
  Fax: z.string().optional(),
  RdapBaseURL: z.string().url('Invalid URL').optional().or(z.literal('')),
  WhoisName: z.string().optional(),
  WhoisURL: z.string().url('Invalid URL').optional().or(z.literal('')),
  // Postal info (single entry minimal)
  PostalType: z.enum(['loc', 'int']),
  Street1: z.string().optional(),
  Street2: z.string().optional(),
  Street3: z.string().optional(),
  City: z.string().min(1, 'City is required'),
  SP: z.string().optional(),
  PC: z.string().optional(),
  CC: z.string().min(2, 'Country code is required').max(2, 'Use 2-letter country code'),
  // Optional initial statuses
  Status: z.enum(['ok', 'readonly', 'terminated']).optional(),
  IANAStatus: z.enum(['Accredited', 'Reserved', 'Terminated', 'Unknown']).optional(),
  Autorenew: z.boolean().optional(),
});

type FormValues = z.infer<typeof formSchema>;

export default function CreateRegistrarPage() {
  const router = useRouter();
  const { mutateAsync, isPending } = useCreateRegistrar();

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      ClID: '',
      Name: '',
      NickName: '',
      Email: '',
      GurIDString: '',
      URL: '',
      Voice: '',
      Fax: '',
      RdapBaseURL: '',
      WhoisName: '',
      WhoisURL: '',
  PostalType: 'int',
      Street1: '',
      Street2: '',
      Street3: '',
      City: '',
      SP: '',
      PC: '',
      CC: '',
      Autorenew: true,
      
    },
  });

  const onSubmit = async (values: FormValues) => {
    const payload: any = {
      ClID: values.ClID,
      Name: values.Name,
      NickName: values.NickName || values.Name,
      Email: values.Email,
      URL: values.URL || undefined,
      Voice: values.Voice || undefined,
      Fax: values.Fax || undefined,
      RdapBaseURL: values.RdapBaseURL || undefined,
      WhoisInfo: undefined as any,
      PostalInfo: [
        {
          Type: values.PostalType,
          Address: {
            Street1: values.Street1 || undefined,
            Street2: values.Street2 || undefined,
            Street3: values.Street3 || undefined,
            City: values.City,
            SP: values.SP || undefined,
            PC: values.PC || undefined,
            CC: values.CC,
          },
        },
      ],
    };
    if (values.GurIDString && values.GurIDString.trim().length > 0) {
      payload.GurID = Number(values.GurIDString);
    }
    if (values.WhoisName || values.WhoisURL) {
      payload.WhoisInfo = {
        Name: values.WhoisName || undefined,
        URL: values.WhoisURL || undefined,
      };
    }
    if (values.Status) {
      payload.Status = values.Status;
    }
    if (values.IANAStatus) {
      payload.IANAStatus = values.IANAStatus;
    }
    // Autorenew defaults to true for new registrars unless explicitly toggled off
    payload.Autorenew = typeof values.Autorenew === 'boolean' ? values.Autorenew : true;

    try {
      const created = await mutateAsync(payload as any);
      toast.success('Registrar created successfully');
      const clid = created?.ClID || values.ClID;
      posthog.capture('registrar_created', {
        clid,
        name: values.Name,
        iana_status: values.IANAStatus,
        status: values.Status,
        country: values.CC,
      });
      router.push(`/registrars/${encodeURIComponent(clid)}`);
    } catch (error: any) {
      posthog.captureException(error);
      toast.error(error?.response?.data?.error || 'Failed to create registrar');
    }
  };

  return (
    <DashboardLayout>
      <div className="max-w-3xl space-y-6">
        {/* Header */}
        <div className="flex items-center gap-4">
          <Link href="/registrars">
            <Button variant="ghost" size="sm">
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back
            </Button>
          </Link>
        </div>

        <div>
          <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
            <UserPlus className="h-7 w-7" />
            Create Registrar
          </h1>
          <p className="text-muted-foreground">Add a new registrar to the system</p>
        </div>

        {/* Basic Info */}
        <Card>
          <CardHeader>
            <CardTitle>Basic Information</CardTitle>
            <CardDescription>Required fields are marked with *</CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <FormField
                    control={form.control}
                    name="ClID"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>ClID *</FormLabel>
                        <FormControl>
                          <Input placeholder="abc-1234" {...field} />
                        </FormControl>
                        <FormDescription>Unique client identifier for the registrar</FormDescription>
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
                          <Input placeholder="Example Registrar, Inc." {...field} />
                        </FormControl>
                        <FormDescription>Legal entity name of the registrar</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="NickName"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Nickname</FormLabel>
                        <FormControl>
                          <Input placeholder="Brand or short name" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="GurIDString"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>IANA Registrar ID (GurID)</FormLabel>
                        <FormControl>
                          <Input placeholder="e.g. 468" {...field} />
                        </FormControl>
                        <FormDescription>Optional numeric IANA registrar ID</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="Status"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Initial Status</FormLabel>
                        <Select
                          onValueChange={(v) => field.onChange(v === '' ? undefined : v)}
                          value={field.value ?? ''}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Auto (default)" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value={RegistrarStatus.OK}>ok</SelectItem>
                            <SelectItem value={RegistrarStatus.Readonly}>readonly</SelectItem>
                            <SelectItem value={RegistrarStatus.Terminated}>terminated</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          Leave empty to let the system choose defaults (usually ok).
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="IANAStatus"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>IANA Status</FormLabel>
                        <Select
                          onValueChange={(v) => field.onChange(v === '' ? undefined : v)}
                          value={field.value ?? ''}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="None" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value={IANARegistrarStatus.Accredited}>Accredited</SelectItem>
                            <SelectItem value={IANARegistrarStatus.Reserved}>Reserved</SelectItem>
                            <SelectItem value={IANARegistrarStatus.Terminated}>Terminated</SelectItem>
                            <SelectItem value={IANARegistrarStatus.Unknown}>Unknown</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          Optional IANA accreditation status to set at creation.
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>

                {/* Contact & URLs */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <FormField
                    control={form.control}
                    name="Autorenew"
                    render={({ field }) => (
                      <FormItem className="flex flex-col justify-end">
                        <FormLabel>Autorenew</FormLabel>
                        <div className="flex items-center gap-3 py-2">
                          <Switch checked={!!field.value} onCheckedChange={field.onChange} />
                          <span className="text-sm text-muted-foreground">
                            When enabled, domains under this registrar will attempt to renew automatically. Success depends on each domain’s TLD configuration.
                          </span>
                        </div>
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
                          <Input type="email" placeholder="contact@registrar.com" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="URL"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Website URL</FormLabel>
                        <FormControl>
                          <Input placeholder="https://registrar.example" {...field} />
                        </FormControl>
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
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="RdapBaseURL"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>RDAP Base URL</FormLabel>
                        <FormControl>
                          <Input placeholder="https://rdap.registrar.example" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6 md:col-span-2">
                    <FormField
                      control={form.control}
                      name="WhoisName"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>WHOIS Server Name</FormLabel>
                          <FormControl>
                            <Input placeholder="whois.example" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="WhoisURL"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>WHOIS Server URL</FormLabel>
                          <FormControl>
                            <Input placeholder="https://example.com/whois" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </div>

                {/* Postal Info */}
                <div className="space-y-4">
                  <div>
                    <h3 className="text-lg font-semibold">Postal Information</h3>
                    <p className="text-sm text-muted-foreground">
                      Provide the primary postal address for the registrar. Country and City are required.
                    </p>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <FormField
                      control={form.control}
                      name="PostalType"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Postal Type</FormLabel>
                          <Select onValueChange={field.onChange} value={field.value}>
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="Select type" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="loc">Local (loc)</SelectItem>
                              <SelectItem value="int">International (int)</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="CC"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Country Code (CC) *</FormLabel>
                          <FormControl>
                            <Input placeholder="US" maxLength={2} {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="City"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>City *</FormLabel>
                          <FormControl>
                            <Input placeholder="Austin" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="SP"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>State/Province (SP)</FormLabel>
                          <FormControl>
                            <Input placeholder="TX" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="PC"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Postal Code (PC)</FormLabel>
                          <FormControl>
                            <Input placeholder="73301" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="Street1"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Street 1</FormLabel>
                          <FormControl>
                            <Input placeholder="1234 Main St" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="Street2"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Street 2</FormLabel>
                          <FormControl>
                            <Input placeholder="Suite 123" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="Street3"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Street 3</FormLabel>
                          <FormControl>
                            <Input placeholder="Attn: Registration" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </div>

                <div className="flex gap-4">
                  <Button type="submit" disabled={isPending}>
                    {isPending ? 'Creating...' : 'Create Registrar'}
                  </Button>
                  <Button type="button" variant="outline" onClick={() => router.back()} disabled={isPending}>
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
