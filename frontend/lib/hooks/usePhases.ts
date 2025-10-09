import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { phasesApi } from '@/lib/api/phases';
import { Phase, CategorizedPhases, PhaseStatus } from '@/lib/types/phase';

// Utility functions to determine phase status
const getPhaseStatus = (phase: Phase): PhaseStatus => {
  const now = new Date();
  const start = new Date(phase.starts);
  const end = phase.ends ? new Date(phase.ends) : null;

  if (end && end < now) {
    return 'past';
  }
  if (start <= now && (!end || end > now)) {
    return 'current';
  }
  return 'future';
};

// Categorize phases by type and status
const categorizePhases = (phases: Phase[]): CategorizedPhases => {
  const gaPhases = phases.filter(p => p.type === 'GA');
  const launchPhases = phases.filter(p => p.type === 'Launch');

  const categorize = (phaseList: Phase[]) => {
    const current: Phase[] = [];
    const past: Phase[] = [];
    const future: Phase[] = [];

    phaseList.forEach(phase => {
      const status = getPhaseStatus(phase);
      if (status === 'current') current.push(phase);
      else if (status === 'past') past.push(phase);
      else future.push(phase);
    });

    // Sort by start date
    past.sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());
    current.sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());
    future.sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());

    return { current, past, future };
  };

  const gaCategorized = categorize(gaPhases);
  const launchCategorized = categorize(launchPhases);

  return {
    ga: {
      current: gaCategorized.current[0] || null, // Only one GA can be active
      past: gaCategorized.past,
      future: gaCategorized.future,
    },
    launch: launchCategorized,
  };
};

// Hook to fetch all phases for a TLD
export function usePhases(tldName: string) {
  return useQuery({
    queryKey: ['phases', tldName],
    queryFn: async () => {
      const response = await phasesApi.listByTLD(tldName);
      return response.Data || [];
    },
    enabled: !!tldName,
  });
}

// Hook to fetch and categorize phases
export function useCategorizedPhases(tldName: string) {
  const { data: phases, ...rest } = usePhases(tldName);
  
  const categorized = phases ? categorizePhases(phases) : {
    ga: { current: null, past: [], future: [] },
    launch: { current: [], past: [], future: [] },
  };

  return {
    ...rest,
    phases,
    categorized,
  };
}

// Hook to get a specific phase
export function usePhase(tldName: string, phaseName: string) {
  return useQuery({
    queryKey: ['phases', tldName, phaseName],
    queryFn: () => phasesApi.getPhase(tldName, phaseName),
    enabled: !!tldName && !!phaseName,
  });
}

// Hook to create a phase
export function useCreatePhase(tldName: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: {
      name: string;
      type: 'GA' | 'Launch';
      starts: string;
      ends?: string | null;
    }) => phasesApi.create(tldName, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['phases', tldName] });
    },
  });
}

// Hook to delete a phase
export function useDeletePhase(tldName: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (phaseName: string) => phasesApi.delete(tldName, phaseName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['phases', tldName] });
    },
  });
}

// Hook to end a phase
export function useEndPhase(tldName: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ phaseName, endDate }: { phaseName: string; endDate: string }) =>
      phasesApi.endPhase(tldName, phaseName, endDate),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['phases', tldName] });
    },
  });
}
