/**
 * Design tokens — mirror the Figma file.
 * Keep these the single source of truth across all screens.
 *
 * Contrast notes — text-on-pill ratios verified for WCAG AA (4.5:1 normal text):
 *   Running #0E7A37 on #DEF6E3 ≈ 5.1:1 ✓
 *   Idle    #8C5A05 on #FEF3CF ≈ 5.4:1 ✓
 *   Stopped #B11F1F on #FDDEDE ≈ 5.0:1 ✓
 *   Offline #475569 on #E5E7EB ≈ 6.0:1 ✓
 */
export const colors = {
  // surfaces
  bg: '#F5F6F8',
  surface: '#FFFFFF',
  surfaceAlt: '#F5F6F8',

  // text
  textPrimary: '#0F172A',
  textSecondary: '#64748B',

  // brand
  primary: '#1E40AF',
  primaryText: '#FFFFFF',

  // status — text-color picked for WCAG AA on bg-color
  statusRunning:   '#0E7A37',
  statusRunningBg: '#DEF6E3',
  statusIdle:      '#8C5A05',
  statusIdleBg:    '#FEF3CF',
  statusStopped:   '#B11F1F',
  statusStoppedBg: '#FDDEDE',
  statusOffline:   '#475569',
  statusOfflineBg: '#E5E7EB',

  // connectivity banner
  connOnline:  '#0E7A37',
  connOffline: '#B11F1F',
  connWarn:    '#8C5A05',

  // misc
  border: '#E5E7EB',
  divider: '#CDD5DF',
} as const;

export const spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 20,
  '2xl': 24,
  '3xl': 32,
  '4xl': 40,
  '5xl': 56,
  '6xl': 64,
} as const;

export const radius = {
  sm: 8,
  md: 12,
  lg: 16,
  xl: 18,
  '2xl': 24,
  pill: 999,
} as const;

export const fontSize = {
  caption: 12,
  small: 13,
  body: 15,
  bodyLg: 16,
  subtitle: 18,
  title: 22,
  titleLg: 24,
  display: 28,
  displayLg: 36,
} as const;

export const fontWeight = {
  regular: '400' as const,
  medium: '500' as const,
  semibold: '600' as const,
  bold: '700' as const,
};

/** Minimum tap target — matches the WCAG 2.1 AAA / senior-friendly target. */
export const minTouchTarget = 48;

export type Status = 'Running' | 'Idle' | 'Stopped' | 'Offline';

export function colorForStatus(status: Status) {
  switch (status) {
    case 'Running': return { fg: colors.statusRunning, bg: colors.statusRunningBg };
    case 'Idle':    return { fg: colors.statusIdle,    bg: colors.statusIdleBg };
    case 'Stopped': return { fg: colors.statusStopped, bg: colors.statusStoppedBg };
    case 'Offline': return { fg: colors.statusOffline, bg: colors.statusOfflineBg };
  }
}
