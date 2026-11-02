/**
 * The built-in policies' starting limits, as the migrate job seeds them.
 * Source: docs/epp/POLICY_REFERENCE.md § Policies and § Command rates
 * (pkg/domain/entities/eppPolicy.go DefaultEPPPolicyLimits and
 * ProbationEPPPolicyLimits). Keep them in step with the Go constants.
 */

import type { EPPCommandRates, EPPPolicyLimits } from "@/lib/types/epp";

export const BUILT_IN_POLICY_NAMES = ["default", "probation"] as const;

export function isBuiltInPolicy(name: string): boolean {
  return (BUILT_IN_POLICY_NAMES as readonly string[]).includes(name);
}

/** The family's rates start at three times the registrar's. */
export function tripleRates(r: EPPCommandRates): EPPCommandRates {
  const x = (n: number) => n * 3;
  return {
    check: { per_second: x(r.check.per_second), burst: x(r.check.burst) },
    info: { per_second: x(r.info.per_second), burst: x(r.info.burst) },
    transform: { per_second: x(r.transform.per_second), burst: x(r.transform.burst) },
    poll: { per_second: x(r.poll.per_second), burst: x(r.poll.burst) },
  };
}

const defaultRates: EPPCommandRates = {
  check: { per_second: 250, burst: 1000 },
  info: { per_second: 20, burst: 100 },
  transform: { per_second: 10, burst: 50 },
  poll: { per_second: 5, burst: 20 },
};

const probationRates: EPPCommandRates = {
  check: { per_second: 100, burst: 500 },
  info: { per_second: 10, burst: 50 },
  transform: { per_second: 5, burst: 25 },
  poll: { per_second: 5, burst: 20 },
};

export const DEFAULT_POLICY_LIMITS: EPPPolicyLimits = {
  sessions_per_registrar: 10,
  sessions_per_login: 10,
  sessions_per_family: 40,
  check_names: 50,
  rates: defaultRates,
  family_rates: tripleRates(defaultRates),
  creates_per_day: 0,
  failed_creates: { failures: 100, ratio_percent: 90 },
  family_failed_creates: { failures: 300, ratio_percent: 90 },
};

export const PROBATION_POLICY_LIMITS: EPPPolicyLimits = {
  sessions_per_registrar: 10,
  sessions_per_login: 10,
  sessions_per_family: 30,
  check_names: 50,
  rates: probationRates,
  family_rates: tripleRates(probationRates),
  creates_per_day: 100,
  failed_creates: { failures: 50, ratio_percent: 90 },
  family_failed_creates: { failures: 150, ratio_percent: 90 },
};

export function builtInLimits(name: string): EPPPolicyLimits | undefined {
  if (name === "default") return DEFAULT_POLICY_LIMITS;
  if (name === "probation") return PROBATION_POLICY_LIMITS;
  return undefined;
}

/** What each rate class meters, for the editor's tooltips. */
export const RATE_CLASSES: { key: keyof EPPCommandRates; label: string; commands: string; note?: string }[] = [
  { key: "check", label: "Check", commands: "<check> on domains, hosts and contacts", note: "Counted per name, not per command." },
  { key: "info", label: "Info", commands: "<info> and <transfer op=\"query\">" },
  { key: "transform", label: "Transform", commands: "create, update, delete, renew, and transfer request, approve, reject, cancel" },
  { key: "poll", label: "Poll", commands: "<poll> request and acknowledge" },
];

/** Policy names: 1 to 32 lower-case letters, digits and hyphens, starting with a letter. */
export const POLICY_NAME_PATTERN = /^[a-z][a-z0-9-]{0,31}$/;
