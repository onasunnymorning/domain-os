import { describe, it, expect } from 'vitest';
import { memberNamedIn } from '../FamilyEditorPage';

describe('memberNamedIn', () => {
  it('finds the member a 409 names, whole and exactly', () => {
    expect(memberNamedIn('the registrar is in another family: rar-002 is in acme', ['rar-001', 'rar-002'])).toBe('rar-002');
    expect(memberNamedIn('the registrar is in another family: rar-0011 is in acme', ['rar-001', 'rar-0011'])).toBe('rar-0011');
    expect(memberNamedIn('the registrar is in another family: rar-0011 is in acme', ['rar-001'])).toBeUndefined();
  });
  it('is undefined when the message names none', () => {
    expect(memberNamedIn('policy not found', ['rar-001'])).toBeUndefined();
  });
});
