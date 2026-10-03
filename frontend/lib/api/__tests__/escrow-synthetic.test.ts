import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AxiosError, AxiosHeaders } from 'axios';
import { apiClient } from '../client';
import { generateSyntheticDeposit } from '../escrow-synthetic';

vi.mock('../client', () => ({
  apiClient: { post: vi.fn() },
}));

const params = { tld: 'test', domains: 10, contactsPerDomain: 2, avgHostsPerDomain: 2, nndns: 0 };

function axiosError(status: number, data: unknown) {
  const headers = new AxiosHeaders();
  return new AxiosError('failed', String(status), { headers }, undefined, {
    status, statusText: '', headers: {}, config: { headers }, data,
  });
}

describe('generateSyntheticDeposit', () => {
  beforeEach(() => vi.clearAllMocks());

  it('posts the params as a blob request and reads filename and seed from the headers', async () => {
    const blob = new Blob(['gz']);
    vi.mocked(apiClient.post).mockResolvedValue({
      data: blob,
      headers: { 'content-disposition': 'attachment; filename="test_2026-10-02_full_S1_R0.xml.gz"', 'x-synthetic-seed': '42' },
    });

    const out = await generateSyntheticDeposit(params);

    expect(apiClient.post).toHaveBeenCalledWith('/escrow/synthetic', params, expect.objectContaining({ responseType: 'blob' }));
    expect(out).toEqual({ blob, filename: 'test_2026-10-02_full_S1_R0.xml.gz', seed: '42' });
  });

  it('falls back to a computed filename when the header is not exposed', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: new Blob(['gz']), headers: {} });
    const out = await generateSyntheticDeposit(params);
    expect(out.filename).toMatch(/^test_\d{4}-\d{2}-\d{2}_full_S1_R0\.xml\.gz$/);
    expect(out.seed).toBeNull();
  });

  it("surfaces the server's error from a blob body", async () => {
    vi.mocked(apiClient.post).mockRejectedValue(
      axiosError(400, new Blob([JSON.stringify({ error: 'domains must be between 0 and 250000' })], { type: 'application/json' })),
    );
    await expect(generateSyntheticDeposit(params)).rejects.toThrow('domains must be between 0 and 250000');
  });

  it('falls back to the status when the error body is not JSON', async () => {
    vi.mocked(apiClient.post).mockRejectedValue(axiosError(502, new Blob(['Bad Gateway'])));
    await expect(generateSyntheticDeposit(params)).rejects.toThrow('HTTP 502');
  });
});
