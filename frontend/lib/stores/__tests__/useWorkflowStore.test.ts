import { beforeEach, describe, expect, it } from 'vitest';
import { useWorkflowStore, type WorkflowRun } from '../useWorkflowStore';

function makeRun(workflowId: string): WorkflowRun {
  return {
    workflowId,
    runId: `run-${workflowId}`,
    type: 'escrow-import',
    displayName: workflowId,
    status: 'RUNNING',
    temporalUrl: '',
    startedAt: new Date().toISOString(),
  };
}

describe('useWorkflowStore.addRun', () => {
  beforeEach(() => {
    useWorkflowStore.setState({ runs: [], modalOpen: false, selectedRunId: null });
  });

  it('selects the run it just added, not the oldest one', () => {
    const { addRun } = useWorkflowStore.getState();

    addRun(makeRun('first'));
    addRun(makeRun('second'));

    expect(useWorkflowStore.getState().selectedRunId).toBe('second');
  });

  it('does not add a duplicate but still selects the existing run', () => {
    const { addRun } = useWorkflowStore.getState();

    addRun(makeRun('first'));
    addRun(makeRun('second'));
    addRun(makeRun('first'));

    const state = useWorkflowStore.getState();
    expect(state.runs).toHaveLength(2);
    expect(state.selectedRunId).toBe('first');
  });
});
