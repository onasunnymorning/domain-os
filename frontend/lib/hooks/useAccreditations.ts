import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { accreditationsApi, RegistrarAccreditationsParams } from '@/lib/api/accreditations';
import type { RegistrarListParams } from '@/lib/types/registrar';
import { toast } from 'sonner';

export function useRegistrarAccreditations(rarClID: string, params?: RegistrarAccreditationsParams) {
  return useQuery({
    queryKey: ['accreditations', 'registrar', rarClID, params],
    queryFn: () => accreditationsApi.listByRegistrar(rarClID, params),
    enabled: !!rarClID,
  });
}

// Accredit a registrar for a TLD
export function useAccreditRegistrar(rarClID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (tldName: string) => accreditationsApi.accredit(tldName, rarClID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accreditations', 'registrar', rarClID] });
      toast.success('Accreditation added');
    },
    onError: (error: unknown) => {
      const e = error as { response?: { data?: { error?: string } } };
      const message = e?.response?.data?.error || 'Failed to add accreditation';
      toast.error(message);
    },
  });
}

// Deaccredit a registrar for a TLD
export function useDeaccreditRegistrar(rarClID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (tldName: string) => accreditationsApi.deaccredit(tldName, rarClID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accreditations', 'registrar', rarClID] });
    },
  });
}

// List registrars accredited for a TLD
export function useTLDRegistrars(tldName: string, params?: RegistrarListParams) {
  return useQuery({
    queryKey: ['accreditations', 'tld', tldName, params],
    queryFn: () => accreditationsApi.listRegistrarsByTLD(tldName, params),
    enabled: !!tldName,
  });
}

// Accredit for a fixed TLD (pass registrar ClID as variable)
export function useAccreditForTLD(tldName: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (rarClID: string) => accreditationsApi.accredit(tldName, rarClID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accreditations', 'tld', tldName] });
    },
  });
}

// Deaccredit for a fixed TLD (pass registrar ClID as variable)
export function useDeaccreditForTLD(tldName: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (rarClID: string) => accreditationsApi.deaccredit(tldName, rarClID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accreditations', 'tld', tldName] });
    },
  });
}
