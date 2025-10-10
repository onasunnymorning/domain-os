'use client';

import { Phase } from '@/lib/types/phase';
import { useState } from 'react';
import { useDeletePhase, useEndPhase, useUpdatePhasePolicy, useAddPrice, useDeletePrice } from '@/lib/hooks/usePhases';
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
  const [isEditingPolicy, setIsEditingPolicy] = useState(false);
  const [editedPolicy, setEditedPolicy] = useState<Phase['policy'] | null>(null);
  const [isEditingPrices, setIsEditingPrices] = useState(false);
  const [newPrice, setNewPrice] = useState<{
    currency: string;
    registrationAmount: string;
    renewalAmount: string;
    transferAmount: string;
    restoreAmount: string;
  }>({
    currency: '',
    registrationAmount: '',
    renewalAmount: '',
    transferAmount: '',
    restoreAmount: '',
  });
  
  const { mutate: deletePhase, isPending: isDeleting } = useDeletePhase(tldName || '');
  const { mutate: endPhase, isPending: isEnding } = useEndPhase(tldName || '');
  const { mutate: updatePolicy, isPending: isSavingPolicy } = useUpdatePhasePolicy(tldName || '', phase?.name || '');
  const { mutate: addPrice, isPending: isAddingPrice } = useAddPrice(tldName || '', phase?.name || '');
  const { mutate: deletePrice, isPending: isDeletingPrice } = useDeletePrice(tldName || '', phase?.name || '');

  // Fetch full phase details with prices and fees when drawer opens
  const { data: fullPhase, refetch: refetchPhase } = useQuery({
    queryKey: ['phase', tldName, phase?.name],
    queryFn: () => phasesApi.getPhase(tldName!, phase!.name),
    enabled: open && !!tldName && !!phase?.name,
  });

  // Fetch full previous phase details when showing diff
  // For GA phases: previous phase is the one that started before this one
  // For Launch phases: previous phase is the most recently ended phase, or if none exist,
  // the one created before this one (since launch phases can overlap)
  const phasesOfSameType = allPhases
    .filter(p => p.type === phase?.type)
    .sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());
  
  let previousPhaseBasic: Phase | null = null;
  
  if (phase?.type === 'GA') {
    // For GA: simple chronological order by start date
    const currentIndex = phasesOfSameType.findIndex(p => p.id === phase?.id);
    previousPhaseBasic = currentIndex > 0 ? phasesOfSameType[currentIndex - 1] : null;
  } else if (phase?.type === 'Launch') {
    // For Launch: find the most recently ended phase (that ended before this one started)
    const currentStartTime = new Date(phase.starts).getTime();
    const endedBeforeThis = phasesOfSameType
      .filter(p => p.id !== phase.id && p.ends && new Date(p.ends).getTime() <= currentStartTime)
      .sort((a, b) => new Date(b.ends!).getTime() - new Date(a.ends!).getTime());
    
    if (endedBeforeThis.length > 0) {
      previousPhaseBasic = endedBeforeThis[0];
    } else {
      // If no ended phases, fall back to the one created before this one
      const sortedByCreation = phasesOfSameType
        .filter(p => p.id !== phase.id)
        .sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());
      const currentCreationIndex = sortedByCreation.findIndex(p => new Date(p.createdAt).getTime() >= new Date(phase.createdAt).getTime());
      previousPhaseBasic = currentCreationIndex > 0 ? sortedByCreation[currentCreationIndex - 1] : null;
    }
  }

  const { data: fullPreviousPhase } = useQuery({
    queryKey: ['phase', tldName, previousPhaseBasic?.name],
    queryFn: () => phasesApi.getPhase(tldName!, previousPhaseBasic!.name),
    enabled: showDiff && !!tldName && !!previousPhaseBasic?.name,
  });

  // Use full phase data if available, otherwise fall back to the phase prop
  const phaseData = fullPhase || phase;
  const previousPhase = showDiff && fullPreviousPhase ? fullPreviousPhase : null;

  if (!phaseData) return null;

  const isFuture = isPhaseFuture(phaseData.starts);
  const isCurrent = isPhaseCurrent(phaseData.starts, phaseData.ends);
  const canDelete = isFuture;
  const canEnd = isCurrent || isFuture;
  const hasNoEndDate = !phaseData.ends;

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

  const handleEditPolicy = () => {
    setEditedPolicy({ ...phaseData.policy });
    setIsEditingPolicy(true);
  };

  const handleCancelEditPolicy = () => {
    setEditedPolicy(null);
    setIsEditingPolicy(false);
  };

  const handleSavePolicy = () => {
    if (!editedPolicy) return;
    
    updatePolicy(editedPolicy, {
      onSuccess: () => {
        setIsEditingPolicy(false);
        setEditedPolicy(null);
        // Refetch to show updated data
        refetchPhase();
      },
    });
  };

  const handlePolicyChange = (field: keyof Phase['policy'], value: number | boolean | string | undefined) => {
    if (!editedPolicy) return;
    setEditedPolicy({ ...editedPolicy, [field]: value });
  };

  const handleEditPrices = () => {
    setIsEditingPrices(true);
  };

  const handleCancelEditPrices = () => {
    setIsEditingPrices(false);
    setNewPrice({
      currency: '',
      registrationAmount: '',
      renewalAmount: '',
      transferAmount: '',
      restoreAmount: '',
    });
  };

  const handleAddPrice = () => {
    if (!newPrice.currency || !newPrice.registrationAmount || !newPrice.renewalAmount || 
        !newPrice.transferAmount || !newPrice.restoreAmount) {
      return;
    }

    addPrice({
      currency: newPrice.currency.toUpperCase(),
      registrationAmount: parseInt(newPrice.registrationAmount),
      renewalAmount: parseInt(newPrice.renewalAmount),
      transferAmount: parseInt(newPrice.transferAmount),
      restoreAmount: parseInt(newPrice.restoreAmount),
    }, {
      onSuccess: () => {
        setNewPrice({
          currency: '',
          registrationAmount: '',
          renewalAmount: '',
          transferAmount: '',
          restoreAmount: '',
        });
        refetchPhase();
      },
    });
  };

  const handleDeletePrice = (currency: string) => {
    deletePrice(currency, {
      onSuccess: () => {
        refetchPhase();
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
              {previousPhaseBasic && (
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
                      {/* Edit Button */}
                      {!isEditingPrices && (
                        <div className="-mt-2 mb-2">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={handleEditPrices}
                            className="h-7"
                          >
                            Edit
                          </Button>
                        </div>
                      )}

                      {/* Existing Prices */}
                      {phaseData.prices.map((price, index) => (
                        <div key={index} className="space-y-2">
                          <div className="flex items-center justify-between">
                            <div className="text-xs font-semibold text-orange-700 uppercase tracking-wide">{price.currency}</div>
                            {isEditingPrices && (
                              <Button
                                size="sm"
                                variant="destructive"
                                onClick={() => handleDeletePrice(price.currency)}
                                disabled={isDeletingPrice}
                                className="h-6 text-xs"
                              >
                                Remove
                              </Button>
                            )}
                          </div>
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

                      {/* Add New Price Form */}
                      {isEditingPrices && (
                        <div className="border-t pt-4 space-y-3">
                          <div className="text-sm font-medium">Add New Currency</div>
                          <div className="grid grid-cols-2 gap-3">
                            <div className="col-span-2">
                              <label className="text-xs text-muted-foreground">Currency Code</label>
                              <input
                                type="text"
                                maxLength={3}
                                value={newPrice.currency}
                                onChange={(e) => setNewPrice({ ...newPrice, currency: e.target.value.toUpperCase() })}
                                placeholder="USD"
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm uppercase"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Registration (cents)</label>
                              <input
                                type="number"
                                min="0"
                                value={newPrice.registrationAmount}
                                onChange={(e) => setNewPrice({ ...newPrice, registrationAmount: e.target.value })}
                                placeholder="1000"
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Renewal (cents)</label>
                              <input
                                type="number"
                                min="0"
                                value={newPrice.renewalAmount}
                                onChange={(e) => setNewPrice({ ...newPrice, renewalAmount: e.target.value })}
                                placeholder="1000"
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Transfer (cents)</label>
                              <input
                                type="number"
                                min="0"
                                value={newPrice.transferAmount}
                                onChange={(e) => setNewPrice({ ...newPrice, transferAmount: e.target.value })}
                                placeholder="1000"
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Restore (cents)</label>
                              <input
                                type="number"
                                min="0"
                                value={newPrice.restoreAmount}
                                onChange={(e) => setNewPrice({ ...newPrice, restoreAmount: e.target.value })}
                                placeholder="1000"
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                          </div>
                          <div className="flex gap-2">
                            <Button
                              size="sm"
                              onClick={handleAddPrice}
                              disabled={isAddingPrice || !newPrice.currency || !newPrice.registrationAmount || 
                                       !newPrice.renewalAmount || !newPrice.transferAmount || !newPrice.restoreAmount}
                            >
                              {isAddingPrice ? 'Adding...' : 'Add Currency'}
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={handleCancelEditPrices}
                            >
                              Done
                            </Button>
                          </div>
                        </div>
                      )}
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
                    {/* Edit Button - Below Policy heading */}
                    {!isEditingPolicy && (
                      <div className="-mt-2 mb-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={handleEditPolicy}
                          className="h-7"
                        >
                          Edit
                        </Button>
                      </div>
                    )}

                    {/* Label Length - Simple display or edit mode */}
                    <div className="space-y-2">
                      <div className="text-xs text-muted-foreground uppercase tracking-wide">Domain Label Length</div>
                      {!isEditingPolicy ? (
                        <div className="font-semibold">
                          {phaseData.policy.minLabelLength || 1}–{phaseData.policy.maxLabelLength || 63} chars
                        </div>
                      ) : (
                        <div className="grid grid-cols-2 gap-4">
                          <div>
                            <label className="text-xs text-muted-foreground">Min Length</label>
                            <input
                              type="number"
                              min="1"
                              max="63"
                              value={editedPolicy?.minLabelLength || 1}
                              onChange={(e) => handlePolicyChange('minLabelLength', parseInt(e.target.value))}
                              className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                            />
                          </div>
                          <div>
                            <label className="text-xs text-muted-foreground">Max Length</label>
                            <input
                              type="number"
                              min="1"
                              max="63"
                              value={editedPolicy?.maxLabelLength || 63}
                              onChange={(e) => handlePolicyChange('maxLabelLength', parseInt(e.target.value))}
                              className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                            />
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Grace Periods Section */}
                    <div className="space-y-3">
                      <div className="text-sm font-medium text-orange-700">Grace Periods</div>
                      <div className="grid grid-cols-2 gap-x-6 gap-y-3">
                        {!isEditingPolicy ? (
                          <>
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
                          </>
                        ) : (
                          <>
                            <div>
                              <label className="text-xs text-muted-foreground">Registration</label>
                              <input
                                type="number"
                                min="0"
                                value={editedPolicy?.registrationGP || 0}
                                onChange={(e) => handlePolicyChange('registrationGP', parseInt(e.target.value))}
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Renewal</label>
                              <input
                                type="number"
                                min="0"
                                value={editedPolicy?.renewalGP || 0}
                                onChange={(e) => handlePolicyChange('renewalGP', parseInt(e.target.value))}
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Auto Renewal</label>
                              <input
                                type="number"
                                min="0"
                                value={editedPolicy?.autoRenewalGP || 0}
                                onChange={(e) => handlePolicyChange('autoRenewalGP', parseInt(e.target.value))}
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Transfer</label>
                              <input
                                type="number"
                                min="0"
                                value={editedPolicy?.transferGP || 0}
                                onChange={(e) => handlePolicyChange('transferGP', parseInt(e.target.value))}
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Redemption</label>
                              <input
                                type="number"
                                min="0"
                                value={editedPolicy?.redemptionGP || 0}
                                onChange={(e) => handlePolicyChange('redemptionGP', parseInt(e.target.value))}
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                            <div>
                              <label className="text-xs text-muted-foreground">Pending Delete</label>
                              <input
                                type="number"
                                min="0"
                                value={editedPolicy?.pendingdeleteGP || 0}
                                onChange={(e) => handlePolicyChange('pendingdeleteGP', parseInt(e.target.value))}
                                className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                              />
                            </div>
                          </>
                        )}
                      </div>
                    </div>

                    {/* Other Policy Settings */}
                    <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                      {!isEditingPolicy ? (
                        <>
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
                        </>
                      ) : (
                        <>
                          <div>
                            <label className="text-xs text-muted-foreground uppercase tracking-wide">Transfer Lock (days)</label>
                            <input
                              type="number"
                              min="0"
                              value={editedPolicy?.transferLockPeriod || 0}
                              onChange={(e) => handlePolicyChange('transferLockPeriod', parseInt(e.target.value))}
                              className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                            />
                          </div>
                          <div>
                            <label className="text-xs text-muted-foreground uppercase tracking-wide">Max Horizon (years)</label>
                            <input
                              type="number"
                              min="0"
                              value={editedPolicy?.maxHorizon || 0}
                              onChange={(e) => handlePolicyChange('maxHorizon', parseInt(e.target.value))}
                              className="mt-1 w-full px-3 py-2 border rounded-md text-sm"
                            />
                          </div>
                          <div>
                            <label className="text-xs text-muted-foreground uppercase tracking-wide">Allow Autorenew</label>
                            <div className="mt-2">
                              <button
                                type="button"
                                onClick={() => handlePolicyChange('allowAutorenew', !editedPolicy?.allowAutorenew)}
                                className={`inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors ${editedPolicy?.allowAutorenew ? 'bg-primary' : 'bg-input'}`}
                              >
                                <div className={`h-4 w-4 rounded-full bg-background shadow-lg transition-transform ${editedPolicy?.allowAutorenew ? 'translate-x-4' : 'translate-x-0'}`} />
                              </button>
                            </div>
                          </div>
                          <div>
                            <label className="text-xs text-muted-foreground uppercase tracking-wide">Requires Validation</label>
                            <div className="mt-2">
                              <button
                                type="button"
                                onClick={() => handlePolicyChange('requiresValidation', !editedPolicy?.requiresValidation)}
                                className={`inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors ${editedPolicy?.requiresValidation ? 'bg-primary' : 'bg-input'}`}
                              >
                                <div className={`h-4 w-4 rounded-full bg-background shadow-lg transition-transform ${editedPolicy?.requiresValidation ? 'translate-x-4' : 'translate-x-0'}`} />
                              </button>
                            </div>
                          </div>
                          <div>
                            <label className="text-xs text-muted-foreground uppercase tracking-wide">Base Currency</label>
                            <input
                              type="text"
                              maxLength={3}
                              value={editedPolicy?.baseCurrency || ''}
                              onChange={(e) => handlePolicyChange('baseCurrency', e.target.value.toUpperCase())}
                              placeholder="USD"
                              className="mt-1 w-full px-3 py-2 border rounded-md text-sm uppercase"
                            />
                          </div>
                        </>
                      )}
                    </div>

                    {/* Save/Cancel Buttons */}
                    {isEditingPolicy && (
                      <div className="flex gap-2 pt-4 border-t">
                        <Button
                          size="sm"
                          onClick={handleSavePolicy}
                          disabled={isSavingPolicy}
                        >
                          {isSavingPolicy ? 'Saving...' : 'Save Changes'}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={handleCancelEditPolicy}
                          disabled={isSavingPolicy}
                        >
                          Cancel
                        </Button>
                      </div>
                    )}
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
