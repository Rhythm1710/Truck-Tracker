/**
 * Phone-number helpers — Indian 10-digit format.
 *
 * Display: "98765 43210" (group of 5 + group of 5)
 * Internal: digits only
 */

export function digitsOnly(s: string): string {
  return s.replace(/\D/g, '');
}

/** Insert a space after the 5th digit. Caps at 10 digits. */
export function formatPhone(raw: string): string {
  const d = digitsOnly(raw).slice(0, 10);
  if (d.length <= 5) return d;
  return `${d.slice(0, 5)} ${d.slice(5)}`;
}

export function isValidPhone(raw: string): boolean {
  return digitsOnly(raw).length === 10;
}

export function toE164India(raw: string): string {
  return '+91' + digitsOnly(raw);
}
