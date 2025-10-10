import { Phase } from '@/lib/types/phase';
import { apiClient } from './client';

export const phasesApi = {
  // List all phases for a TLD
  listByTLD: async (tldName: string): Promise<{ Data: Phase[] }> => {
    const response = await apiClient.get(`/tlds/${tldName}/phases`);
    return response.data;
  },

  // List active phases for a TLD
  listActiveByTLD: async (tldName: string): Promise<{ Data: Phase[] }> => {
    const response = await apiClient.get(`/tlds/${tldName}/phases/active`);
    return response.data;
  },

  // Get a specific phase
  getPhase: async (tldName: string, phaseName: string): Promise<Phase> => {
    const response = await apiClient.get(`/tlds/${tldName}/phases/${phaseName}`);
    return response.data;
  },

  // Create a new phase
  create: async (tldName: string, data: {
    name: string;
    type: 'GA' | 'Launch';
    starts: string;
    ends?: string | null;
  }): Promise<Phase> => {
    const response = await apiClient.post(`/tlds/${tldName}/phases`, data);
    return response.data;
  },

  // Delete a phase
  delete: async (tldName: string, phaseName: string): Promise<void> => {
    await apiClient.delete(`/tlds/${tldName}/phases/${phaseName}`);
  },

  // End a phase (set end date)
  endPhase: async (tldName: string, phaseName: string, endDate: string): Promise<Phase> => {
    const response = await apiClient.put(`/tlds/${tldName}/phases/${phaseName}/end`, { ends: endDate });
    return response.data;
  },

  // Update phase policy
  updatePolicy: async (tldName: string, phaseName: string, policy: Phase['policy']): Promise<Phase> => {
    const response = await apiClient.put(`/tlds/${tldName}/phases/${phaseName}/policy`, { Policy: policy });
    return response.data;
  },

  // Add a price to a phase
  addPrice: async (tldName: string, phaseName: string, price: {
    currency: string;
    registrationAmount: number;
    renewalAmount: number;
    transferAmount: number;
    restoreAmount: number;
  }): Promise<Phase['prices'][0]> => {
    const response = await apiClient.post(`/tlds/${tldName}/phases/${phaseName}/prices`, price);
    return response.data;
  },

  // Delete a price from a phase
  deletePrice: async (tldName: string, phaseName: string, currency: string): Promise<void> => {
    await apiClient.delete(`/tlds/${tldName}/phases/${phaseName}/prices/${currency}`);
  },
};

