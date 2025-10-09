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

  return (
    <>
      <Sheet open={open} onOpenChange={onClose}>
        <SheetContent className="w-full sm:max-w-2xl overflow-y-auto bg-gradient-to-b from-background to-muted/20">
          <SheetHeader className="pb-6 border-b">
            <div className="flex items-start justify-between">
              <div className="space-y-2">
                <div className="flex items-center gap-3">
                  <SheetTitle className="text-2xl">{phase.name}</SheetTitle>
                  <Badge 
                    variant={phase.type === 'GA' ? 'default' : 'secondary'}
                    className="text-xs px-2 py-0.5"
                  >
                    {phase.type}
                  </Badge>
                </div>
                <SheetDescription className="text-sm">
                  {isPhaseCurrent(phase.starts, phase.ends) 
                    ? '🟢 Currently active' 
                    : isPhaseFuture(phase.starts)
                    ? '🔵 Scheduled'
                    : '⚫ Past'}
                </SheetDescription>
              </div>
            </div>
            
            {/* Action Buttons */}
            <div className="flex flex-wrap gap-2 pt-4">
              {previousPhase && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setShowDiff(!showDiff)}
                  className="gap-2"
                >
                  <GitCompare className="h-4 w-4" />
                  {showDiff ? 'Hide' : 'Show'} Diff
                </Button>
              )}
              {canDelete && (
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => setShowDeleteDialog(true)}
                  className="gap-2"
                >
                  <Trash2 className="h-4 w-4" />
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

            {/* Timeline */}
            <div className="rounded-lg border bg-card p-4 shadow-sm">
              <div className="flex items-center gap-2 text-sm font-semibold mb-3 text-orange-700">
                <Calendar className="h-5 w-5" />
                Timeline
              </div>
              <div className="space-y-4">
                {/* Start Date */}
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground uppercase tracking-wide">
                    {isFuture ? 'Starts' : 'Started'}
                  </div>
                  <div className="text-2xl font-bold">{formatDateWithTime(phase.starts).date}</div>
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Clock className="h-3.5 w-3.5" />
                    <span>{formatDateWithTime(phase.starts).time} UTC</span>
                  </div>
                  <div className="text-sm text-muted-foreground">{formatRelativeDate(phase.starts)}</div>
                </div>
                
                {/* End Date */}
                <div className={`space-y-1 ${!phase.ends ? 'opacity-40' : ''}`}>
                  <div className="flex items-center justify-between">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide">End</div>
                    {!phase.ends && canEnd && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setShowEndPhaseDialog(true)}
                        className="gap-2 border-orange-300 text-orange-700 hover:bg-orange-50"
                      >
                        <CalendarX className="h-4 w-4" />
                        Set End Date
                      </Button>
                    )}
                  </div>
                  {phase.ends ? (
                    <>
                      <div className="text-2xl font-bold">{formatDateWithTime(phase.ends).date}</div>
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Clock className="h-3.5 w-3.5" />
                        <span>{formatDateWithTime(phase.ends).time} UTC</span>
                      </div>
                      <div className="text-sm text-muted-foreground">{formatRelativeDate(phase.ends)}</div>
                    </>
                  ) : (
                    <div className="text-2xl font-bold italic">
                      {isCurrent ? 'No end date set (ongoing)' : 'No end date set'}
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Policy */}
            <div className="rounded-lg border bg-card p-4 shadow-sm">
              <div className="flex items-center gap-2 text-sm font-semibold mb-3 text-orange-700">
                <Settings className="h-5 w-5" />
                Policy Configuration
              </div>
              <div className="grid grid-cols-2 gap-4">
                {phase.policy.minLabelLength !== undefined && (
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide">Min Label</div>
                    <div className="font-medium">{phase.policy.minLabelLength} chars</div>
                  </div>
                )}
                {phase.policy.maxLabelLength !== undefined && (
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide">Max Label</div>
                    <div className="font-medium">{phase.policy.maxLabelLength} chars</div>
                  </div>
                )}
                {phase.policy.registrationGP !== undefined && (
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide">Registration GP</div>
                    <div className="font-medium">{phase.policy.registrationGP} days</div>
                  </div>
                )}
                {phase.policy.renewalGP !== undefined && (
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide">Renewal GP</div>
                    <div className="font-medium">{phase.policy.renewalGP} days</div>
                  </div>
                )}
                {phase.policy.transferGP !== undefined && (
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide">Transfer GP</div>
                    <div className="font-medium">{phase.policy.transferGP} days</div>
                  </div>
                )}
                {phase.policy.redemptionGP !== undefined && (
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide">Redemption GP</div>
                    <div className="font-medium">{phase.policy.redemptionGP} days</div>
                  </div>
                )}
              </div>
            </div>

            {/* Pricing */}
            {phase.prices && phase.prices.length > 0 && (
              <div className="rounded-lg border bg-card p-4 shadow-sm">
                <div className="flex items-center gap-2 text-sm font-semibold mb-3 text-orange-700">
                  <DollarSign className="h-5 w-5" />
                  Pricing
                </div>
                <div className="space-y-2">
                  {phase.prices.map((price) => (
                    <div key={price.id} className="flex items-center justify-between p-2 rounded bg-muted/30">
                      <span className="text-sm text-muted-foreground font-medium">{price.currency}</span>
                      <span className="text-lg font-semibold">
                        ${(price.amount / 100).toFixed(2)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Fees */}
            {phase.fees && phase.fees.length > 0 && (
              <div className="rounded-lg border bg-card p-4 shadow-sm">
                <div className="flex items-center gap-2 text-sm font-semibold mb-3 text-orange-700">
                  <Tag className="h-5 w-5" />
                  Fees
                </div>
                <div className="space-y-2">
                  {phase.fees.map((fee) => (
                    <div key={fee.id} className="flex items-center justify-between p-2 rounded bg-muted/30">
                      <span className="text-sm text-muted-foreground">{fee.name}</span>
                      <span className="font-semibold">
                        {fee.currency} ${(fee.amount / 100).toFixed(2)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Premium List */}
            {phase.premiumListName && (
              <div className="rounded-lg border bg-card p-4 shadow-sm">
                <div className="text-sm font-semibold mb-2 text-orange-700">Premium List</div>
                <Badge variant="outline" className="text-sm">{phase.premiumListName}</Badge>
              </div>
            )}

            {/* Metadata */}
            <div className="pt-4 border-t space-y-1">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Info className="h-3 w-3" />
                <span>Created: {new Date(phase.createdAt).toLocaleString()}</span>
              </div>
              <div className="flex items-center gap-2 text-xs text-muted-foreground pl-5">
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
