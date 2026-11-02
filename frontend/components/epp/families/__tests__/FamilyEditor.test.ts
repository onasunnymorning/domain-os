import { describe, it, expect } from 'vitest';
import { memberNamedIn } from '../FamilyEditorPage';

describe('memberNamedIn', () => {
  it('finds the member a 409 names', () => {
    expect(memberNamedIn('registrar "rar-002" is already in family acme', ['rar-001', 'rar-002'])).toBe('rar-002');
  });
  it('is undefined when the message names none', () => {
    expect(memberNamedIn('policy not found', ['rar-001'])).toBeUndefined();
  });
});
