import { describe, expect, it } from '@jest/globals';
import { digitsOnly, formatPhone, isValidPhone, toE164India } from '../phone';

describe('digitsOnly', () => {
  it('strips non-digits', () => {
    expect(digitsOnly('+91 98765-43210')).toBe('919876543210');
  });

  it('returns empty when there are no digits', () => {
    expect(digitsOnly('abc')).toBe('');
    expect(digitsOnly('')).toBe('');
  });
});

describe('formatPhone', () => {
  it('returns up to five digits unchanged', () => {
    expect(formatPhone('')).toBe('');
    expect(formatPhone('98765')).toBe('98765');
  });

  it('inserts a space after the fifth digit', () => {
    expect(formatPhone('987654')).toBe('98765 4');
    expect(formatPhone('9876543210')).toBe('98765 43210');
  });

  it('caps at ten digits and ignores characters that are not digits', () => {
    expect(formatPhone('98765 43210')).toBe('98765 43210');
    expect(formatPhone('98a76543210999')).toBe('98765 43210');
  });
});

describe('isValidPhone', () => {
  it('accepts exactly ten digits, formatted or not', () => {
    expect(isValidPhone('9876543210')).toBe(true);
    expect(isValidPhone('98765 43210')).toBe(true);
  });

  it('rejects numbers that are not ten digits', () => {
    expect(isValidPhone('')).toBe(false);
    expect(isValidPhone('98765')).toBe(false);
    expect(isValidPhone('98765432101')).toBe(false);
  });
});

describe('toE164India', () => {
  it('prefixes +91 and keeps the digits', () => {
    expect(toE164India('98765 43210')).toBe('+919876543210');
  });

  it('does not pad a short number', () => {
    expect(toE164India('98765')).toBe('+9198765');
  });
});
