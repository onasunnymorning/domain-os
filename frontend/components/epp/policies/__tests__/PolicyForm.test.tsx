import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { PolicyForm } from '../PolicyForm';
import { DEFAULT_POLICY_LIMITS, PROBATION_POLICY_LIMITS, tripleRates } from '@/lib/constants/eppPolicyDefaults';

function renderForm(over: Partial<React.ComponentProps<typeof PolicyForm>> = {}) {
  const onSave = vi.fn().mockResolvedValue(undefined);
  render(
    <PolicyForm
      mode="edit"
      initial={{ name: 'gold', description: 'Big ones', limits: DEFAULT_POLICY_LIMITS }}
      onSave={onSave}
      {...over}
    />,
  );
  return onSave;
}

describe('PolicyForm', () => {
  it('refuses a check burst below the names per check, on the burst field', async () => {
    const onSave = renderForm();
    fireEvent.change(screen.getByLabelText('Check burst'), { target: { value: '10' } });
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(screen.getByText(/check burst must be at least the names per check/i)).toBeInTheDocument());
    expect(onSave).not.toHaveBeenCalled();
  });

  it('writes 0 creates per day when the switch is off, and the number when on', async () => {
    const onSave = renderForm();
    expect(screen.getByText(/no cap/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0][0].limits.creates_per_day).toBe(0);

    fireEvent.click(screen.getByRole('switch', { name: /limit successful domain creates/i }));
    fireEvent.change(screen.getByLabelText('Creates per day'), { target: { value: '250' } });
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
    expect(onSave.mock.calls[1][0].limits.creates_per_day).toBe(250);
  });

  it('resets a built-in policy to its starting values', async () => {
    const onSave = renderForm({
      initial: { name: 'probation', description: '', limits: { ...PROBATION_POLICY_LIMITS, check_names: 7, creates_per_day: 0 } },
      builtIn: true,
      resetValues: PROBATION_POLICY_LIMITS,
    });
    expect(screen.getByLabelText('Names per check')).toHaveValue(7);
    fireEvent.click(screen.getByRole('button', { name: /reset to built-in/i }));
    expect(screen.getByLabelText('Names per check')).toHaveValue(50);
    expect(screen.getByLabelText('Creates per day')).toHaveValue(100);
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0][0].limits).toEqual(PROBATION_POLICY_LIMITS);
    expect(screen.queryByRole('button', { name: /delete policy/i })).not.toBeInTheDocument();
  });

  it('fills family rates at three times the registrar rates', async () => {
    const onSave = renderForm({ initial: { name: 'gold', limits: { ...DEFAULT_POLICY_LIMITS, family_rates: { ...DEFAULT_POLICY_LIMITS.rates } } } });
    fireEvent.change(screen.getByLabelText('Transform per second'), { target: { value: '40' } });
    fireEvent.click(screen.getByRole('button', { name: /set to 3×/i }));
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const limits = onSave.mock.calls[0][0].limits;
    expect(limits.family_rates).toEqual(tripleRates(limits.rates));
    expect(limits.family_rates.transform.per_second).toBe(120);
  });

  it('locks the name when editing and requires a valid one when creating', async () => {
    renderForm();
    expect(screen.getByLabelText('Name')).toBeDisabled();
  });
});
