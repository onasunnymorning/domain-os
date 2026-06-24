'use client';

import { CheckCircle2, Circle, Loader2, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { WorkflowStep, WorkflowStatus } from '@/lib/stores/useWorkflowStore';

interface WorkflowStepperProps {
  steps: WorkflowStep[];
  currentStep?: string;
  status: WorkflowStatus;
}

type StepState = 'completed' | 'current' | 'pending' | 'failed';

function getStepState(
  stepKey: string,
  steps: WorkflowStep[],
  currentStep: string | undefined,
  status: WorkflowStatus
): StepState {
  // If the overall workflow is completed, all steps are completed
  if (status === 'COMPLETED') return 'completed';

  if (!currentStep) return 'pending';

  const stepIndex = steps.findIndex((s) => s.key === stepKey);
  const currentIndex = steps.findIndex((s) => s.key === currentStep);

  if (stepIndex < 0 || currentIndex < 0) return 'pending';

  if (stepIndex < currentIndex) return 'completed';
  if (stepIndex === currentIndex) {
    // If the workflow failed, mark the current step as failed
    if (status === 'FAILED' || status === 'TIMED_OUT' || status === 'TERMINATED') {
      return 'failed';
    }
    return 'current';
  }
  return 'pending';
}

const stepIcons: Record<StepState, React.ReactNode> = {
  completed: <CheckCircle2 className="size-4 text-green-500" />,
  current: <Loader2 className="size-4 animate-spin text-blue-500" />,
  pending: <Circle className="text-muted-foreground/40 size-4" />,
  failed: <XCircle className="size-4 text-red-500" />,
};

const stepTextStyles: Record<StepState, string> = {
  completed: 'text-green-600 dark:text-green-400',
  current: 'text-blue-600 dark:text-blue-400 font-medium',
  pending: 'text-muted-foreground',
  failed: 'text-red-600 dark:text-red-400 font-medium',
};

export function WorkflowStepper({ steps, currentStep, status }: WorkflowStepperProps) {
  if (!steps.length) return null;

  return (
    <div className="flex flex-col gap-0">
      {steps.map((step, index) => {
        const state = getStepState(step.key, steps, currentStep, status);
        const isLast = index === steps.length - 1;

        return (
          <div key={step.key} className="flex items-stretch gap-3">
            {/* Icon column with connecting line */}
            <div className="flex flex-col items-center">
              <div className="flex size-6 shrink-0 items-center justify-center">
                {stepIcons[state]}
              </div>
              {!isLast && (
                <div
                  className={cn(
                    'w-px flex-1 min-h-3',
                    state === 'completed'
                      ? 'bg-green-300 dark:bg-green-700'
                      : 'bg-border'
                  )}
                />
              )}
            </div>

            {/* Label */}
            <div className={cn('pb-3 pt-0.5 text-sm', stepTextStyles[state])}>
              {step.label}
            </div>
          </div>
        );
      })}
    </div>
  );
}
