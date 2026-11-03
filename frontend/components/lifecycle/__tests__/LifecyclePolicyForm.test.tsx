import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { LifecyclePolicyForm } from '../LifecyclePolicyForm';
import { EMPTY_LIFECYCLE_SETTINGS, STANDARD_LIFECYCLE, resolveLifecycle } from '@/lib/constants/lifecycleDefaults';

function renderForm(over: Partial<React.ComponentProps<typeof LifecyclePolicyForm>> = {}) {
  const onSave = vi.fn().mockResolvedValue(undefined);
  render(
    <LifecyclePolicyForm
      mode="edit"
      initial={{ name: 'short', description: 'demo', settings: EMPTY_LIFECYCLE_SETTINGS }}
      onSave={onSave}
      {...over}
    />,
  );
  return onSave;
}

describe('resolveLifecycle', () => {
  it('fills null settings with the standard values and keeps 0 as zero days', () => {
    expect(resolveLifecycle(EMPTY_LIFECYCLE_SETTINGS)).toEqual(STANDARD_LIFECYCLE);
    const r = resolveLifecycle({ ...EMPTY_LIFECYCLE_SETTINGS, transferLockPeriod: 0, redemptionGP: 3, allowAutorenew: false });
    expect(r.transferLockPeriod).toBe(0);
    expect(r.redemptionGP).toBe(3);
    expect(r.allowAutorenew).toBe(false);
    expect(r.pendingdeleteGP).toBe(STANDARD_LIFECYCLE.pendingdeleteGP);
  });
});

describe('LifecyclePolicyForm', () => {
  it('saves an empty field as null (the standard value) and a number as given', async () => {
    const onSave = renderForm();
    fireEvent.change(screen.getByLabelText('Redemption period'), { target: { value: '3' } });
    fireEvent.change(screen.getByLabelText('Transfer lock'), { target: { value: '0' } });
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const { settings } = onSave.mock.calls[0][0];
    expect(settings.redemptionGP).toBe(3);
    expect(settings.transferLockPeriod).toBe(0);
    expect(settings.registrationGP).toBeNull();
    expect(settings.maxHorizon).toBeNull();
  });

  it('refuses a duration over its bound and a horizon outside 1 to 10, on the field', async () => {
    const onSave = renderForm();
    fireEvent.change(screen.getByLabelText('Pending delete'), { target: { value: '91' } });
    fireEvent.change(screen.getByLabelText('Max horizon (years)'), { target: { value: '11' } });
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(screen.getByText(/pending delete must be 0 to 90/i)).toBeInTheDocument());
    expect(screen.getByText(/horizon is 1 to 10 years/i)).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
  });

  it('shows what a save reaches, and offers no delete on the built-in policy', () => {
    renderForm({
      initial: { name: 'gtld-standard', settings: EMPTY_LIFECYCLE_SETTINGS },
      builtIn: true,
      onDelete: vi.fn(),
      usedBy: [
        { tld: 'alpha', phase: 'ga-2026' },
        { tld: 'alpha', phase: 'sunrise' },
        { tld: 'beta', phase: 'ga' },
      ],
    });
    expect(screen.getByRole('status')).toHaveTextContent('A save reaches 3 phases across 2 TLDs at once.');
    expect(screen.getByText('alpha/ga-2026')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^delete$/i })).not.toBeInTheDocument();
  });

  it('writes a switch turned off as false, and leaves an untouched one null', async () => {
    const onSave = renderForm();
    fireEvent.click(screen.getByRole('switch', { name: /restore report required/i }));
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const { settings } = onSave.mock.calls[0][0];
    expect(settings.restoreReportRequired).toBe(false);
    expect(settings.allowAutorenew).toBeNull();
  });
});
