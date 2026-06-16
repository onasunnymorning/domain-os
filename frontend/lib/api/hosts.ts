import { apiClient } from './client';
import { DomainCountResponse } from '@/lib/types/domain';

export async function getHostCount(): Promise<DomainCountResponse> {
    const { data } = await apiClient.get('/hosts/count');
    return data;
}
