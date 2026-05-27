/**
 * Home screen truck card — matches the Figma design 1:1.
 */
import React from 'react';
import { View, Text, Pressable, StyleSheet } from 'react-native';

import { StatusPill } from './StatusPill';
import { colors, fontSize, fontWeight, radius, spacing } from '../theme';
import { statusFromPosition, formatRelativeTime } from '../lib/types';
import type { TruckSummary } from '../lib/types';

interface Props {
  truck: TruckSummary;
  onPress: () => void;
}

export function TruckCard({ truck, onPress }: Props) {
  const { device, latest } = truck;
  const status = statusFromPosition(latest);
  const moving = status === 'Running';
  const speed = latest ? Math.round(latest.speed) : 0;

  return (
    <Pressable onPress={onPress} style={({ pressed }) => [styles.card, pressed && styles.cardPressed]}>
      <View style={styles.topRow}>
        <Text style={styles.plate}>{device.plate}</Text>
        <StatusPill status={status} />
      </View>
      <View style={styles.bottomRow}>
        <View style={styles.metaItem}>
          <Text style={styles.icon}>{moving ? '💨' : '⏸'}</Text>
          <Text style={styles.metaValue}>{speed} km/h</Text>
        </View>
        <View style={{ flex: 1 }} />
        <View style={styles.metaItem}>
          <Text style={styles.icon}>🕐</Text>
          <Text style={styles.metaTime}>{formatRelativeTime(latest?.at)}</Text>
        </View>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    paddingHorizontal: spacing.xl,
    paddingVertical: spacing.lg + 2,
    borderWidth: 1,
    borderColor: colors.border,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.04,
    shadowRadius: 6,
    elevation: 1,
    gap: spacing.md,
  },
  cardPressed: { opacity: 0.85 },
  topRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  bottomRow: { flexDirection: 'row', alignItems: 'center' },
  plate: { fontSize: fontSize.title, fontWeight: fontWeight.bold, color: colors.textPrimary },
  metaItem: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  icon: { fontSize: 16 },
  metaValue: { fontSize: fontSize.subtitle, fontWeight: fontWeight.bold, color: colors.textPrimary },
  metaTime: { fontSize: fontSize.body, fontWeight: fontWeight.medium, color: colors.textSecondary },
});
