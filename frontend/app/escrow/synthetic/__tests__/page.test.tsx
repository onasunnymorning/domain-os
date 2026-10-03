import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import SyntheticDepositPage from '../page';
import { generateSyntheticDeposit } from '@/lib/api/escrow-synthetic';

vi.mock('@/lib/api/escrow-synthetic', () => ({ generateSyntheticDeposit: vi.fn() }));
vi.mock('@/components/layout/DashboardLayout', () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

const wrap = (children: React.ReactNode) => {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
};

describe('SyntheticDepositPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    URL.createObjectURL = vi.fn(() => 'blob:x');
    URL.revokeObjectURL = vi.fn();
  });

  it('shows the objects the deposit will carry', () => {
    render(wrap(<SyntheticDepositPage />));
    fireEvent.change(screen.getByLabelText('Domains'), { target: { value: '10' } });
    fireEvent.change(screen.getByLabelText('Contacts per domain'), { target: { value: '4' } });
    fireEvent.change(screen.getByLabelText('Average hosts per domain'), { target: { value: '1.5' } });
    fireEvent.change(screen.getByLabelText('NNDNs'), { target: { value: '3' } });

    expect(screen.getByTestId('synthetic-summary').textContent).toMatch(/10 domains · 40 contacts · 15 hosts · 5 registrars · 3 NNDNs/);
  });

  it('blocks a request over the domain ceiling', () => {
    render(wrap(<SyntheticDepositPage />));
    fireEvent.change(screen.getByLabelText('Domains'), { target: { value: '250001' } });
    expect(screen.getByRole('alert').textContent).toMatch(/250,000/);
    expect(screen.getByRole('button', { name: /Generate/ })).toBeDisabled();
  });

  it('generates with the entered params and saves the file', async () => {
    vi.mocked(generateSyntheticDeposit).mockResolvedValue({ blob: new Blob(['gz']), filename: 'best_2026-10-02_full_S1_R0.xml.gz', seed: '9' });
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    render(wrap(<SyntheticDepositPage />));

    fireEvent.change(screen.getByLabelText('TLD'), { target: { value: 'best' } });
    fireEvent.change(screen.getByLabelText('Domains'), { target: { value: '25' } });
    fireEvent.change(screen.getByLabelText('Seed (optional)'), { target: { value: '9' } });
    fireEvent.click(screen.getByRole('button', { name: /Generate/ }));

    await waitFor(() => expect(screen.getByTestId('synthetic-last').textContent).toMatch(/best_2026-10-02_full_S1_R0\.xml\.gz.*seed 9/));
    expect(vi.mocked(generateSyntheticDeposit).mock.calls[0][0]).toEqual({
      tld: 'best', domains: 25, contactsPerDomain: 2, avgHostsPerDomain: 2, nndns: 0, seed: 9,
    });
    expect(click).toHaveBeenCalled();
  });
});
