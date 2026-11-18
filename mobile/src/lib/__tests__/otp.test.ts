import { describe, expect, it } from '@jest/globals';
import { isOtpComplete, sanitizeOtp } from '../otp';

describe('sanitizeOtp', () => {
  it('keeps digits and drops everything else', () => {
    expect(sanitizeOtp('12a 3-4')).toBe('1234');
    expect(sanitizeOtp('')).toBe('');
  });

  it('caps at six digits by default', () => {
    expect(sanitizeOtp('123456789')).toBe('123456');
  });

  it('honors a custom cell count', () => {
    expect(sanitizeOtp('123456', 4)).toBe('1234');
  });

  it('returns empty when the cell count is not positive', () => {
    expect(sanitizeOtp('1234', 0)).toBe('');
    expect(sanitizeOtp('1234', -1)).toBe('');
  });
});

describe('isOtpComplete', () => {
  it('is true only for exactly cellCount digits', () => {
    expect(isOtpComplete('123456')).toBe(true);
    expect(isOtpComplete('12345')).toBe(false);
    expect(isOtpComplete('1234567')).toBe(false);
    expect(isOtpComplete('1234', 4)).toBe(true);
  });

  it('rejects non-digits even when the length matches', () => {
    expect(isOtpComplete('12345a')).toBe(false);
    expect(isOtpComplete('123 456')).toBe(false);
  });

  it('is false for a non-positive cell count', () => {
    expect(isOtpComplete('', 0)).toBe(false);
    expect(isOtpComplete('1', 0)).toBe(false);
  });
});
