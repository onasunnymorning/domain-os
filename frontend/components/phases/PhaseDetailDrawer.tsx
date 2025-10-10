'use client';

import { Phase } from '@/lib/types/phase';
import { useState } from 'react';
import { useDeletePhase, useEndPhase } from '@/lib/hooks/usePhases';
import { useQuery } from '@tanstack/react-query';
import { phasesApi } from '@/lib/api/phases';
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

  // Fetch full phase details with prices and fees when drawer opens
  const { data: fullPhase } = useQuery({
    queryKey: ['phase', tldName, phase?.name],
    queryFn: () => phasesApi.getPhase(tldName!, phase!.name),
    enabled: open && !!tldName && !!phase?.name,
  });

  // Use full phase data if available, otherwise fall back to the phase prop
  const phaseData = fullPhase || phase;

  if (!phaseData) return null;

  const isFuture = isPhaseFuture(phaseData.starts);
  const isCurrent = isPhaseCurrent(phaseData.starts, phaseData.ends);
  const canDelete = isFuture;
  const canEnd = isCurrent || isFuture;
  const hasNoEndDate = !phaseData.ends;
  
  // Find previous phase for diff comparison
  const phasesOfSameType = allPhases
    .filter(p => p.type === phaseData.type)
    .sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());
  const currentIndex = phasesOfSameType.findIndex(p => p.id === phaseData.id);
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
    deletePhase(phaseData.name, {
      onSuccess: () => {
        setShowDeleteDialog(false);
        onClose();
      },
    });
  };

  const handleEndPhase = () => {
    if (!endDate) return;
    
    endPhase({
      phaseName: phaseData.name,
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
      // Major Global Currencies
      'USD': '$',        // US Dollar
      'EUR': '€',        // Euro
      'GBP': '£',        // British Pound
      'JPY': '¥',        // Japanese Yen
      'CHF': 'Fr',       // Swiss Franc
      'CAD': 'C$',       // Canadian Dollar
      'AUD': 'A$',       // Australian Dollar
      'NZD': 'NZ$',      // New Zealand Dollar
      
      // Asian Currencies
      'CNY': '¥',        // Chinese Yuan
      'INR': '₹',        // Indian Rupee
      'KRW': '₩',        // South Korean Won
      'SGD': 'S$',       // Singapore Dollar
      'HKD': 'HK$',      // Hong Kong Dollar
      'THB': '฿',        // Thai Baht
      'MYR': 'RM',       // Malaysian Ringgit
      'IDR': 'Rp',       // Indonesian Rupiah
      'PHP': '₱',        // Philippine Peso
      'VND': '₫',        // Vietnamese Dong
      'TWD': 'NT$',      // Taiwan Dollar
      
      // Middle East & Africa
      'AED': 'د.إ',      // UAE Dirham
      'SAR': '﷼',        // Saudi Riyal
      'ZAR': 'R',        // South African Rand
      'ILS': '₪',        // Israeli Shekel
      'EGP': 'E£',       // Egyptian Pound
      
      // European Currencies
      'SEK': 'kr',       // Swedish Krona
      'NOK': 'kr',       // Norwegian Krone
      'DKK': 'kr',       // Danish Krone
      'PLN': 'zł',       // Polish Zloty
      'CZK': 'Kč',       // Czech Koruna
      'HUF': 'Ft',       // Hungarian Forint
      'RON': 'lei',      // Romanian Leu
      'RUB': '₽',        // Russian Ruble
      'TRY': '₺',        // Turkish Lira
      'UAH': '₴',        // Ukrainian Hryvnia
      
      // Latin American Currencies
      'MXN': '$',        // Mexican Peso
      'BRL': 'R$',       // Brazilian Real
      'ARS': '$',        // Argentine Peso
      'CLP': '$',        // Chilean Peso
      'COP': '$',        // Colombian Peso
      'PEN': 'S/',       // Peruvian Sol
      'UYU': '$U',       // Uruguayan Peso
      'CRC': '₡',        // Costa Rican Colón
      
      // Other
      'PKR': '₨',        // Pakistani Rupee
      'BDT': '৳',        // Bangladeshi Taka
      'LKR': 'Rs',       // Sri Lankan Rupee
      'NPR': 'Rs',       // Nepalese Rupee
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
                <SheetTitle className="text-4xl font-bold">{phaseData.name}</SheetTitle>
                <Badge 
                  variant={phaseData.type === 'GA' ? 'default' : 'secondary'}
                  className="text-xs"
                >
                  {phaseData.type}
                </Badge>
              </div>
              <SheetDescription className="text-sm text-muted-foreground">
                {isPhaseCurrent(phaseData.starts, phaseData.ends) 
                  ? '🟢 Currently active' 
                  : isPhaseFuture(phaseData.starts)
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
              <PhaseConfigDiff phase={phaseData} compareWith={previousPhase} />
            )}

            {/* Timeline Section - Always Visible */}
            <div className="space-y-4">
              {/* Start Date */}
              <div className="space-y-1.5">
                <div className="text-xs text-muted-foreground uppercase tracking-wide">
                  {isFuture ? 'Starts' : 'Started'}
                </div>
                <div className="text-3xl font-bold">{formatDateWithTime(phaseData.starts).date}</div>
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Clock className="h-3.5 w-3.5" />
                  <span>{formatDateWithTime(phaseData.starts).time} UTC</span>
                  <span>•</span>
                  <span>{formatRelativeDate(phaseData.starts)}</span>
                </div>
              </div>
              
              {/* End Date */}
              <div className={`space-y-1.5 ${!phaseData.ends ? 'opacity-60' : ''}`}>
                <div className="flex items-center justify-between">
                  <div className="text-xs text-muted-foreground uppercase tracking-wide">Ends</div>
                  {!phaseData.ends && canEnd && (
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
                {phaseData.ends ? (
                  <>
                    <div className="text-3xl font-bold">{formatDateWithTime(phaseData.ends).date}</div>
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Clock className="h-3.5 w-3.5" />
                      <span>{formatDateWithTime(phaseData.ends).time} UTC</span>
                      <span>•</span>
                      <span>{formatRelativeDate(phaseData.ends)}</span>
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
            <Accordion type="multiple" defaultValue={[]} className="space-y-2">
              {/* Pricing */}
              {phaseData.prices && phaseData.prices.length > 0 && (
                <AccordionItem value="pricing" className="border rounded-lg px-4">
                  <AccordionTrigger className="hover:no-underline">
                    <div className="flex items-center gap-2 text-sm font-semibold">
                      <DollarSign className="h-4 w-4 text-orange-600" />
                      Pricing
                      <Badge variant="secondary" className="text-xs ml-2">{phaseData.prices.length} {phaseData.prices.length === 1 ? 'currency' : 'currencies'}</Badge>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <div className="space-y-4 pt-2">
                      {phaseData.prices.map((price, index) => (
                        <div key={index} className="space-y-2">
                          <div className="text-xs font-semibold text-orange-700 uppercase tracking-wide">{price.currency}</div>
                          <div className="grid grid-cols-2 gap-3">
                            <div className="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                              <span className="text-sm font-medium text-muted-foreground">Registration</span>
                              <span className="text-lg font-bold">
                                {getCurrencySymbol(price.currency)}{(price.registrationAmount / 100).toFixed(2)}
                              </span>
                            </div>
                            <div className="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                              <span className="text-sm font-medium text-muted-foreground">Renewal</span>
                              <span className="text-lg font-bold">
                                {getCurrencySymbol(price.currency)}{(price.renewalAmount / 100).toFixed(2)}
                              </span>
                            </div>
                            <div className="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                              <span className="text-sm font-medium text-muted-foreground">Transfer</span>
                              <span className="text-lg font-bold">
                                {getCurrencySymbol(price.currency)}{(price.transferAmount / 100).toFixed(2)}
                              </span>
                            </div>
                            <div className="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                              <span className="text-sm font-medium text-muted-foreground">Restore</span>
                              <span className="text-lg font-bold">
                                {getCurrencySymbol(price.currency)}{(price.restoreAmount / 100).toFixed(2)}
                              </span>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </AccordionContent>
                </AccordionItem>
              )}

              {/* Fees */}
              {phaseData.fees && phaseData.fees.length > 0 && (
                <AccordionItem value="fees" className="border rounded-lg px-4">
                  <AccordionTrigger className="hover:no-underline">
                    <div className="flex items-center gap-2 text-sm font-semibold">
                      <Tag className="h-4 w-4 text-orange-600" />
                      Fees
                      <Badge variant="secondary" className="text-xs ml-2">{phaseData.fees.length}</Badge>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <div className="space-y-2 pt-2">
                      {phaseData.fees.map((fee, index) => (
                        <div key={index} className="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                          <div className="flex flex-col">
                            <span className="text-sm font-medium">{fee.name}</span>
                            <span className="text-xs text-muted-foreground">{fee.currency}{fee.refundable ? ' • Refundable' : ''}</span>
                          </div>
                          <span className="text-lg font-semibold">
                            {getCurrencySymbol(fee.currency)}{(fee.amount / 100).toFixed(2)}
                          </span>
                        </div>
                      ))}
                    </div>
                  </AccordionContent>
                </AccordionItem>
              )}

              {/* Policy */}
              <AccordionItem value="policy" className="border rounded-lg px-4">
                <AccordionTrigger className="hover:no-underline">
                  <div className="flex items-center gap-2 text-sm font-semibold">
                    <Settings className="h-4 w-4 text-orange-600" />
                    Policy
                  </div>
                </AccordionTrigger>
                <AccordionContent>
                  <div className="space-y-6 pt-2">
                    {/* Label Length - Visual representation */}
                    {(phaseData.policy.minLabelLength !== undefined || phaseData.policy.maxLabelLength !== undefined) && (
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground uppercase tracking-wide">Domain Label Length</div>
                        <div className="flex items-center gap-3">
                          <div className="flex-1">
                            <div className="h-2 bg-muted rounded-full overflow-hidden">
                              <div 
                                className="h-full bg-orange-500"
                                style={{ 
                                  marginLeft: `${((phaseData.policy.minLabelLength || 1) - 1) / 62 * 100}%`,
                                  width: `${((phaseData.policy.maxLabelLength || 63) - (phaseData.policy.minLabelLength || 1)) / 62 * 100}%` 
                                }}
                              />
                            </div>
                            <div className="flex justify-between text-xs text-muted-foreground mt-1">
                              <span>1</span>
                              <span>63</span>
                            </div>
                          </div>
                          <div className="text-sm font-semibold min-w-[80px] text-right">
                            {phaseData.policy.minLabelLength || 1}–{phaseData.policy.maxLabelLength || 63} chars
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Grace Periods Section */}
                    {(phaseData.policy.registrationGP !== undefined || 
                      phaseData.policy.renewalGP !== undefined || 
                      phaseData.policy.autoRenewalGP !== undefined || 
                      phaseData.policy.transferGP !== undefined || 
                      phaseData.policy.redemptionGP !== undefined || 
                      phaseData.policy.pendingdeleteGP !== undefined) && (
                      <div className="space-y-3">
                        <div className="text-sm font-medium text-orange-700">Grace Periods</div>
                        <div className="grid grid-cols-2 gap-x-6 gap-y-3">
                          {phaseData.policy.registrationGP !== undefined && (
                            <div className="space-y-1">
                              <div className="text-xs text-muted-foreground">Registration</div>
                              <div className="font-semibold">{phaseData.policy.registrationGP} days</div>
                            </div>
                          )}
                          {phaseData.policy.renewalGP !== undefined && (
                            <div className="space-y-1">
                              <div className="text-xs text-muted-foreground">Renewal</div>
                              <div className="font-semibold">{phaseData.policy.renewalGP} days</div>
                            </div>
                          )}
                          {phaseData.policy.autoRenewalGP !== undefined && (
                            <div className="space-y-1">
                              <div className="text-xs text-muted-foreground">Auto Renewal</div>
                              <div className="font-semibold">{phaseData.policy.autoRenewalGP} days</div>
                            </div>
                          )}
                          {phaseData.policy.transferGP !== undefined && (
                            <div className="space-y-1">
                              <div className="text-xs text-muted-foreground">Transfer</div>
                              <div className="font-semibold">{phaseData.policy.transferGP} days</div>
                            </div>
                          )}
                          {phaseData.policy.redemptionGP !== undefined && (
                            <div className="space-y-1">
                              <div className="text-xs text-muted-foreground">Redemption</div>
                              <div className="font-semibold">{phaseData.policy.redemptionGP} days</div>
                            </div>
                          )}
                          {phaseData.policy.pendingdeleteGP !== undefined && (
                            <div className="space-y-1">
                              <div className="text-xs text-muted-foreground">Pending Delete</div>
                              <div className="font-semibold">{phaseData.policy.pendingdeleteGP} days</div>
                            </div>
                          )}
                        </div>
                      </div>
                    )}

                    {/* Other Policy Settings */}
                    <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                      {phaseData.policy.transferLockPeriod !== undefined && (
                        <div className="space-y-1">
                          <div className="text-xs text-muted-foreground uppercase tracking-wide">Transfer Lock</div>
                          <div className="font-semibold">{phaseData.policy.transferLockPeriod} days</div>
                        </div>
                      )}
                      {phaseData.policy.maxHorizon !== undefined && (
                        <div className="space-y-1">
                          <div className="text-xs text-muted-foreground uppercase tracking-wide">Max Horizon</div>
                          <div className="font-semibold">{phaseData.policy.maxHorizon} years</div>
                        </div>
                      )}
                      {phaseData.policy.allowAutorenew !== undefined && (
                        <div className="space-y-2">
                          <div className="text-xs text-muted-foreground uppercase tracking-wide">Allow Autorenew</div>
                          <div className="flex items-center gap-2">
                            <div className={`inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors ${phaseData.policy.allowAutorenew ? 'bg-primary' : 'bg-input'}`}>
                              <div className={`h-4 w-4 rounded-full bg-background shadow-lg transition-transform ${phaseData.policy.allowAutorenew ? 'translate-x-4' : 'translate-x-0'}`} />
                            </div>
                            <span className="text-sm font-medium">{phaseData.policy.allowAutorenew ? 'Enabled' : 'Disabled'}</span>
                          </div>
                        </div>
                      )}
                      {phaseData.policy.requiresValidation !== undefined && (
                        <div className="space-y-2">
                          <div className="text-xs text-muted-foreground uppercase tracking-wide">Requires Validation</div>
                          <div className="flex items-center gap-2">
                            <div className={`inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors ${phaseData.policy.requiresValidation ? 'bg-primary' : 'bg-input'}`}>
                              <div className={`h-4 w-4 rounded-full bg-background shadow-lg transition-transform ${phaseData.policy.requiresValidation ? 'translate-x-4' : 'translate-x-0'}`} />
                            </div>
                            <span className="text-sm font-medium">{phaseData.policy.requiresValidation ? 'Required' : 'Not Required'}</span>
                          </div>
                        </div>
                      )}
                      {phaseData.policy.baseCurrency && (
                        <div className="space-y-1">
                          <div className="text-xs text-muted-foreground uppercase tracking-wide">Base Currency</div>
                          <div className="font-semibold">{getCurrencySymbol(phaseData.policy.baseCurrency)} ({phaseData.policy.baseCurrency})</div>
                        </div>
                      )}
                    </div>
                  </div>
                </AccordionContent>
              </AccordionItem>

              {/* Premium List */}
              {phaseData.premiumListName && (
                <AccordionItem value="premium" className="border rounded-lg px-4">
                  <AccordionTrigger className="hover:no-underline">
                    <div className="flex items-center gap-2 text-sm font-semibold">
                      <Tag className="h-4 w-4 text-orange-600" />
                      Premium List
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <div className="pt-2">
                      <Badge variant="outline" className="text-sm font-mono">{phaseData.premiumListName}</Badge>
                    </div>
                  </AccordionContent>
                </AccordionItem>
              )}
            </Accordion>

            {/* Metadata */}
            <div className="pt-4 border-t space-y-2 text-xs text-muted-foreground">
              <div className="flex items-center gap-2">
                <Info className="h-3 w-3" />
                <span>Created: {new Date(phaseData.createdAt).toLocaleString()}</span>
              </div>
              <div className="pl-5">
                <span>Updated: {new Date(phaseData.updatedAt).toLocaleString()}</span>
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
              Are you sure you want to delete the phase &quot;{phaseData.name}&quot;?
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
              Choose when this phase should end. The phase &quot;{phaseData.name}&quot; will become inactive after this date.
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
                  disabled={(date) => date < new Date(phaseData.starts) || date < new Date()}
                  initialFocus
                />
              </PopoverContent>
            </Popover>
            <p className="text-xs text-muted-foreground mt-2">
              End date must be after the start date ({formatPhaseDateLong(phaseData.starts)}) and in the future.
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
