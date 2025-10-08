import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { registryOperatorsApi } from '@/lib/api/registry-operators';
import type { CreateRegistryOperatorCommand, ListQueryParams } from '@/lib/api/types';

/**
 * Hook to fetch list of registry operators
 */
export function useRegistryOperators(params?: ListQueryParams) {
  return useQuery({
    queryKey: ['registry-operators', params],
    queryFn: () => registryOperatorsApi.list(params),
  });
}

/**
 * Hook to fetch a single registry operator
 */
export function useRegistryOperator(ryid: string) {
  return useQuery({
    queryKey: ['registry-operator', ryid],
    queryFn: () => registryOperatorsApi.getById(ryid),
    enabled: !!ryid,
  });
}

/**
 * Hook to create a registry operator
 */
export function useCreateRegistryOperator() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (data: CreateRegistryOperatorCommand) => 
      registryOperatorsApi.create(data),
    onSuccess: () => {
      // Invalidate and refetch the list and count
      queryClient.invalidateQueries({ queryKey: ['registry-operators'] });
      queryClient.invalidateQueries({ queryKey: ['registry-operators-count'] });
    },
  });
}

/**
 * Hook to update a registry operator
 */
export function useUpdateRegistryOperator() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: ({ ryid, data }: { ryid: string; data: any }) => 
      registryOperatorsApi.update(ryid, data),
    onSuccess: (_, variables) => {
      // Invalidate the specific operator and the list
      queryClient.invalidateQueries({ queryKey: ['registry-operator', variables.ryid] });
      queryClient.invalidateQueries({ queryKey: ['registry-operators'] });
    },
  });
}

/**
 * Hook to delete a registry operator
 */
export function useDeleteRegistryOperator() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (ryid: string) => registryOperatorsApi.delete(ryid),
    onSuccess: () => {
      // Invalidate the list
      queryClient.invalidateQueries({ queryKey: ['registry-operators'] });
      // Invalidate the count
      queryClient.invalidateQueries({ queryKey: ['registry-operators-count'] });
    },
  });
}

/**
 * Hook to get count of registry operators
 */
export function useRegistryOperatorsCount(params?: ListQueryParams) {
  return useQuery({
    queryKey: ['registry-operators-count', params],
    queryFn: () => registryOperatorsApi.count(params),
  });
}
