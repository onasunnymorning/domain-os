import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AddKeyVersionDialog } from '../AddKeyVersionDialog';
import * as api from '@/lib/api/escrow-keys';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@/lib/api/escrow-keys', async (orig) => {
  const actual = await (orig() as Promise<typeof api>);
  return {
    ...actual,
    importEscrowPrivateKey: vi.fn(),
    addEscrowPublicKey: vi.fn(),
    generateEscrowKey: vi.fn(),
  };
});

function renderDialog(purpose: api.EscrowKeyPurpose, onOpenChange = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const utils = render(
    <QueryClientProvider client={client}>
      <AddKeyVersionDialog scope={{}} partyId="eve" purpose={purpose} open onOpenChange={onOpenChange} />
    </QueryClientProvider>
  );
  return { ...utils, onOpenChange };
}

describe('AddKeyVersionDialog', () => {
  beforeEach(() => vi.clearAllMocks());

  it('imports a private key and forgets the key and passphrase once sent', async () => {
    let release: (v: api.EscrowKeyVersion) => void = () => {};
    vi.mocked(api.importEscrowPrivateKey).mockImplementation(
      () => new Promise((resolve) => { release = resolve; })
    );
    renderDialog('decrypt-inbound');

    const key = screen.getByLabelText('ASCII-armored private key') as HTMLTextAreaElement;
    const passphrase = screen.getByLabelText('Passphrase') as HTMLInputElement;
    fireEvent.change(key, { target: { value: 'fixture-private-key-material' } });
    fireEvent.change(passphrase, { target: { value: 'hunter2' } });
    expect(passphrase.type).toBe('password');

    fireEvent.click(screen.getByRole('button', { name: 'Import' }));

    await waitFor(() =>
      expect(api.importEscrowPrivateKey).toHaveBeenCalledWith({}, 'eve', {
        purpose: 'decrypt-inbound',
        armoredPrivateKey: 'fixture-private-key-material',
        passphrase: 'hunter2',
      })
    );
    // Cleared while the request is still in flight, not only on success.
    expect(key.value).toBe('');
    expect(passphrase.value).toBe('');
    release({ version: 1 } as api.EscrowKeyVersion);
  });

  it('clears secrets when closed without sending', () => {
    const { onOpenChange } = renderDialog('decrypt-inbound');
    const key = screen.getByLabelText('ASCII-armored private key') as HTMLTextAreaElement;
    fireEvent.change(key, { target: { value: 'SECRET' } });
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(key.value).toBe('');
    expect(api.importEscrowPrivateKey).not.toHaveBeenCalled();
  });

  it('asks for nothing when the service generates the key', async () => {
    vi.mocked(api.generateEscrowKey).mockResolvedValue({ version: 2 } as api.EscrowKeyVersion);
    renderDialog('pseudonymise');
    expect(screen.queryByLabelText('Passphrase')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));
    await waitFor(() => expect(api.generateEscrowKey).toHaveBeenCalledWith({}, 'eve', 'pseudonymise'));
  });
});
