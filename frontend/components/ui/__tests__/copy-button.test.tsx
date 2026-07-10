import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { CopyButton } from '../copy-button';
import { toast } from 'sonner';

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

describe('CopyButton', () => {
  const writeTextMock = vi.fn().mockResolvedValue(undefined);
  const vibrateMock = vi.fn();

  beforeEach(() => {
    vi.useFakeTimers();
    
    // Mock navigator APIs safely
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: writeTextMock,
      },
      writable: true,
      configurable: true,
    });
    
    Object.defineProperty(navigator, 'vibrate', {
      value: vibrateMock,
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it('renders Copy button initially', () => {
    render(<CopyButton value="test-value" />);
    const button = screen.getByRole('button');
    expect(button).toBeInTheDocument();
  });

  it('copies text and triggers haptics on click', async () => {
    render(<CopyButton value="test-value" />);
    const button = screen.getByRole('button');
    
    await act(async () => {
      fireEvent.click(button);
    });

    expect(writeTextMock).toHaveBeenCalledWith('test-value');
    expect(vibrateMock).toHaveBeenCalledWith(15);
  });

  it('shows success checkmark and then reverts after successDuration', async () => {
    render(<CopyButton value="test-value" successDuration={500} />);
    const button = screen.getByRole('button');

    await act(async () => {
      fireEvent.click(button);
    });

    expect(button.querySelector('svg')?.classList.contains('text-emerald-500')).toBe(true);

    // Fast-forward time
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    expect(button.querySelector('svg')?.classList.contains('text-emerald-500')).toBe(false);
  });

  it('displays a toast if showToast is true', async () => {
    render(<CopyButton value="test-value" showToast={true} toastMessage="Custom message" />);
    const button = screen.getByRole('button');

    await act(async () => {
      fireEvent.click(button);
    });

    expect(toast.success).toHaveBeenCalledWith('Custom message');
  });
});
