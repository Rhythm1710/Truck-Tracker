/**
 * Home screen truck card.
 *
 * Memoized: a card only re-renders when its truck's identity or `latest.at`
 * actually changes. Lets Home stay smooth when one of 5 trucks moves but the
 * others haven't reported.
 */
import React, { useEffect, useState } from 'react';
import { View, Text, Pressable, StyleSheet, Platform } from 'react-native';

import { StatusPill } from './StatusPill';
import { colors, fontSize, fontWeight, minTouchTarget, radius, spacing } from '../theme';
import { statusFromPosition, formatRelativeTime } from '../lib/types';
import type { TruckSummary } from '../lib/types';
import { haptics } from '../lib/haptics';

interface Props {
  truck: TruckSummary;
  onPress: () => void;
}

function TruckCardImpl({ truck, onPress }: Props) {
  const { device, latest } = truck;
  const status = statusFromPosition(latest);
  const speed = latest ? Math.round(latest.speed) : 0;

  const handlePress = () => {
    haptics.tap();
    onPress();
  };

  return (
    <Pressable
      onPress={handlePress}
      accessibilityRole="button"
      accessibilityLabel={`${device.plate}, ${status}, ${speed} kilometers per hour, updated ${formatRelativeTime(latest?.at)}`}
      accessibilityHint="Opens truck details with live map"
      style={({ pressed }) => [
        styles.card,
        pressed && styles.cardPressed,
        { minHeight: minTouchTarget * 1.5 },
      ]}
      android_ripple={{ color: '#00000010' }}
    >
      <View style={styles.topRow}>
        <View style={styles.identity}>
          <Text style={styles.plate} maxFontSizeMultiplier={1.3}>{device.plate}</Text>
          {device.label ? (
            <Text style={styles.truckLabel} maxFontSizeMultiplier={1.3} numberOfLines={1}>
              {device.label}
            </Text>
          ) : null}
        </View>
        <StatusPill status={status} compact />
      </View>
      <View style={styles.bottomRow}>
        <Text style={styles.metaValue} maxFontSizeMultiplier={1.3}>{speed} km/h</Text>
        <RelativeTime iso={latest?.at} style={styles.metaTime} />
      </View>
    </Pressable>
  );
}

function RelativeTime({ iso, style }: { iso?: string; style?: object }) {
  const [, tick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => tick(n => n + 1), 30_000);
    return () => clearInterval(id);
  }, []);
  return <Text style={style} maxFontSizeMultiplier={1.3}>{formatRelativeTime(iso)}</Text>;
}

/** Re-render only when the data the card displays actually changes. */
export const TruckCard = React.memo(TruckCardImpl, (prev: Props, next: Props) => {
  return (
    prev.truck.device.id === next.truck.device.id &&
    prev.truck.device.plate === next.truck.device.plate &&
    prev.truck.device.label === next.truck.device.label &&
    prev.truck.latest?.at === next.truck.latest?.at &&
    prev.truck.latest?.speed === next.truck.latest?.speed &&
    prev.truck.latest?.ignition === next.truck.latest?.ignition &&
    prev.onPress === next.onPress
  );
});

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.xl,
    paddingHorizontal: 20,
    paddingVertical: 16,
    borderWidth: 1,
    borderColor: colors.border,
    ...Platform.select({
      ios: {
        shadowColor: '#000',
        shadowOffset: { width: 0, height: 2 },
        shadowOpacity: 0.04,
        shadowRadius: 6,
      },
      android: { elevation: 1 },
    }),
    overflow: 'hidden', // keeps android_ripple inside the rounded corners
  },
  cardPressed: { transform: [{ scale: 0.99 }] },
  topRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start', gap: spacing.sm },
  identity: { flex: 1 },
  bottomRow: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: spacing.md },
  plate: {
    fontSize: fontSize.title,
    fontWeight: fontWeight.bold,
    letterSpacing: -0.4,
    color: colors.textPrimary,
  },
  truckLabel: {
    marginTop: 2,
    fontSize: 14,
    fontWeight: fontWeight.medium,
    color: colors.textSecondary,
  },
  metaValue: { fontSize: fontSize.subtitle, fontWeight: fontWeight.bold, color: colors.textPrimary },
  metaTime: { fontSize: fontSize.body, fontWeight: fontWeight.medium, color: colors.textSecondary },
});
