'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useCreatePhase } from '@/lib/hooks/usePhases';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { CalendarIcon, AlertCircle, CheckCircle2, ArrowRight, ArrowLeft } from 'lucide-react';
import { format } from 'date-fns';
import { cn } from '@/lib/utils';

interface PhaseCreateWizardProps {
  tldName: string;
  open: boolean;
  onClose: () => void;
  existingPhases?: Array<{
    name: string;
    type: 'GA' | 'Launch';
    starts: string;
    ends?: string | null;
    policy?: any;
  }>;
}

type WizardStep = 'basic' | 'policy' | 'pricing' | 'review';

interface PhaseFormData {
  name: string;
  type: 'GA' | 'Launch';
  starts: Date | undefined;
  ends: Date | undefined;
  copyPolicyFrom: string;
  customizePolicy: boolean;
  policy: {
    minLabelLength?: number;
    maxLabelLength?: number;
    registrationGP?: number;
    renewalGP?: number;
    autoRenewalGP?: number;
    transferGP?: number;
    redemptionGP?: number;
    pendingdeleteGP?: number;
    transferLockPeriod?: number;
    maxHorizon?: number;
    allowAutorenew?: boolean;
    requiresValidation?: boolean;
    baseCurrency?: string;
  };
}

export function PhaseCreateWizard({ tldName, open, onClose, existingPhases = [] }: PhaseCreateWizardProps) {
  const router = useRouter();
  const { mutate: createPhase, isPending, error } = useCreatePhase(tldName);
  
  const [step, setStep] = useState<WizardStep>('basic');
  const [formData, setFormData] = useState<PhaseFormData>({
    name: '',
    type: 'GA',
    starts: undefined,
    ends: undefined,
    copyPolicyFrom: '',
    customizePolicy: false,
    policy: {},
  });

  const [validationErrors, setValidationErrors] = useState<string[]>([]);

  const validateBasicInfo = (): boolean => {
    const errors: string[] = [];

    if (!formData.name.trim()) {
      errors.push('Phase name is required');
    }
    if (formData.name.length < 3) {
      errors.push('Phase name must be at least 3 characters');
    }
    if (formData.name.length > 16) {
      errors.push('Phase name must be 16 characters or less');
    }
    // Check for invalid characters (only lowercase, numbers, hyphens allowed)
    if (!/^[a-z0-9-]+$/.test(formData.name)) {
      errors.push('Phase name can only contain lowercase letters, numbers, and hyphens');
    }
    if (!formData.starts) {
      errors.push('Start date is required');
    }
    if (formData.ends && formData.starts && formData.ends <= formData.starts) {
      errors.push('End date must be after start date');
    }

    // Check for name collision
    if (existingPhases.some(p => p.name.toLowerCase() === formData.name.toLowerCase())) {
      errors.push('A phase with this name already exists');
    }

    // Check for GA overlap
    if (formData.type === 'GA' && formData.starts) {
      const overlappingGA = existingPhases.filter(p => p.type === 'GA').find(p => {
        const pStart = new Date(p.starts);
        const pEnd = p.ends ? new Date(p.ends) : null;
        
        if (!formData.starts) return false;
        
        // Check various overlap scenarios
        if (!pEnd && !formData.ends) return true; // Both ongoing
        if (!pEnd) return formData.starts >= pStart; // Existing ongoing
        if (!formData.ends) return formData.starts <= pEnd; // New ongoing
        
        // Both have end dates
        return formData.starts <= pEnd && (!formData.ends || formData.ends >= pStart);
      });

      if (overlappingGA) {
        errors.push(`GA phase would overlap with existing phase "${overlappingGA.name}"`);
      }
    }

    setValidationErrors(errors);
    return errors.length === 0;
  };

  const handleNext = () => {
    if (step === 'basic' && !validateBasicInfo()) {
      return;
    }

    const steps: WizardStep[] = ['basic', 'policy', 'pricing', 'review'];
    const currentIndex = steps.indexOf(step);
    if (currentIndex < steps.length - 1) {
      setStep(steps[currentIndex + 1]);
      setValidationErrors([]);
    }
  };

  const handleBack = () => {
    const steps: WizardStep[] = ['basic', 'policy', 'pricing', 'review'];
    const currentIndex = steps.indexOf(step);
    if (currentIndex > 0) {
      setStep(steps[currentIndex - 1]);
      setValidationErrors([]);
    }
  };

  const handleSubmit = () => {
    // Final validation before submit
    if (!validateBasicInfo()) {
      setStep('basic'); // Go back to basic step if validation fails
      return;
    }

    createPhase({
      name: formData.name,
      type: formData.type,
      starts: formData.starts!.toISOString(),
      ends: formData.ends?.toISOString() || null,
    }, {
      onSuccess: () => {
        onClose();
        setStep('basic');
        setFormData({
          name: '',
          type: 'GA',
          starts: undefined,
          ends: undefined,
          copyPolicyFrom: '',
          customizePolicy: false,
          policy: {},
        });
      },
      onError: (error: any) => {
        // API errors are now logged automatically by the API client interceptor
        // Display user-friendly error message
        const errorMessage = error?.response?.data?.message || 
                            error?.response?.data?.error ||
                            error?.message || 
                            'Failed to create phase';
        setValidationErrors([errorMessage]);
        setStep('basic'); // Go back to show error
      },
    });
  };

  const getStepTitle = () => {
    switch (step) {
      case 'basic': return 'Basic Information';
      case 'policy': return 'Policy Configuration';
      case 'pricing': return 'Pricing & Fees';
      case 'review': return 'Review & Create';
    }
  };

  const getStepDescription = () => {
    switch (step) {
      case 'basic': return 'Set the phase name, type, and timeline';
      case 'policy': return 'Configure grace periods and domain policies';
      case 'pricing': return 'Set prices and fees for this phase';
      case 'review': return 'Review your configuration before creating';
    }
  };

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create New Phase for .{tldName}</DialogTitle>
          <DialogDescription>{getStepDescription()}</DialogDescription>
        </DialogHeader>

        {/* Progress Steps */}
        <div className="flex items-center justify-between mb-6">
          {(['basic', 'policy', 'pricing', 'review'] as WizardStep[]).map((s, idx) => (
            <div key={s} className="flex items-center flex-1">
              <div className={cn(
                'flex items-center justify-center w-8 h-8 rounded-full border-2 transition-colors',
                step === s && 'border-primary bg-primary text-primary-foreground',
                ['basic', 'policy', 'pricing', 'review'].indexOf(step) > idx && 'border-primary bg-primary text-primary-foreground',
                ['basic', 'policy', 'pricing', 'review'].indexOf(step) <= idx && step !== s && 'border-muted-foreground/30'
              )}>
                <span className="text-xs font-medium">{idx + 1}</span>
              </div>
              {idx < 3 && (
                <div className={cn(
                  'flex-1 h-0.5 mx-2',
                  ['basic', 'policy', 'pricing', 'review'].indexOf(step) > idx ? 'bg-primary' : 'bg-muted-foreground/30'
                )} />
              )}
            </div>
          ))}
        </div>

        {/* Validation Errors */}
        {validationErrors.length > 0 && (
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>
              <ul className="list-disc list-inside space-y-1">
                {validationErrors.map((error, idx) => (
                  <li key={idx}>{error}</li>
                ))}
              </ul>
            </AlertDescription>
          </Alert>
        )}

        {/* Step Content */}
        <div className="space-y-4">
          {step === 'basic' && (
            <BasicInfoStep formData={formData} setFormData={setFormData} />
          )}
          {step === 'policy' && (
            <PolicyStep 
              formData={formData} 
              setFormData={setFormData}
              existingPhases={existingPhases}
            />
          )}
          {step === 'pricing' && (
            <PricingStep formData={formData} setFormData={setFormData} />
          )}
          {step === 'review' && (
            <ReviewStep formData={formData} tldName={tldName} />
          )}
        </div>

        {/* Navigation */}
        <DialogFooter className="flex justify-between">
          <div className="flex gap-2">
            {step !== 'basic' && (
              <Button variant="outline" onClick={handleBack}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                Back
              </Button>
            )}
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={onClose}>
              Cancel
            </Button>
            {step !== 'review' ? (
              <Button onClick={handleNext}>
                Next
                <ArrowRight className="h-4 w-4 ml-2" />
              </Button>
            ) : (
              <Button onClick={handleSubmit} disabled={isPending}>
                {isPending ? 'Creating...' : 'Create Phase'}
              </Button>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// Step 1: Basic Information
function BasicInfoStep({ formData, setFormData }: {
  formData: PhaseFormData;
  setFormData: React.Dispatch<React.SetStateAction<PhaseFormData>>;
}) {
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="name">Phase Name *</Label>
        <Input
          id="name"
          placeholder="e.g., sunrise, ga-1, landrush"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value.toLowerCase() })}
          maxLength={16}
          className={formData.name.length > 16 || (formData.name.length > 0 && formData.name.length < 3) ? 'border-destructive' : ''}
        />
        <div className="flex justify-between items-center text-xs">
          <p className="text-muted-foreground">
            3-16 characters: lowercase, numbers, hyphens only
          </p>
          <p className={`${
            formData.name.length > 16 ? 'text-destructive font-medium' :
            formData.name.length >= 3 ? 'text-green-600' :
            formData.name.length > 0 ? 'text-orange-500' :
            'text-muted-foreground'
          }`}>
            {formData.name.length}/16
          </p>
        </div>
      </div>

      <div className="space-y-2">
        <Label>Phase Type *</Label>
        <RadioGroup
          value={formData.type}
          onValueChange={(value) => setFormData({ ...formData, type: value as 'GA' | 'Launch' })}
        >
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="GA" id="ga" />
            <Label htmlFor="ga" className="font-normal cursor-pointer">
              <div>
                <div className="font-medium">General Availability (GA)</div>
                <div className="text-xs text-muted-foreground">
                  Only one GA phase can be active at a time. Cannot overlap with other GA phases.
                </div>
              </div>
            </Label>
          </div>
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="Launch" id="launch" />
            <Label htmlFor="launch" className="font-normal cursor-pointer">
              <div>
                <div className="font-medium">Launch</div>
                <div className="text-xs text-muted-foreground">
                  Can have multiple active. Can overlap with GA and other Launch phases.
                </div>
              </div>
            </Label>
          </div>
        </RadioGroup>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Start Date *</Label>
          <Popover>
            <PopoverTrigger asChild>
              <Button
                variant="outline"
                className={cn(
                  'w-full justify-start text-left font-normal',
                  !formData.starts && 'text-muted-foreground'
                )}
              >
                <CalendarIcon className="mr-2 h-4 w-4" />
                {formData.starts ? format(formData.starts, 'PPP') : 'Pick a date'}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0">
              <Calendar
                mode="single"
                selected={formData.starts}
                onSelect={(date) => setFormData({ ...formData, starts: date })}
              />
            </PopoverContent>
          </Popover>
        </div>

        <div className="space-y-2">
          <Label>End Date (Optional)</Label>
          <Popover>
            <PopoverTrigger asChild>
              <Button
                variant="outline"
                className={cn(
                  'w-full justify-start text-left font-normal',
                  !formData.ends && 'text-muted-foreground'
                )}
              >
                <CalendarIcon className="mr-2 h-4 w-4" />
                {formData.ends ? format(formData.ends, 'PPP') : 'Ongoing'}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0">
              <Calendar
                mode="single"
                selected={formData.ends}
                onSelect={(date) => setFormData({ ...formData, ends: date })}
                disabled={(date) => formData.starts ? date <= formData.starts : false}
              />
            </PopoverContent>
          </Popover>
        </div>
      </div>
    </div>
  );
}

// Step 2: Policy Configuration
function PolicyStep({ formData, setFormData, existingPhases }: {
  formData: PhaseFormData;
  setFormData: React.Dispatch<React.SetStateAction<PhaseFormData>>;
  existingPhases: any[];
}) {
  return (
    <div className="space-y-4">
      <Alert>
        <CheckCircle2 className="h-4 w-4" />
        <AlertDescription>
          Policy configuration will use default values. You can customize these later in Phase 2.
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Default Policy Preview</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <div className="text-xs text-muted-foreground">Min Label Length</div>
              <div className="font-medium">1</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Max Label Length</div>
              <div className="font-medium">63</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Registration GP</div>
              <div className="font-medium">5 days</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Renewal GP</div>
              <div className="font-medium">5 days</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Transfer GP</div>
              <div className="font-medium">5 days</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Redemption GP</div>
              <div className="font-medium">30 days</div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

// Step 3: Pricing (Placeholder)
function PricingStep({ formData, setFormData }: {
  formData: PhaseFormData;
  setFormData: React.Dispatch<React.SetStateAction<PhaseFormData>>;
}) {
  return (
    <div className="space-y-4">
      <Alert>
        <CheckCircle2 className="h-4 w-4" />
        <AlertDescription>
          Pricing and fees will be added in Phase 2. The phase will be created with default values.
        </AlertDescription>
      </Alert>
    </div>
  );
}

// Step 4: Review
function ReviewStep({ formData, tldName }: {
  formData: PhaseFormData;
  tldName: string;
}) {
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Phase Configuration</CardTitle>
          <CardDescription>Review the details before creating</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-3 items-center gap-4">
            <span className="font-semibold">TLD:</span>
            <span className="col-span-2">.{tldName}</span>
          </div>
          <div className="grid grid-cols-3 items-center gap-4">
            <span className="font-semibold">Name:</span>
            <span className="col-span-2">{formData.name}</span>
          </div>
          <div className="grid grid-cols-3 items-center gap-4">
            <span className="font-semibold">Type:</span>
            <span className="col-span-2">
              <Badge variant={formData.type === 'GA' ? 'default' : 'secondary'}>
                {formData.type}
              </Badge>
            </span>
          </div>
          <div className="grid grid-cols-3 items-center gap-4">
            <span className="font-semibold">Starts:</span>
            <span className="col-span-2">
              {formData.starts && format(formData.starts, 'PPP')}
            </span>
          </div>
          <div className="grid grid-cols-3 items-center gap-4">
            <span className="font-semibold">Ends:</span>
            <span className="col-span-2">
              {formData.ends ? format(formData.ends, 'PPP') : <span className="text-muted-foreground italic">Ongoing</span>}
            </span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
