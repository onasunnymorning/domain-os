'use client';

import { useState, useEffect } from 'react';
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
import { CalendarIcon, AlertCircle, CheckCircle2, ArrowRight, ArrowLeft, Clock } from 'lucide-react';
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

type WizardStep = 'basic' | 'review';

interface PhaseFormData {
  name: string;
  type: 'GA' | 'Launch';
  starts: Date | undefined;
  startsTime: { hours: string; minutes: string };
  ends: Date | undefined;
  endsTime: { hours: string; minutes: string };
}

export function PhaseCreateWizard({ tldName, open, onClose, existingPhases = [] }: PhaseCreateWizardProps) {
  const router = useRouter();
  const { mutate: createPhase, isPending, error } = useCreatePhase(tldName);
  
  const [step, setStep] = useState<WizardStep>('basic');
  const [formData, setFormData] = useState<PhaseFormData>({
    name: '',
    type: 'GA',
    starts: undefined,
    startsTime: { hours: '00', minutes: '00' },
    ends: undefined,
    endsTime: { hours: '00', minutes: '00' },
  });

  const [validationErrors, setValidationErrors] = useState<string[]>([]);

  // Auto-populate start date from previous phase's end date based on selected type
  useEffect(() => {
    if (open && existingPhases.length > 0) {
      // Find the most recent phase of the same type with an end date
      const phasesOfSameType = existingPhases.filter(p => p.type === formData.type && p.ends);
      
      if (phasesOfSameType.length > 0) {
        // Get the most recent phase of the same type
        const previousPhase = phasesOfSameType.sort((a, b) => 
          new Date(b.ends!).getTime() - new Date(a.ends!).getTime()
        )[0];

        if (previousPhase.ends) {
          const endDate = new Date(previousPhase.ends);
          setFormData(prev => ({
            ...prev,
            starts: endDate,
            startsTime: {
              hours: endDate.getUTCHours().toString().padStart(2, '0'),
              minutes: endDate.getUTCMinutes().toString().padStart(2, '0'),
            },
          }));
        }
      } else {
        // No previous phase of this type, clear the start date
        setFormData(prev => ({
          ...prev,
          starts: undefined,
          startsTime: { hours: '00', minutes: '00' },
        }));
      }
    } else if (open) {
      // No existing phases at all, clear the start date
      setFormData(prev => ({
        ...prev,
        starts: undefined,
        startsTime: { hours: '00', minutes: '00' },
      }));
    }
  }, [open, existingPhases, formData.type]);

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

    const steps: WizardStep[] = ['basic', 'review'];
    const currentIndex = steps.indexOf(step);
    if (currentIndex < steps.length - 1) {
      setStep(steps[currentIndex + 1]);
      setValidationErrors([]);
    }
  };

  const handleBack = () => {
    const steps: WizardStep[] = ['basic', 'review'];
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

    // Combine date and time for starts
    const startsDate = new Date(formData.starts!);
    startsDate.setUTCHours(parseInt(formData.startsTime.hours), parseInt(formData.startsTime.minutes), 0, 0);

    // Combine date and time for ends if provided
    let endsDate = null;
    if (formData.ends) {
      endsDate = new Date(formData.ends);
      endsDate.setUTCHours(parseInt(formData.endsTime.hours), parseInt(formData.endsTime.minutes), 0, 0);
    }

    createPhase({
      name: formData.name,
      type: formData.type,
      starts: startsDate.toISOString(),
      ends: endsDate?.toISOString() || null,
    }, {
      onSuccess: () => {
        onClose();
        setStep('basic');
        setFormData({
          name: '',
          type: 'GA',
          starts: undefined,
          startsTime: { hours: '00', minutes: '00' },
          ends: undefined,
          endsTime: { hours: '00', minutes: '00' },
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
      case 'review': return 'Review & Create';
    }
  };

  const getStepDescription = () => {
    switch (step) {
      case 'basic': return 'Set the phase name, type, and timeline';
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

      <div className="space-y-4">
        {/* Start Date and Time */}
        <div className="space-y-2">
          <Label>Start Date & Time (UTC) *</Label>
          <div className="flex gap-2">
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  className={cn(
                    'flex-1 justify-start text-left font-normal',
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
            <div className="flex gap-1">
              <Input
                type="number"
                min="0"
                max="23"
                value={formData.startsTime.hours}
                onChange={(e) => {
                  const val = Math.min(23, Math.max(0, parseInt(e.target.value) || 0));
                  setFormData({
                    ...formData,
                    startsTime: { ...formData.startsTime, hours: val.toString().padStart(2, '0') }
                  });
                }}
                className="w-16 text-center"
                placeholder="HH"
              />
              <span className="flex items-center">:</span>
              <Input
                type="number"
                min="0"
                max="59"
                value={formData.startsTime.minutes}
                onChange={(e) => {
                  const val = Math.min(59, Math.max(0, parseInt(e.target.value) || 0));
                  setFormData({
                    ...formData,
                    startsTime: { ...formData.startsTime, minutes: val.toString().padStart(2, '0') }
                  });
                }}
                className="w-16 text-center"
                placeholder="MM"
              />
            </div>
          </div>
          <p className="text-xs text-muted-foreground flex items-center gap-1">
            <Clock className="h-3 w-3" />
            Time is in UTC. Current: {formData.startsTime.hours}:{formData.startsTime.minutes}
          </p>
        </div>

        {/* End Date and Time */}
        <div className="space-y-2">
          <Label>End Date & Time (UTC) - Optional</Label>
          <div className="flex gap-2">
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  className={cn(
                    'flex-1 justify-start text-left font-normal',
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
            {formData.ends && (
              <div className="flex gap-1">
                <Input
                  type="number"
                  min="0"
                  max="23"
                  value={formData.endsTime.hours}
                  onChange={(e) => {
                    const val = Math.min(23, Math.max(0, parseInt(e.target.value) || 0));
                    setFormData({
                      ...formData,
                      endsTime: { ...formData.endsTime, hours: val.toString().padStart(2, '0') }
                    });
                  }}
                  className="w-16 text-center"
                  placeholder="HH"
                />
                <span className="flex items-center">:</span>
                <Input
                  type="number"
                  min="0"
                  max="59"
                  value={formData.endsTime.minutes}
                  onChange={(e) => {
                    const val = Math.min(59, Math.max(0, parseInt(e.target.value) || 0));
                    setFormData({
                      ...formData,
                      endsTime: { ...formData.endsTime, minutes: val.toString().padStart(2, '0') }
                    });
                  }}
                  className="w-16 text-center"
                  placeholder="MM"
                />
              </div>
            )}
          </div>
          {formData.ends && (
            <p className="text-xs text-muted-foreground flex items-center gap-1">
              <Clock className="h-3 w-3" />
              Time is in UTC. Current: {formData.endsTime.hours}:{formData.endsTime.minutes}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

// Review Step
function ReviewStep({ formData, tldName }: {
  formData: PhaseFormData;
  tldName: string;
}) {
  const formatDateTime = (date: Date | undefined, time: { hours: string; minutes: string }) => {
    if (!date) return null;
    return `${format(date, 'PPP')} at ${time.hours}:${time.minutes} UTC`;
  };

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
              {formatDateTime(formData.starts, formData.startsTime)}
            </span>
          </div>
          <div className="grid grid-cols-3 items-center gap-4">
            <span className="font-semibold">Ends:</span>
            <span className="col-span-2">
              {formData.ends ? (
                formatDateTime(formData.ends, formData.endsTime)
              ) : (
                <span className="text-muted-foreground italic">Ongoing</span>
              )}
            </span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
