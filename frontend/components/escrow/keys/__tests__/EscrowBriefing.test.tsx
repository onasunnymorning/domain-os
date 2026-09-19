import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EscrowBriefing } from '../EscrowBriefing';

/**
 * The briefing exists to prevent one expensive mistake: handing out the key
 * that was supposed to stay in the key store. The words that say which is
 * which are the point of the panel, so they are what the tests hold.
 */
describe('EscrowBriefing', () => {
  it('says which key is secret, which is handed out, and which way each travels', () => {
    render(<EscrowBriefing />);

    expect(screen.getByText('Their public verification key')).toBeInTheDocument();
    expect(screen.getByText('comes to us')).toBeInTheDocument();

    expect(screen.getByText('Our private decryption key')).toBeInTheDocument();
    expect(screen.getByText('stays put')).toBeInTheDocument();
    expect(screen.getByText(/never leaves the key store/)).toBeInTheDocument();

    expect(screen.getByText('The public half of that key')).toBeInTheDocument();
    expect(screen.getByText('goes to them')).toBeInTheDocument();
  });

  // Both directions are drawn, so someone reading the outbound rail has to be
  // able to tell it is a picture of what will exist rather than of what does.
  it('draws both directions and marks the outbound one as unbuilt', () => {
    render(<EscrowBriefing />);

    const inbound = screen.getByRole('region', { name: 'Deposits coming to us' });
    expect(inbound).toHaveTextContent('Live today');
    expect(inbound).toHaveTextContent('They sign it');
    expect(inbound).toHaveTextContent('We check the signature');

    const outbound = screen.getByRole('region', { name: 'Deposits going out' });
    expect(outbound).toHaveTextContent('Not built yet');
    expect(outbound).toHaveTextContent('We sign it');
    expect(outbound).toHaveTextContent('They check our signature');
  });
});
