import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PhaseLaunchSection } from '../PhaseLaunchSection';
import { phasesApi } from '@/lib/api/phases';
import type { Phase } from '@/lib/types/phase';

vi.mock('@/lib/api/phases', () => ({ phasesApi: { setLaunchSettings: vi.fn() } }));
vi.mock('@/lib/api/tlds', () => ({ tldsApi: { get: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function phase(over: Partial<Phase> = {}): Phase {
  return {
    id: 1, name: 'landrush', type: 'Launch', starts: '2030-01-01T00:00:00Z', ends: null,
    prices: [], fees: [], createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', tldName: 'paco',
    policy: {}, lifecyclePolicyName: 'gtld-standard',
    launch: { mode: 'landrush', model: 'application', acceptSignedMarks: true, acceptCodeMarks: false, retainMarks: false },
    ...over,
  };
}

function show(p: Phase, client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })) {
  client.setQueryData(['tld', 'paco'], { Name: 'paco', RyID: 'PacoRy' });
  return render(
    <QueryClientProvider client={client}>
      <PhaseLaunchSection phase={p} tldName="paco" onRefetch={() => {}} />
    </QueryClientProvider>,
  );
}

describe('PhaseLaunchSection', () => {
  beforeEach(() => vi.clearAllMocks());

  it('shows the mode and the registration model, with their meaning', () => {
    show(phase());
    expect(screen.getByText('Landrush')).toBeInTheDocument();
    expect(screen.getByText('Applications')).toBeInTheDocument();
    expect(screen.getByText(/you allocate each name/)).toBeInTheDocument();
    expect(screen.queryByText('not yet over EPP')).not.toBeInTheDocument();
  });

  it('marks a sunrise as not yet served over EPP', () => {
    show(phase({ launch: { mode: 'sunrise', model: 'first-come', acceptSignedMarks: true, acceptCodeMarks: false, retainMarks: true } }));
    expect(screen.getByText('Sunrise')).toBeInTheDocument();
    expect(screen.getByText('not yet over EPP')).toBeInTheDocument();
  });

  it("saves the settings as the TLD's operator", async () => {
    vi.mocked(phasesApi.setLaunchSettings).mockResolvedValue(phase());
    show(phase());
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
    // A future phase: the mode and the model may still change.
    expect(screen.getByLabelText('Mode')).not.toBeDisabled();
    await userEvent.selectOptions(screen.getByLabelText('Registration model'), 'first-come');
    await userEvent.click(screen.getByRole('switch', { name: 'Code marks' }));
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(phasesApi.setLaunchSettings).toHaveBeenCalledWith('paco', 'landrush',
      { mode: 'landrush', model: 'first-come', acceptSignedMarks: true, acceptCodeMarks: true, retainMarks: false }, 'PacoRy'));
  });

  it('fixes the mode and the model once the phase has started', async () => {
    show(phase({ starts: '2020-01-01T00:00:00Z' }));
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
    expect(screen.getByLabelText('Mode')).toBeDisabled();
    expect(screen.getByLabelText('Registration model')).toBeDisabled();
    expect(screen.getByText('Fixed once the phase has started.')).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Retain marks' })).not.toBeDisabled();
  });

  it('offers no edit on an ended phase', () => {
    show(phase({ starts: '2020-01-01T00:00:00Z', ends: '2021-01-01T00:00:00Z' }));
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
  });

  it('refuses a sunrise that accepts no marks', async () => {
    show(phase({ launch: { mode: 'sunrise', model: 'first-come', acceptSignedMarks: true, acceptCodeMarks: false, retainMarks: false } }));
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
    await userEvent.click(screen.getByRole('switch', { name: 'Signed marks (SMD)' }));
    expect(screen.getByText('A sunrise must accept signed marks or code marks.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });
});
