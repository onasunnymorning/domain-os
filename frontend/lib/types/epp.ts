/**
 * EPP admin types. Field names mirror the admin API's JSON exactly
 * (pkg/domain/entities/eppRegistrarAccess.go, eppPolicy.go, eppSession.go,
 * eppFailedCreates.go). See docs/epp/POLICY_REFERENCE.md.
 */

export type EPPCredentialState = 'active' | 'retiring' | 'retired';
export type EPPRetireKind = 'normal' | 'compromise' | 'replaced';
export type EPPLoginRole = 'transform' | 'readonly';

/** Shared by passwords, certificate pins and allowed networks. */
export interface EPPCredentialLifecycle {
  id: string;
  created_at: string;
  created_by?: string;
  retire_at?: string;
  retire_kind?: EPPRetireKind;
  retired_by?: string;
  last_used_at?: string;
  last_used_from?: string;
  /** Computed by the API for "now". */
  state: EPPCredentialState;
}

export type EPPPassword = EPPCredentialLifecycle;

export interface EPPCertificatePin extends EPPCredentialLifecycle {
  spki_sha256: string;
  subject: string;
  issuer: string;
  serial: string;
  not_before: string;
  not_after: string;
  /** Empty or absent means every login. */
  logins?: string[];
  expiry_notice_days?: number;
}

export interface EPPAllowedNetwork extends EPPCredentialLifecycle {
  prefix: string;
  note?: string;
}

export interface EPPLogin {
  login_id: string;
  role: EPPLoginRole;
  enabled: boolean;
  /** 0 means the policy default. */
  session_cap: number;
  created_at: string;
  created_by?: string;
  updated_at: string;
  passwords: EPPPassword[];
}

/** GET /registrars/{clid}/epp */
export interface EPPRegistrarAccess {
  clid: string;
  logins: EPPLogin[];
  certificate_pins: EPPCertificatePin[];
  allowed_networks: EPPAllowedNetwork[];
}

/** A pin across registrars, as GET /epp/certificate-pins returns it. */
export interface EPPExpiringCertificatePin extends EPPCertificatePin {
  clid: string;
}

export interface EPPRate {
  per_second: number;
  burst: number;
}

export interface EPPCommandRates {
  check: EPPRate;
  info: EPPRate;
  transform: EPPRate;
  poll: EPPRate;
}

export interface EPPFailedCreateBudget {
  failures: number;
  ratio_percent: number;
}

export interface EPPPolicyLimits {
  sessions_per_registrar: number;
  sessions_per_login: number;
  sessions_per_family: number;
  check_names: number;
  rates: EPPCommandRates;
  family_rates: EPPCommandRates;
  /** 0 means no cap. */
  creates_per_day: number;
  failed_creates: EPPFailedCreateBudget;
  family_failed_creates: EPPFailedCreateBudget;
}

export interface EPPPolicy {
  name: string;
  description?: string;
  limits: EPPPolicyLimits;
  updated_at: string;
  updated_by?: string;
}

export interface RegistrarFamily {
  name: string;
  description?: string;
  /** Empty means default. */
  policy?: string;
  members: string[];
  updated_at: string;
  updated_by?: string;
}

/** GET /registrars/{clid}/epp/policy */
export interface EPPRegistrarPolicy {
  policy: EPPPolicy;
  family?: RegistrarFamily;
  family_policy?: EPPPolicy;
}

export interface EPPSession {
  id: string;
  node: string;
  client_ip: string;
  clid?: string;
  login_id?: string;
  role?: EPPLoginRole;
  credential_ids?: string[];
  family?: string;
  connected_at: string;
  authenticated_at?: string;
  last_command_at?: string;
  commands: number;
}

export interface EPPSessionListParams {
  clid?: string;
  login?: string;
  client_ip?: string;
  credential_id?: string;
  pagesize?: number;
  cursor?: string;
}

export interface EPPSessionListResponse {
  Meta: { PageSize: number; PageCursor: string; NextCursor?: string };
  Data: EPPSession[];
}

export interface EPPLoginLockout {
  clid: string;
  login: string;
  client_ip: string;
  ends_at: string;
}

export interface EPPLoginSuspension {
  clid: string;
  login: string;
  scope: 'registrar' | 'family';
  family?: string;
  ends_at: string;
}

export interface EPPCreateOutcomes {
  clid: string;
  day: string;
  succeeded: number;
  failed: number;
}

// Request bodies (internal/application/commands/epp_*_commands.go)

export interface CreateEPPLoginRequest {
  login_id: string;
  role?: EPPLoginRole;
}

export interface UpdateEPPLoginRequest {
  role?: EPPLoginRole;
  enabled?: boolean;
  session_cap?: number;
}

export interface AddEPPPasswordRequest {
  password?: string;
}

/** The server's answer to adding a password: `password` is set once, only when generated. */
export interface AddEPPPasswordResponse extends EPPPassword {
  password?: string;
}

export interface AddEPPCertificatePinRequest {
  certificate_pem: string;
  logins?: string[];
}

export interface AddEPPAllowedNetworkRequest {
  network: string;
  note?: string;
}

export interface RetireEPPCredentialRequest {
  kind?: 'normal' | 'compromise';
  at?: string;
}

export type EPPCredentialKind = 'password' | 'certificate_pin' | 'allowed_network';

export interface AssignEPPPolicyRequest {
  policy: string;
}

export interface SaveEPPPolicyRequest {
  description?: string;
  limits: EPPPolicyLimits;
}

export interface SaveRegistrarFamilyRequest {
  description?: string;
  policy?: string;
  members: string[];
}
