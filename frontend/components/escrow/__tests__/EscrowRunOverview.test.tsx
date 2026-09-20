import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { EscrowRunOverview } from '../EscrowRunOverview';
import type {
  EscrowDeposit,
  EscrowValidationRun,
  EscrowValidationSummary,
} from '@/lib/api/escrow-runs';
import { getEscrowValidationSummary } from '@/lib/api/escrow-runs';

vi.mock('@/lib/api/escrow-runs', async (orig) => ({
  ...(await (orig() as Promise<Record<string, unknown>>)),
  getEscrowValidationSummary: vi.fn(),
}));

const summary = (counts: EscrowValidationSummary['deposit']['counts']) =>
  ({ deposit: { counts } }) as EscrowValidationSummary;

const RUN: EscrowValidationRun = {
  id: 'r1',
  depositId: '93446ed7-70ba-46e0-9702-bc98348c9a5d',
  tld: 'best',
  workflowId: 'wf',
  profile: 'xml',
  outcome: 'PASS',
  verified: false,
  stageReached: 'rde',
  findings: [],
  findingTally: [{ code: 'RDE_OBJECT_ENTITY_REJECTED', severity: 'WARNING', stage: 'rde', count: 324 }],
  rdeDepositId: 'TI8QO0',
  rdeKind: 'FULL',
  rdeResend: 0,
  rdeWatermark: '2026-07-16T00:00:00Z',
  summaryObjectKey: 'escrow-validation/best/r1/summary.json',
  startedAt: '2026-09-19T16:11:47Z',
};

const DEPOSIT = { receivedAt: '2026-09-19T16:11:47Z' } as EscrowDeposit;

const show = (run: EscrowValidationRun = RUN) =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <EscrowRunOverview run={run} deposit={DEPOSIT} />
    </QueryClientProvider>
  );

const OBJ = 'urn:ietf:params:xml:ns:';

beforeEach(() => {
  vi.mocked(getEscrowValidationSummary).mockReset();
});

describe('EscrowRunOverview', () => {
  it('names the deposit and says what kind it is', async () => {
    vi.mocked(getEscrowValidationSummary).mockResolvedValue(summary([]));
    show();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Deposit 2026-07-16 / TI8QO0');
    expect(screen.getByText(/Full deposit · Resend: No · Received 2026-09-19 16:11 UTC/)).toBeInTheDocument();
    expect(screen.getByText('PASS')).toBeInTheDocument();
    await screen.findByText(/No object counts were recorded/);
  });

  it('shows the object counts once the summary loads', async () => {
    vi.mocked(getEscrowValidationSummary).mockResolvedValue(
      summary([
        { uri: `${OBJ}rdeDomain-1.0`, observed: 6241, declared: 6241, status: 'match' },
        { uri: `${OBJ}rdeContact-1.0`, observed: 8913, declared: 8913, status: 'match' },
        { uri: `${OBJ}rdeHost-1.0`, observed: 327, declared: 327, status: 'match' },
        { uri: `${OBJ}rdeRegistrar-1.0`, observed: 42, declared: 42, status: 'match' },
      ])
    );
    show();
    expect(await screen.findByText('6,241')).toBeInTheDocument();
    expect(screen.getByText('8,913')).toBeInTheDocument();
    expect(screen.getByText('Registrars').nextSibling).toHaveTextContent('42');
  });

  it('says so when the header disagrees with what was found', async () => {
    vi.mocked(getEscrowValidationSummary).mockResolvedValue(
      summary([{ uri: `${OBJ}rdeDomain-1.0`, observed: 6241, declared: 6300, status: 'mismatch' }])
    );
    show();
    expect(await screen.findByText('header says 6,300')).toBeInTheDocument();
  });

  it('keeps the verdict and the steps when the summary cannot be loaded', async () => {
    vi.mocked(getEscrowValidationSummary).mockRejectedValue(new Error('CORS'));
    show();
    expect(await screen.findByText(/Object counts could not be loaded/)).toBeInTheDocument();
    expect(screen.getByText('PASS')).toBeInTheDocument();
    expect(screen.getByLabelText('Validation: passed, 324 warnings')).toBeInTheDocument();
  });

  it('does not fetch or claim a count for a run with no summary', () => {
    show({ ...RUN, summaryObjectKey: undefined });
    expect(getEscrowValidationSummary).not.toHaveBeenCalled();
    expect(screen.getByText(/Object counts are not available for this run/)).toBeInTheDocument();
  });

  it('does not report zero objects for a run that never reached them', async () => {
    vi.mocked(getEscrowValidationSummary).mockResolvedValue(summary([]));
    show({ ...RUN, profile: 'ryde+sig', outcome: 'FAIL', stageReached: 'decrypt' });
    expect(await screen.findByText(/the run stopped at decrypt/)).toBeInTheDocument();
  });

  it('does not tick signature or decrypt for an unsigned deposit', async () => {
    vi.mocked(getEscrowValidationSummary).mockResolvedValue(summary([]));
    show();
    const pipeline = screen.getByRole('list', { name: 'Pipeline' });
    expect(within(pipeline).getByLabelText('Signature: not applicable')).toBeInTheDocument();
    expect(within(pipeline).getByLabelText('Decrypt: not applicable')).toBeInTheDocument();
    expect(within(pipeline).getByLabelText('Structure: passed')).toBeInTheDocument();
    await screen.findByText(/No object counts/);
  });
});
