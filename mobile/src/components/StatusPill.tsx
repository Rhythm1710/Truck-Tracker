/**
 * Color-coded status pill (Running / Idle / Stopped / Offline).
 *
 * - 15pt text (body size) for senior-friendly readability.
 * - Color + dot + text → never relies on color alone.
 * - WCAG AA contrast verified for every combo (see theme.ts).
 */
import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import { colorForStatus, fontSize, fontWeight, radius, Status } from '../theme';

interface Props {
  status: Status;
  /** Use slightly smaller chrome inside list rows. */
  compact?: boolean;
}

export const StatusPill = React.memo(function StatusPill({ status, compact }: Props) {
  const { fg, bg } = colorForStatus(status);
  return (
    <View
      style={[styles.pill, compact && styles.pillCompact, { backgroundColor: bg }]}
      accessibilityRole="text"
      accessibilityLabel={`Status: ${status}`}
    >
      <View style={[styles.dot, { backgroundColor: fg }]} />
      <Text
        style={[styles.label, compact && styles.labelCompact, { color: fg }]}
        maxFontSizeMultiplier={1.4}
      >
        {status}
      </Text>
    </View>
  );
});

const styles = StyleSheet.create({
  pill: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: radius.pill,
    gap: 6,
    flexShrink: 0,
  },
  pillCompact: { paddingVertical: 6 },
  dot: { width: 9, height: 9, borderRadius: 99 },
  label: { fontSize: fontSize.body, fontWeight: fontWeight.semibold },
  labelCompact: { fontSize: fontSize.small },
});
