import { describe, it, expect } from 'vitest';
import {
  decodeJwtPayload,
  expiryFromPayload,
  scopesFromPayload,
} from '../jwt';

/** Builds an unsigned, structurally valid JWT around the given payload. */
function makeToken(payload: Record<string, unknown>): string {
  const b64url = (value: object) =>
    btoa(JSON.stringify(value)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `${b64url({ alg: 'RS256', typ: 'JWT' })}.${b64url(payload)}.signature-not-verified`;
}

describe('decodeJwtPayload', () => {
  it('decodes a well-formed payload', () => {
    const token = makeToken({
      sub: 'auth0|abc123',
      permissions: ['escrow:keys:admin'],
      scope: 'openid profile',
      exp: 1800000000,
    });

    expect(decodeJwtPayload(token)).toEqual({
      sub: 'auth0|abc123',
      permissions: ['escrow:keys:admin'],
      scope: 'openid profile',
      exp: 1800000000,
    });
  });

  it('decodes base64url payloads containing - and _', () => {
    // '~~~?>>>' base64-encodes to 'fn5+Pz4+Pg==', whose base64url form uses both
    // '-' and '_'. If the substitution were missing, atob() would throw.
    const token = makeToken({ sub: '~~~?>>>' });
    expect(token.split('.')[1]).toMatch(/[-_]/);
    expect(decodeJwtPayload(token)?.sub).toBe('~~~?>>>');
  });

  it('returns null for a token with the wrong segment count', () => {
    expect(decodeJwtPayload('not.a-jwt')).toBeNull();
    expect(decodeJwtPayload('a.b.c.d')).toBeNull();
  });

  it('returns null when the payload segment is not decodable JSON', () => {
    expect(decodeJwtPayload('header.!!!not-base64!!!.sig')).toBeNull();
    expect(decodeJwtPayload(`header.${btoa('plain text')}.sig`)).toBeNull();
  });

  it('returns null when the payload is not a JSON object', () => {
    expect(decodeJwtPayload(`header.${btoa('["a"]')}.sig`)).toBeNull();
    expect(decodeJwtPayload(`header.${btoa('42')}.sig`)).toBeNull();
  });

  it('returns null for empty input', () => {
    expect(decodeJwtPayload('')).toBeNull();
    expect(decodeJwtPayload(null)).toBeNull();
    expect(decodeJwtPayload(undefined)).toBeNull();
  });
});

describe('scopesFromPayload', () => {
  it('splits the space-delimited scope claim', () => {
    expect(scopesFromPayload({ scope: 'openid profile email' })).toEqual([
      'openid',
      'profile',
      'email',
    ]);
  });

  it('returns an empty array when scope is absent or null payload', () => {
    expect(scopesFromPayload({})).toEqual([]);
    expect(scopesFromPayload(null)).toEqual([]);
  });
});

describe('expiryFromPayload', () => {
  it('converts the exp claim from seconds to a Date', () => {
    expect(expiryFromPayload({ exp: 1800000000 })).toEqual(new Date(1800000000 * 1000));
  });

  it('returns null when exp is missing or not a number', () => {
    expect(expiryFromPayload({})).toBeNull();
    expect(expiryFromPayload(null)).toBeNull();
    expect(expiryFromPayload({ exp: 'soon' } as never)).toBeNull();
  });
});
