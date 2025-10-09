'use client';

import { Phase } from '@/lib/types/phase';
import { useState } from 'react';
import { useDeletePhase, useEndPhase } from '@/lib/hooks/usePhases';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Calendar as CalendarComponent } from '@/components/ui/calendar';
import { formatPhaseDateLong, formatRelativeDate, isPhaseFuture, isPhaseCurrent } from '@/lib/utils/dateUtils';
import { Calendar, DollarSign, Settings, Tag, Trash2, GitCompare, CalendarX, Info, Clock } from 'lucide-react';
import { PhaseConfigDiff } from './PhaseConfigDiff';
import { format } from 'date-fns';

interface PhaseDetailDrawerProps {
  phase: Phase | null;
  open: boolean;
  onClose: () => void;
  tldName?: string;
  allPhases?: Phase[];
}

export function PhaseDetailDrawer({ phase, open, onClose, tldName, allPhases = [] }: PhaseDetailDrawerProps) {
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [showEndPhaseDialog, setShowEndPhaseDialog] = useState(false);
  const [showDiff, setShowDiff] = useState(false);
  const [endDate, setEndDate] = useState<Date | undefined>(undefined);
  
  const { mutate: deletePhase, isPending: isDeleting } = useDeletePhase(tldName || '');
  const { mutate: endPhase, isPending: isEnding } = useEndPhase(tldName || '');

  if (!phase) return null;

  const isFuture = isPhaseFuture(phase.starts);
  const isCurrent = isPhaseCurrent(phase.starts, phase.ends);
  const canDelete = isFuture;
  const canEnd = isCurrent || isFuture;
  const hasNoEndDate = !phase.ends;
  
  // Find previous phase for diff comparison
  const phasesOfSameType = allPhases
    .filter(p => p.type === phase.type)
    .sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());
  const currentIndex = phasesOfSameType.findIndex(p => p.id === phase.id);
  const previousPhase = currentIndex > 0 ? phasesOfSameType[currentIndex - 1] : null;

  // Helper to format date with time in UTC
  const formatDateWithTime = (dateString: string) => {
    const date = new Date(dateString);
    return {
      date: format(date, 'MMMM d, yyyy'),
      time: format(date, 'HH:mm:ss'),
    };
  };

  const handleDelete = () => {
    deletePhase(phase.name, {
      onSuccess: () => {
        setShowDeleteDialog(false);
        onClose();
      },
    });
  };

  const handleEndPhase = () => {
    if (!endDate) return;
    
    endPhase({
      phaseName: phase.name,
      endDate: endDate.toISOString(),
    }, {
      onSuccess: () => {
        setShowEndPhaseDialog(false);
        setEndDate(undefined);
        onClose();
      },
    });
  };

  // Helper to get currency symbol
  const getCurrencySymbol = (currency: string): string => {
    const symbols: Record<string, string> = {
      'USD': '$',
      'EUR': '€',
      'GBP': '£',
      'JPY': '¥',
      'CNY': '¥',
      'INR': '₹',
      'AUD': 'A$',
      'CAD': 'C$',
      'CHF': 'Fr',
      'SEK': 'kr',
      'NZD': 'NZ$',
      'KRW': '₩',
      'SGD': 'S$',
      'HKD': 'HK$',
      'NOK': 'kr',
      'MXN': '$',
      'ZAR': 'R',
      'BRL': 'R$',
      'RUB': '₽',
      'TRY': '₺',
    };
    const upperCurrency = currency?.toUpperCase() || '';
    return symbols[upperCurrency] || '$';
  };

  return (
    <>
      <Sheet open={open} onOpenChange={onClose}>
        <SheetContent className="w-full sm:max-w-2xl overflow-y-auto">
          <SheetHeader className="pb-6 border-b space-y-4">
            {/* Title and Badge */}
            <div className="space-y-3">
              <div className="flex items-baseline gap-3">
                <SheetTitle className="text-4xl font-bold">{phase.name}</SheetTitle>
                <Badge 
                  variant={phase.type === 'GA' ? 'default' : 'secondary'}
                  className="text-xs"
                >
                  {phase.type}
                </Badge>
              </div>
              <SheetDescription className="text-sm text-muted-foreground">
                {isPhaseCurrent(phase.starts, phase.ends) 
                  ? '🟢 Currently active' 
                  : isPhaseFuture(phase.starts)
                  ? '🔵 Scheduled'
                  : '⚫ Past'}
              </SheetDescription>
            </div>
            
            {/* Action Buttons */}
            <div className="flex flex-wrap gap-2">
              {previousPhase && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setShowDiff(!showDiff)}
                >
                  <GitCompare className="h-4 w-4 mr-1.5" />
                  {showDiff ? 'Hide' : 'Show'} Diff
                </Button>
              )}
              {canDelete && (
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => setShowDeleteDialog(true)}
                >
                  <Trash2 className="h-4 w-4 mr-1.5" />
                  Delete
                </Button>
              )}
            </div>
          </SheetHeader>

          <div className="mt-6 space-y-6">
            {/* Configuration Diff */}
            {showDiff && previousPhase && (
              <PhaseConfigDiff phase={phase} compareWith={previousPhase} />
            )}

            {/* Timeline Section - Always Visible */}
            <div className="space-y-4">
              {/* Start Date */}
              <div className="space-y-1.5">
                <div className="text-xs text-muted-foreground uppercase tracking-wide">
                  {isFuture ? 'Starts' : 'Started'}
                </div>
                <div className="text-3xl font-bold">{formatDateWithTime(phase.starts).date}</div>
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Clock className="h-3.5 w-3.5" />
                  <span>{formatDateWithTime(phase.starts).time} UTC</span>
                  <span>•</span>
                  <span>{formatRelativeDate(phase.starts)}</span>
                </div>
              </div>
              
              {/* End Date */}
              <div className={`space-y-1.5 ${!phase.ends ? 'opacity-60' : ''}`}>
                <div className="flex items-center justify-between">
                  <div className="text-xs text-muted-foreground uppercase tracking-wide">Ends</div>
                  {!phase.ends && canEnd && (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => setShowEndPhaseDialog(true)}
                      className="h-7 text-xs"
                    >
                      <CalendarX className="h-3.5 w-3.5 mr-1" />
                      Set End Date
                    </Button>
                  )}
                </div>
                {phase.ends ? (
                  <>
                    <div className="text-3xl font-bold">{formatDateWithTime(phase.ends).date}</div>
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Clock className="h-3.5 w-3.5" />
                      <span>{formatDateWithTime(phase.ends).time} UTC</span>
                      <span>•</span>
                      <span>{formatRelativeDate(phase.ends)}</span>
                    </div>
                  </>
                ) : (
                  <div className="text-lg font-semibold italic text-muted-foreground">
                    {isCurrent ? 'Ongoing (no end date)' : 'Not set'}
                  </div>
                )}
              </div>
            </div>

            {/* Expandable Sections */}
            <Accordion type="multiple" defaultValue={["policy"]} className="space-y-2">
              {/* Policy Configuration */}
              <AccordionItem value="policy" className="border rounded-lg px-4">
                <AccordionTrigger className="hover:no-underline">
                  <div className="flex items-center gap-2 text-sm font-semibold">
                    <Settings className="h-4 w-4 text-orange-600" />
                    Policy Configuration
                  </div>
                </AccordionTrigger>
                <AccordionContent>
                  <div className="grid grid-cols-2 gap-x-6 gap-y-4 pt-2">
                    {/* Label Length - Visual representation */}
                    {(phase.policy.minLabelLength !== undefined || phase.policy.maxLabelLength !== undefined) && (
                      <div className="col-span-2 space-y-2">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Domain Label Length</div>
                        <div className="flex items-center gap-3">
                          <div className="flex-1">
                            <div className="h-2 bg-muted rounded-full overflow-hidden">
                              <div 
                                className="h-full bg-orange-500"
                                style={{ 
                                  marginLeft: `${((phase.policy.minLabelLength || 1) - 1) / 62 * 100}%`,
                                  width: `${((phase.policy.maxLabelLength || 63) - (phase.policy.minLabelLength || 1)) / 62 * 100}%` 
                                }}
                              />
                            </div>
                            <div className="flex justify-between text-xs text-muted-foreground mt-1">
                              <span>1</span>
                              <span>63</span>
                            </div>
                          </div>
                          <div className="text-sm font-semibold min-w-[80px] text-right">
                            {phase.policy.minLabelLength || 1}–{phase.policy.maxLabelLength || 63} chars
                          </div>
                        </div>
                      </div>
                    )}
                    
                    {phase.policy.registrationGP !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Registration GP</div>
                        <div className="font-semibold">{phase.policy.registrationGP} days</div>
                      </div>
                    )}
                    {phase.policy.renewalGP !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Renewal GP</div>
                        <div className="font-semibold">{phase.policy.renewalGP} days</div>
                      </div>
                    )}
                    {phase.policy.autoRenewalGP !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Auto Renewal GP</div>
                        <div className="font-semibold">{phase.policy.autoRenewalGP} days</div>
                      </div>
                    )}
                    {phase.policy.transferGP !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Transfer GP</div>
                        <div className="font-semibold">{phase.policy.transferGP} days</div>
                      </div>
                    )}
                    {phase.policy.redemptionGP !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Redemption GP</div>
                        <div className="font-semibold">{phase.policy.redemptionGP} days</div>
                      </div>
                    )}
                    {phase.policy.pendingdeleteGP !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Pending Delete GP</div>
                        <div className="font-semibold">{phase.policy.pendingdeleteGP} days</div>
                      </div>
                    )}
                    {phase.policy.transferLockPeriod !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Transfer Lock</div>
                        <div className="font-semibold">{phase.policy.transferLockPeriod} days</div>
                      </div>
                    )}
                    {phase.policy.maxHorizon !== undefined && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Max Horizon</div>
                        <div className="font-semibold">{phase.policy.maxHorizon} years</div>
                      </div>
                    )}
                    {phase.policy.allowAutorenew !== undefined && (
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Allow Autorenew</div>
                        <div className="flex items-center gap-2">
                          <div className={`inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors ${phase.policy.allowAutorenew ? 'bg-primary' : 'bg-input'}`}>
                            <div className={`h-4 w-4 rounded-full bg-background shadow-lg transition-transform ${phase.policy.allowAutorenew ? 'translate-x-4' : 'translate-x-0'}`} />
                          </div>
                          <span className="text-sm font-medium">{phase.policy.allowAutorenew ? 'Enabled' : 'Disabled'}</span>
                        </div>
                      </div>
                    )}
                    {phase.policy.requiresValidation !== undefined && (
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Requires Validation</div>
                        <div className="flex items-center gap-2">
                          <div className={`inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors ${phase.policy.requiresValidation ? 'bg-primary' : 'bg-input'}`}>
                            <div className={`h-4 w-4 rounded-full bg-background shadow-lg transition-transform ${phase.policy.requiresValidation ? 'translate-x-4' : 'translate-x-0'}`} />
                          </div>
                          <span className="text-sm font-medium">{phase.policy.requiresValidation ? 'Required' : 'Not Required'}</span>
                        </div>
                      </div>
                    )}
                    {phase.policy.baseCurrency && (
                      <div className="space-y-1">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Base Currency</div>
                        <div className="font-semibold">{getCurrencySymbol(phase.policy.baseCurrency)} ({phase.policy.baseCurrency})</div>
                      </div>
                    )}
                  </div>
                </AccordionContent>
              </AccordionItem>

              {/* Pricing */}
              {phase.prices && phase.prices.length > 0 && (
                <AccordionItem value="pricing" className="border rounded-lg px-4">
                  <AccordionTrigger className="hover:no-underline">
                    <div className="flex items-center gap-2 text-sm font-semibold">
                      <DollarSign className="h-4 w-4 text-orange-600" />
                      Pricing
                      <Badge variant="secondary" className="text-xs ml-2">{phase.prices.length}</Badge>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <div className="space-y-2 pt-2">
                      {phase.prices.map((price) => (
                        <div key={price.id} className="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                          <span className="text-sm font-medium text-muted-foreground uppercase">{price.currency}</span>
                          <span className="text-xl font-bold">
                            {getCurrencySymbol(price.currency)}{(price.amount / 100).toFixed(2)}
                          </span>
                        </div>
                      ))}
                    </div>
                  </AccordionContent>
                </AccordionItem>
              )}

              {/* Fees */}
              {phase.fees && phase.fees.length > 0 && (
                <AccordionItem value="fees" className="border rounded-lg px-4">
                  <AccordionTrigger className="hover:no-underline">
                    <div className="flex items-center gap-2 text-sm font-semibold">
                      <Tag className="h-4 w-4 text-orange-600" />
                      Fees
                      <Badge variant="secondary" className="text-xs ml-2">{phase.fees.length}</Badge>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <div className="space-y-2 pt-2">
                      {phase.fees.map((fee) => (
                        <div key={fee.id} className="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                          <span className="text-sm font-medium">{fee.name}</span>
                          <span className="text-lg font-semibold">
                            {getCurrencySymbol(fee.currency)}{(fee.amount / 100).toFixed(2)}
                          </span>
                        </div>
                      ))}
                    </div>
                  </AccordionContent>
                </AccordionItem>
              )}

              {/* Premium List */}
              {phase.premiumListName && (
                <AccordionItem value="premium" className="border rounded-lg px-4">
                  <AccordionTrigger className="hover:no-underline">
                    <div className="flex items-center gap-2 text-sm font-semibold">
                      <Tag className="h-4 w-4 text-orange-600" />
                      Premium List
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <div className="pt-2">
                      <Badge variant="outline" className="text-sm font-mono">{phase.premiumListName}</Badge>
                    </div>
                  </AccordionContent>
                </AccordionItem>
              )}
            </Accordion>

            {/* Metadata */}
            <div className="pt-4 border-t space-y-2 text-xs text-muted-foreground">
              <div className="flex items-center gap-2">
                <Info className="h-3 w-3" />
                <span>Created: {new Date(phase.createdAt).toLocaleString()}</span>
              </div>
              <div className="pl-5">
                <span>Updated: {new Date(phase.updatedAt).toLocaleString()}</span>
              </div>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Phase?</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the phase &quot;{phase.name}&quot;?
              <br /><br />
              <strong>Note:</strong> Only future phases can be deleted. Current and past phases are kept for traceability.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={isDeleting}
              className="bg-destructive text-destructive-foreground"
            >
              {isDeleting ? 'Deleting...' : 'Delete Phase'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* End Phase Dialog */}
      <AlertDialog open={showEndPhaseDialog} onOpenChange={setShowEndPhaseDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Set End Date for Phase</AlertDialogTitle>
            <AlertDialogDescription>
              Choose when this phase should end. The phase &quot;{phase.name}&quot; will become inactive after this date.
            </AlertDialogDescription>
          </AlertDialogHeader>
          
          <div className="py-4">
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  className="w-full justify-start text-left font-normal"
                >
                  <CalendarX className="mr-2 h-4 w-4" />
                  {endDate ? format(endDate, 'PPP') : 'Pick an end date'}
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-auto p-0" align="start">
                <CalendarComponent
                  mode="single"
                  selected={endDate}
                  onSelect={setEndDate}
                  disabled={(date) => date < new Date(phase.starts) || date < new Date()}
                  initialFocus
                />
              </PopoverContent>
            </Popover>
            <p className="text-xs text-muted-foreground mt-2">
              End date must be after the start date ({formatPhaseDateLong(phase.starts)}) and in the future.
            </p>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setEndDate(undefined)}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleEndPhase}
              disabled={isEnding || !endDate}
              className="bg-orange-600 hover:bg-orange-700"
            >
              {isEnding ? 'Setting End Date...' : 'Set End Date'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
