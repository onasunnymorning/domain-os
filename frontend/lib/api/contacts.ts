import { apiClient } from './client';
import { DomainCountResponse } from '@/lib/types/domain';

export async function getContactCount(): Promise<DomainCountResponse> {
    const { data } = await apiClient.get('/contacts/count');
    return data;
}
