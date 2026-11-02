import { describe, it, expect } from 'vitest';
import { statusReasons } from '../EPPStatusStrip';
import type { EPPRegistrarAccess } from '@/lib/types/epp';

const now = new Date('2026-11-02T12:00:00Z');
const iso = (d: Date) => d.toISOString();
const days = (n: number) => new Date(now.getTime() + n * 86400000);

function access(over: Partial<EPPRegistrarAccess> = {}): EPPRegistrarAccess {
  return {
    clid: 'rar-001',
    logins: [
      {
        login_id: 'rar-001', role: 'transform', enabled: true, session_cap: 0,
        created_at: iso(days(-10)), updated_at: iso(days(-10)),
        passwords: [{ id: 'p1', created_at: iso(days(-10)), state: 'active' }],
      },
    ],
    allowed_networks: [{ id: 'n1', created_at: iso(days(-10)), state: 'active', prefix: '192.0.2.0/24' }],
    certificate_pins: [
      {
        id: 'c1', created_at: iso(days(-10)), state: 'active', spki_sha256: 'abc', subject: 'CN=x', issuer: '', serial: '',
        not_before: iso(days(-10)), not_after: iso(days(200)),
      },
    ],
    ...over,
  };
}

describe('statusReasons', () => {
  it('is empty when a registrar can log in', () => {
    expect(statusReasons(access(), [], [], now)).toEqual([]);
  });

  it('names each missing piece and where to fix it', () => {
    const r = statusReasons(access({ logins: [], allowed_networks: [], certificate_pins: [] }), [], [], now);
    expect(r.map((x) => x.key)).toEqual(['no-login', 'no-network', 'no-cert']);
    expect(r[0].href).toBe('#epp-logins');
    expect(r[1].href).toBe('#epp-networks');
    expect(r[2].href).toBe('#epp-certificates');
  });

  it('flags an enabled login with only retired passwords', () => {
    const a = access();
    a.logins[0].passwords[0].state = 'retired';
    expect(statusReasons(a, [], [], now).map((x) => x.key)).toEqual(['no-password:rar-001']);
  });

  it('ignores retired networks and certificates', () => {
    const a = access();
    a.allowed_networks[0].state = 'retired';
    a.certificate_pins[0].state = 'retired';
    expect(statusReasons(a, [], [], now).map((x) => x.key)).toEqual(['no-network', 'no-cert']);
  });

  it('warns 30 days before a certificate expires, and when none is valid today', () => {
    const soon = access();
    soon.certificate_pins[0].not_after = iso(days(12));
    expect(statusReasons(soon, [], [], now)[0].text).toMatch(/expires in 12 days/);

    const gone = access();
    gone.certificate_pins[0].not_after = iso(days(-1));
    expect(statusReasons(gone, [], [], now).map((x) => x.key)).toEqual(['no-cert-for:rar-001']);

    const early = access();
    early.certificate_pins[0].not_before = iso(days(2));
    expect(statusReasons(early, [], [], now).map((x) => x.key)).toEqual(['no-cert-for:rar-001']);
  });

  it('counts a restricted certificate only for the logins it names', () => {
    const a = access();
    a.certificate_pins[0].logins = ['other-login'];
    const r = statusReasons(a, [], [], now);
    expect(r.map((x) => x.key)).toEqual(['no-cert-for:rar-001']);
    expect(r[0].text).toMatch(/restricted to other logins/);

    a.certificate_pins[0].logins = ['rar-001'];
    expect(statusReasons(a, [], [], now)).toEqual([]);
  });

  it('lists lockouts and suspensions', () => {
    const r = statusReasons(
      access(),
      [{ clid: 'rar-001', login: 'rar-001', client_ip: '203.0.113.9', ends_at: iso(days(1)) }],
      [{ clid: 'rar-001', login: 'rar-001', scope: 'family', family: 'acme', ends_at: iso(days(1)) }],
      now,
    );
    expect(r).toHaveLength(2);
    expect(r[0].text).toMatch(/locked out from 203\.0\.113\.9/);
    expect(r[1].text).toMatch(/suspended .* \(family acme\)/);
  });
});
