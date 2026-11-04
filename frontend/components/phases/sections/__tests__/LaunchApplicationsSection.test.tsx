import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { LaunchApplicationsSection } from '../LaunchApplicationsSection';
import { launchApplicationsApi } from '@/lib/api/launchApplications';
import type { LaunchApplication, Phase } from '@/lib/types/phase';

vi.mock('@/lib/api/launchApplications', () => ({
  launchApplicationsApi: { list: vi.fn(), validate: vi.fn(), allocate: vi.fn(), reject: vi.fn() },
}));
vi.mock('@/lib/api/tlds', () => ({ tldsApi: { get: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const phase = {
  id: 1, name: 'auction', type: 'Launch', starts: '2026-01-01T00:00:00Z', ends: null, prices: [], fees: [],
  createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', tldName: 'paco', policy: {}, lifecyclePolicyName: 'gtld-standard',
  launch: { mode: 'landrush', model: 'application', acceptSignedMarks: true, acceptCodeMarks: false, retainMarks: false },
} as Phase;

function application(over: Partial<LaunchApplication> = {}): LaunchApplication {
  return {
    ID: '7_APP-APEX', Name: 'prize.paco', TLDName: 'paco', PhaseName: 'auction', PhaseMode: 'landrush', ClID: 'rar-one',
    Status: 'pendingAllocation', RegistrantID: 'c1', AdminID: '', TechID: 'c1', BillingID: '', HostNames: null, Years: 2,
    AuthInfoSet: true, Quote: { Currency: 'USD', Amount: 5000, Class: 'standard' }, Reason: '', DomainRoID: '',
    CreatedAt: '2026-11-01T10:00:00Z', UpdatedAt: '2026-11-01T10:00:00Z', ...over,
  };
}

function show(apps: LaunchApplication[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  client.setQueryData(['tld', 'paco'], { Name: 'paco', RyID: 'PacoRy' });
  vi.mocked(launchApplicationsApi.list).mockResolvedValue({ Data: apps, Meta: { PageSize: 25, PageCursor: '', NextLink: '' } });
  return render(
    <QueryClientProvider client={client}>
      <LaunchApplicationsSection phase={phase} tldName="paco" />
    </QueryClientProvider>,
  );
}

describe('LaunchApplicationsSection', () => {
  beforeEach(() => vi.clearAllMocks());

  it("lists the phase's applications as the TLD's operator, with the quote from filing", async () => {
    show([application(), application({ ID: '8_APP-APEX', ClID: 'rar-two', Status: 'rejected', Reason: 'prize.paco was allocated to another application' })]);
    expect(await screen.findByText('7_APP-APEX')).toBeInTheDocument();
    expect(launchApplicationsApi.list).toHaveBeenCalledWith('paco', 'auction', 'PacoRy', { status: '', cursor: '', pageSize: 25 });
    expect(screen.getAllByText('50.00 USD · 2y')).toHaveLength(2);
    expect(screen.getAllByText('rejected').length).toBeGreaterThanOrEqual(2); // the badge and the status filter's option
    expect(screen.getByText(/allocated to another application/)).toBeInTheDocument();
  });

  it('allocates a pendingAllocation application', async () => {
    vi.mocked(launchApplicationsApi.allocate).mockResolvedValue(application({ Status: 'allocated', DomainRoID: '9_DOM-APEX' }));
    show([application()]);
    await userEvent.click(await screen.findByRole('button', { name: 'Allocate' }));
    await waitFor(() => expect(launchApplicationsApi.allocate).toHaveBeenCalledWith('7_APP-APEX', 'PacoRy'));
  });

  it('rejects only with a reason', async () => {
    vi.mocked(launchApplicationsApi.reject).mockResolvedValue(application({ Status: 'rejected' }));
    show([application()]);
    await userEvent.click(await screen.findByRole('button', { name: 'Reject…' }));
    const confirm = screen.getByRole('button', { name: 'Confirm' });
    expect(confirm).toBeDisabled();
    await userEvent.type(screen.getByLabelText('Reason'), 'the phase was cancelled');
    await userEvent.click(confirm);
    await waitFor(() => expect(launchApplicationsApi.reject).toHaveBeenCalledWith('7_APP-APEX', 'PacoRy', 'the phase was cancelled'));
  });

  it('validates with an explicit outcome', async () => {
    vi.mocked(launchApplicationsApi.validate).mockResolvedValue(application({ Status: 'pendingAllocation' }));
    show([application({ Status: 'pendingValidation' })]);
    await userEvent.click(await screen.findByRole('button', { name: 'Valid' }));
    await waitFor(() => expect(launchApplicationsApi.validate).toHaveBeenCalledWith('7_APP-APEX', 'PacoRy', { valid: true, reason: undefined }));
  });

  it('offers no decision on a final application', async () => {
    show([application({ Status: 'allocated', DomainRoID: '9_DOM-APEX' })]);
    expect(await screen.findByText('9_DOM-APEX')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Allocate' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Reject…' })).not.toBeInTheDocument();
  });
});
