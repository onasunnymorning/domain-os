import { apiClient } from './client';
import type { 
  RegistryOperator, 
  CreateRegistryOperatorCommand,
  ListResponse,
  ListQueryParams
} from './types';

export const registryOperatorsApi = {
  /**
   * List all registry operators with optional filters
   */
  list: async (params?: ListQueryParams): Promise<ListResponse<RegistryOperator>> => {
    const { data } = await apiClient.get('/registry-operators', { params });
    return data;
  },

  /**
   * Get a single registry operator by RyID
   */
  getById: async (ryid: string): Promise<RegistryOperator> => {
    const { data } = await apiClient.get(`/registry-operators/${ryid}`);
    return data;
  },

  /**
   * Create a new registry operator
   */
  create: async (command: CreateRegistryOperatorCommand): Promise<RegistryOperator> => {
    const { data } = await apiClient.post('/registry-operators', command);
    return data;
  },

  /**
   * Update an existing registry operator
   */
  update: async (ryid: string, operator: Partial<RegistryOperator>): Promise<RegistryOperator> => {
    const { data } = await apiClient.put(`/registry-operators/${ryid}`, operator);
    return data;
  },

  /**
   * Delete a registry operator
   */
  delete: async (ryid: string): Promise<void> => {
    await apiClient.delete(`/registry-operators/${ryid}`);
  },

  /**
   * Get count of registry operators
   */
  count: async (params?: ListQueryParams): Promise<{ Count: number; ObjectType: string; Timestamp: string }> => {
    const { data } = await apiClient.get('/registry-operators/count', { params });
    return data;
  },
};
