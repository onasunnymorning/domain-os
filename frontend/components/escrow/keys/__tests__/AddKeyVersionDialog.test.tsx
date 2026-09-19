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

// Assembled from the kind so this file holds no literal armor header, which
// the secret scanner would read as a committed key.
const block = (kind: 'PUBLIC' | 'PRIVATE') =>
  [`-----BEGIN PGP ${kind} KEY BLOCK-----`, 'fixture-key-material', `-----END PGP ${kind} KEY BLOCK-----`].join('\n');
const privateBlock = block('PRIVATE');
const publicBlock = block('PUBLIC');

describe('AddKeyVersionDialog', () => {
  beforeEach(() => vi.clearAllMocks());

  it('imports a private key and forgets the key and passphrase once sent', async () => {
    let release: (v: api.EscrowKeyVersion) => void = () => {};
    vi.mocked(api.importEscrowPrivateKey).mockImplementation(
      () => new Promise((resolve) => { release = resolve; })
    );
    renderDialog('decrypt-inbound');

    const key = screen.getByLabelText('Private decryption key') as HTMLTextAreaElement;
    const passphrase = screen.getByLabelText('Passphrase') as HTMLInputElement;
    fireEvent.change(key, { target: { value: privateBlock } });
    fireEvent.change(passphrase, { target: { value: 'hunter2' } });
    expect(passphrase.type).toBe('password');

    fireEvent.click(screen.getByRole('button', { name: 'Import' }));

    await waitFor(() =>
      expect(api.importEscrowPrivateKey).toHaveBeenCalledWith({}, 'eve', {
        purpose: 'decrypt-inbound',
        armoredPrivateKey: privateBlock,
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
    const key = screen.getByLabelText('Private decryption key') as HTMLTextAreaElement;
    fireEvent.change(key, { target: { value: privateBlock } });
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

  it('refuses to send a private key pasted into the public field', () => {
    renderDialog('verify-inbound');
    fireEvent.change(screen.getByLabelText('Public verification key'), { target: { value: privateBlock } });

    expect(screen.getByRole('alert').textContent).toContain('appears to be a private key');
    expect(screen.getByRole('button', { name: 'Add' })).toBeDisabled();
    expect(api.addEscrowPublicKey).not.toHaveBeenCalled();
  });

  it('keeps a refused public key in the field and shows why', async () => {
    vi.mocked(api.addEscrowPublicKey).mockRejectedValue(
      Object.assign(new Error('rejected'), {
        isAxiosError: true,
        response: { status: 400, data: { error: 'OpenPGP could not read this key: user ID self-signature invalid' } },
      })
    );
    const { onOpenChange } = renderDialog('verify-inbound');
    const field = screen.getByLabelText('Public verification key') as HTMLTextAreaElement;
    fireEvent.change(field, { target: { value: publicBlock } });
    fireEvent.click(screen.getByRole('button', { name: 'Add' }));

    await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('self-signature invalid'));
    // A public key is not a secret: it stays so the paste can be corrected.
    expect(field.value).toBe(publicBlock);
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
