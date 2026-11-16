/**
 * OTP helpers for the 6-cell login code.
 *
 * The input is controlled: callers store the sanitized digit string, and
 * completion is "exactly cellCount digits".
 */

/** Digits only, capped at cellCount. Non-digits are dropped. */
export function sanitizeOtp(raw: string, cellCount = 6): string {
  if (cellCount <= 0) return '';
  return raw.replace(/\D/g, '').slice(0, cellCount);
}

/** True when value is exactly cellCount digits and nothing else. */
export function isOtpComplete(value: string, cellCount = 6): boolean {
  if (cellCount <= 0) return false;
  return value.length === cellCount && sanitizeOtp(value, cellCount) === value;
}
