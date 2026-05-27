/**
 * Color-coded status pill (Running / Idle / Stopped / Offline).
 */
import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import { colors, colorForStatus, fontSize, fontWeight, radius, spacing, Status } from '../theme';

export function StatusPill({ status }: { status: Status }) {
  const { fg, bg } = colorForStatus(status);
  return (
    <View style={[styles.pill, { backgroundColor: bg }]}>
      <View style={[styles.dot, { backgroundColor: fg }]} />
      <Text style={[styles.label, { color: fg }]}>{status}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  pill: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm - 2,
    borderRadius: radius.pill,
    gap: spacing.sm - 2,
  },
  dot: { width: 8, height: 8, borderRadius: 999 },
  label: { fontSize: fontSize.small, fontWeight: fontWeight.semibold },
});

// silence unused import in CI if needed
void colors;
