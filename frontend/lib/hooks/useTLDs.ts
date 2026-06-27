import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { tldsApi, CreateTLDRequest, ListQueryParams } from '../api/tlds';
import { toast } from 'sonner';
import { useWorkflowStore, type WorkflowRun } from '@/lib/stores/useWorkflowStore';

// List TLDs with pagination/filters
export function useTLDs(params?: ListQueryParams) {
  return useQuery({
    queryKey: ['tlds', params],
    queryFn: () => tldsApi.list(params),
  });
}

// Get TLDs for a specific Registry Operator
export function useTLDsByRyID(ryid: string) {
  return useQuery({
    queryKey: ['tlds', 'by-ryid', ryid],
    queryFn: () => tldsApi.list({ ryid_equals: ryid, pagesize: 100 }),
    enabled: !!ryid,
  });
}

// Get TLD count
export function useTLDsCount(params?: ListQueryParams) {
  return useQuery({
    queryKey: ['tlds-count', params],
    queryFn: () => tldsApi.count(params),
  });
}

// Get single TLD
export function useTLD(name: string) {
  return useQuery({
    queryKey: ['tld', name],
    queryFn: () => tldsApi.get(name),
    enabled: !!name,
  });
}

// Create TLD
export function useCreateTLD() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (tld: CreateTLDRequest) => tldsApi.create(tld),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tlds'] });
      queryClient.invalidateQueries({ queryKey: ['tlds-count'] });
      toast.success('TLD created successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to create TLD');
    },
  });
}

// Delete TLD triggers the deletion workflow
export function useDeleteTLD() {
  const queryClient = useQueryClient();
  const addRun = useWorkflowStore((s) => s.addRun);
  const setModalOpen = useWorkflowStore((s) => s.setModalOpen);
  const selectRun = useWorkflowStore((s) => s.selectRun);
  
  return useMutation({
    mutationFn: ({ name, keepTLDAndPhases }: { name: string, keepTLDAndPhases: boolean }) => tldsApi.triggerCleanup(name, keepTLDAndPhases),
    onSuccess: (data: any, variables) => {
      queryClient.invalidateQueries({ queryKey: ['tlds'] });
      queryClient.invalidateQueries({ queryKey: ['tlds-count'] });

      // Register the run in the workflow store
      const run: WorkflowRun = {
        workflowId: data.workflowId,
        runId: data.runId,
        type: 'tld-cleanup',
        displayName: 'TLD Cleanup',
        status: 'RUNNING',
        temporalUrl: data.url,
        startedAt: new Date().toISOString(),
        params: {
          tld: variables.name,
          keepTLDAndPhases: variables.keepTLDAndPhases,
        },
      };

      addRun(run);
      selectRun(data.workflowId);
      setModalOpen(true);

      toast.success(`Cleanup workflow started for .${variables.name}`, {
        description: `Review and confirm the deletion in the workflow control center.`,
        action: data.url ? {
          label: 'Temporal UI',
          onClick: () => window.open(data.url, '_blank'),
        } : undefined,
        duration: 8000,
      });
    },
    onError: (error: any) => {
      const message = error.response?.data?.error || 'Failed to trigger TLD deletion workflow';
      toast.error(message);
    },
  });
}

// Update TLD
export function useUpdateTLD() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: Partial<{ AllowEscrowImport: boolean; EnableDNS: boolean; RyID: string; UName: string }>}) =>
      tldsApi.update(name, data as any),
    onSuccess: (res, vars) => {
      // Refresh list and the specific TLD
      queryClient.invalidateQueries({ queryKey: ['tlds'] });
      queryClient.invalidateQueries({ queryKey: ['tlds-count'] });
      queryClient.invalidateQueries({ queryKey: ['tld', vars.name] });
      toast.success('TLD updated successfully');
    },
    onError: (error: any) => {
      const message = error?.response?.data?.error || 'Failed to update TLD';
      toast.error(message);
    },
  });
}
