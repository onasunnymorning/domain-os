import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MissingKeyNotice } from '../PartySelect';
import type { EscrowParty } from '@/lib/api/escrow-keys';

function party(over: Partial<EscrowParty> = {}): EscrowParty {
  return {
    id: 'core',
    name: 'core',
    owner: { kind: 'platform' },
    kind: 'RSP',
    side: 'external',
    purposes: ['verify-inbound'],
    activePurposes: [],
    manageable: true,
    createdAt: '',
    ...over,
  };
}

// Choosing a party with no ACTIVE key is the one mistake that looks like
// success: the arrangement saves, and every deposit fails afterwards.
describe('MissingKeyNotice', () => {
  it('warns, and says what will happen, when the depositor has no active signing key', () => {
    render(
      <MissingKeyNotice party={party()} purpose="verify-inbound" side="depositor" href="/escrow/keys/core" />
    );
    expect(screen.getByText(/no active key for this side/)).toBeInTheDocument();
    expect(screen.getByText(/fails as untrusted/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'core' })).toHaveAttribute('href', '/escrow/keys/core');
  });

  it('says nothing when the party holds an active key for that purpose', () => {
    const { container } = render(
      <MissingKeyNotice
        party={party({ activePurposes: ['verify-inbound'] })}
        purpose="verify-inbound"
        side="depositor"
        href="/escrow/keys/core"
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('says nothing when the server did not report active purposes', () => {
    const { container } = render(
      <MissingKeyNotice
        party={party({ activePurposes: undefined })}
        purpose="verify-inbound"
        side="depositor"
        href="/escrow/keys/core"
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('warns about decryption, not signing, on the receiver side', () => {
    render(
      <MissingKeyNotice party={party({ name: 'migrations' })} purpose="decrypt-inbound" side="receiver" href="/escrow/keys/eve" />
    );
    expect(screen.getByText(/fails as undecryptable/)).toBeInTheDocument();
  });
});
