/**
 * Handing out a public key.
 *
 * A source needs the public half of the key we decrypt with, and we need the
 * public half of the key a source signs with — both are meant to be shared,
 * and both are fetched the same way. Keeping the fetch here means the two
 * places that offer it cannot drift on the filename or the error text.
 */

import { getEscrowPublicKey, type EscrowKeyScope } from '@/lib/api/escrow-keys';

/** Saves the armored public key as `<fingerprint>.asc`. */
export function saveArmoredKey(armored: string, fingerprint: string): void {
  const url = URL.createObjectURL(new Blob([armored], { type: 'application/pgp-keys' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `${fingerprint}.asc`;
  a.click();
  URL.revokeObjectURL(url);
}

/** Fetches a version's armored public key and saves it to a file. */
export async function downloadPublicKey(scope: EscrowKeyScope, versionId: string, fingerprint: string): Promise<void> {
  saveArmoredKey(await getEscrowPublicKey(scope, versionId), fingerprint);
}
