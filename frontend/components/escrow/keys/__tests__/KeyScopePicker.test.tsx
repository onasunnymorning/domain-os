import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { KeyScopePicker } from '../KeyScopePicker';

vi.mock('@/components/workflows/OperatorScopeSelect', () => ({
  OperatorScopeSelect: ({ onChange }: { onChange: (ryid: string) => void }) => (
    <button type="button" onClick={() => onChange('ryop1')}>
      Select registry operator
    </button>
  ),
}));

/**
 * Editing the platform default reaches every operator that has none of its
 * own. That is not recoverable by undo, so the page has to say which of the
 * two things the reader is about to change — before they change it.
 */
describe('KeyScopePicker', () => {
  it('says the platform default reaches every operator', () => {
    render(<KeyScopePicker tenantId="" platform onChange={vi.fn()} />);

    expect(screen.getByText('Editing now')).toBeInTheDocument();
    expect(screen.getByText(/Everything below is the platform default/)).toBeInTheDocument();
    expect(screen.getByText(/unless you open that operator/)).toBeInTheDocument();
  });

  it('names the operator, and says the platform parties it also shows are read-only', () => {
    render(<KeyScopePicker tenantId="ryop1" platform={false} onChange={vi.fn()} />);

    expect(screen.getByText(/You are looking at ryop1's setup/)).toBeInTheDocument();
    expect(screen.getByText(/read-only/)).toBeInTheDocument();
  });

  it('asks for a choice rather than showing someone else’s setup by default', () => {
    render(<KeyScopePicker tenantId="" platform={false} onChange={vi.fn()} />);

    expect(screen.getByText(/Choose a registry operator/)).toBeInTheDocument();
    expect(screen.queryByText('Editing now')).not.toBeInTheDocument();
  });

  it('switches scope from either side', async () => {
    const onChange = vi.fn();
    render(<KeyScopePicker tenantId="" platform={false} onChange={onChange} />);

    await userEvent.click(screen.getByRole('button', { name: /Registry platform/ }));
    expect(onChange).toHaveBeenCalledWith({ tenantId: '', platform: true });

    await userEvent.click(screen.getByRole('button', { name: 'Select registry operator' }));
    expect(onChange).toHaveBeenCalledWith({ tenantId: 'ryop1', platform: false });
  });
});
