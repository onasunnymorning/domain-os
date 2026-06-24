'use client';

import { Phase } from '@/lib/types/phase';
import { useState } from 'react';
import { useDeletePhase, useEndPhase } from '@/lib/hooks/usePhases';
import { useQuery } from '@tanstack/react-query';
import { phasesApi } from '@/lib/api/phases';
import { Sheet, SheetContent } from '@/components/ui/sheet';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel,
  AlertDialogContent, AlertDialogDescription, AlertDialogFooter,
  AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { formatPhaseDateLong, isPhaseCurrent } from '@/lib/utils/dateUtils';
import { Clock, Tag } from 'lucide-react';
import { format } from 'date-fns';

// Section components
import { PhaseDetailHeader } from './sections/PhaseDetailHeader';
import { PhaseDateSection } from './sections/PhaseDateSection';
import { PhasePricingSection } from './sections/PhasePricingSection';
import { PhaseFeeSection } from './sections/PhaseFeeSection';
import { PhasePolicySection } from './sections/PhasePolicySection';
import { PhaseMetadataSection } from './sections/PhaseMetadataSection';
import { PhaseConfigDiff } from './PhaseConfigDiff';

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

  // Fetch full phase details when drawer opens
  const { data: fullPhase, refetch: refetchPhase } = useQuery({
    queryKey: ['phase', tldName, phase?.name],
    queryFn: () => phasesApi.getPhase(tldName!, phase!.name),
    enabled: open && !!tldName && !!phase?.name,
  });

  // Find previous phase for diff
  const phasesOfSameType = allPhases
    .filter(p => p.type === phase?.type)
    .sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());

  let previousPhaseBasic: Phase | null = null;
  if (phase?.type === 'GA') {
    const currentIndex = phasesOfSameType.findIndex(p => p.id === phase?.id);
    previousPhaseBasic = currentIndex > 0 ? phasesOfSameType[currentIndex - 1] : null;
  } else if (phase?.type === 'Launch') {
    const currentStartTime = phase ? new Date(phase.starts).getTime() : 0;
    const endedBeforeThis = phasesOfSameType
      .filter(p => p.id !== phase?.id && p.ends && new Date(p.ends).getTime() <= currentStartTime)
      .sort((a, b) => new Date(b.ends!).getTime() - new Date(a.ends!).getTime());
    previousPhaseBasic = endedBeforeThis.length > 0 ? endedBeforeThis[0] : null;
  }

  const { data: fullPreviousPhase } = useQuery({
    queryKey: ['phase', tldName, previousPhaseBasic?.name],
    queryFn: () => phasesApi.getPhase(tldName!, previousPhaseBasic!.name),
    enabled: showDiff && !!tldName && !!previousPhaseBasic?.name,
  });

  // Use full phase data if available, fall back to prop
  const phaseData = fullPhase || phase;
  const previousPhase = showDiff && fullPreviousPhase ? fullPreviousPhase : null;

  if (!phaseData) return null;

  const handleDelete = () => {
    deletePhase(phaseData.name, {
      onSuccess: () => { setShowDeleteDialog(false); onClose(); },
    });
  };

  const handleEndPhase = () => {
    if (!endDate) return;
    endPhase({ phaseName: phaseData.name, endDate: endDate.toISOString() }, {
      onSuccess: () => { setShowEndPhaseDialog(false); setEndDate(undefined); onClose(); },
    });
  };

  const handleRefetch = () => { refetchPhase(); };

  return (
    <>
      <Sheet open={open} onOpenChange={onClose}>
        <SheetContent className="w-full sm:max-w-2xl overflow-y-auto">
          {/* Header */}
          <PhaseDetailHeader
            phase={phaseData}
            hasPreviousPhase={!!previousPhaseBasic}
            showDiff={showDiff}
            onToggleDiff={() => setShowDiff(!showDiff)}
            onDelete={() => setShowDeleteDialog(true)}
            onEndPhase={() => setShowEndPhaseDialog(true)}
          />

          <div className="mt-6 space-y-6">
            {/* Diff */}
            {showDiff && previousPhase && (
              <PhaseConfigDiff phase={phaseData} compareWith={previousPhase} />
            )}

            {/* Date Section */}
            <PhaseDateSection phase={phaseData} />

            {/* Divider */}
            <div className="border-t" />

            {/* Pricing Section */}
            <PhasePricingSection
              phase={phaseData}
              tldName={tldName || ''}
              onRefetch={handleRefetch}
            />

            {/* Divider */}
            <div className="border-t" />

            {/* Fees Section */}
            <PhaseFeeSection
              phase={phaseData}
              tldName={tldName || ''}
              onRefetch={handleRefetch}
            />

            {/* Divider */}
            <div className="border-t" />

            {/* Policy Section */}
            <PhasePolicySection
              phase={phaseData}
              tldName={tldName || ''}
              onRefetch={handleRefetch}
            />

            {/* Premium List */}
            {phaseData.premiumListName && (
              <>
                <div className="border-t" />
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Tag className="h-4 w-4 text-orange-600" />
                    <span className="text-sm font-semibold">Premium List</span>
                  </div>
                  <Badge variant="outline" className="text-sm font-mono">{phaseData.premiumListName}</Badge>
                </div>
              </>
            )}

            {/* Metadata */}
            <PhaseMetadataSection phase={phaseData} />
          </div>
        </SheetContent>
      </Sheet>

      {/* Delete Confirmation */}
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

          <div className="py-4 space-y-4">
            <Button
              variant="secondary"
              className="w-full flex justify-center text-orange-600 bg-orange-50 hover:bg-orange-100 hover:text-orange-700 border border-orange-200"
              onClick={() => {
                const now = new Date();
                now.setSeconds(now.getSeconds() + 1);
                endPhase({ phaseName: phaseData.name, endDate: now.toISOString() }, {
                  onSuccess: () => { setShowEndPhaseDialog(false); setEndDate(undefined); onClose(); },
                });
              }}
              disabled={isEnding}
            >
              <Clock className="mr-2 h-4 w-4" />
              {isEnding ? 'Ending Phase...' : 'End Now (in 1 second)'}
            </Button>

            <div className="relative flex items-center py-2">
              <div className="flex-grow border-t border-border" />
              <span className="flex-shrink-0 mx-4 text-muted-foreground text-xs uppercase">Or specify date &amp; time</span>
              <div className="flex-grow border-t border-border" />
            </div>

            <div className="space-y-2">
              <Input
                type="datetime-local"
                value={endDate ? format(endDate, "yyyy-MM-dd'T'HH:mm") : ''}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  setEndDate(e.target.value ? new Date(e.target.value) : undefined);
                }}
              />
              <p className="text-xs text-muted-foreground">
                End date must be after the start date ({formatPhaseDateLong(phaseData.starts)}).
              </p>
            </div>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setEndDate(undefined)}>Cancel</AlertDialogCancel>
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
