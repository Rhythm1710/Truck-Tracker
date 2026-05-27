/**
 * Home — list of trucks with status summary, live updates via WebSocket.
 */
import React, { useMemo } from 'react';
import { View, Text, ScrollView, StyleSheet, RefreshControl, ActivityIndicator } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../../App';
import { listTrucks } from '../lib/api';
import { statusFromPosition } from '../lib/types';
import { useLivePositions } from '../hooks/useLivePositions';
import { TruckCard } from '../components/TruckCard';
import { colors, fontSize, fontWeight, radius, spacing } from '../theme';

type Props = NativeStackScreenProps<RootStackParamList, 'Home'>;

export function HomeScreen({ navigation }: Props) {
  // Live updates merge into the cache for this query key.
  useLivePositions();

  const { data: trucks, isLoading, refetch, isFetching } = useQuery({
    queryKey: ['trucks'],
    queryFn: listTrucks,
  });

  const counts = useMemo(() => {
    const c = { running: 0, idle: 0, stopped: 0, offline: 0, total: 0 };
    for (const t of trucks ?? []) {
      const s = statusFromPosition(t.latest);
      c.total++;
      if (s === 'Running') c.running++;
      else if (s === 'Idle') c.idle++;
      else if (s === 'Stopped') c.stopped++;
      else c.offline++;
    }
    return c;
  }, [trucks]);

  return (
    <SafeAreaView style={styles.safe} edges={['top']}>
      <View style={styles.header}>
        <View style={{ flex: 1 }}>
          <Text style={styles.greeting}>Namaste 🙏</Text>
          <Text style={styles.title}>My Trucks</Text>
        </View>
        <View style={styles.avatar}>
          <Text style={styles.avatarText}>R</Text>
        </View>
      </View>

      <View style={styles.summary}>
        <Pill label={`${counts.running} Running`} color={colors.statusRunning} />
        <Pill label={`${counts.idle} Idle`} color={colors.statusIdle} />
        <Pill label={`Total ${counts.total}`} color={colors.textSecondary} />
      </View>

      <View style={styles.separator} />

      <ScrollView
        contentContainerStyle={styles.list}
        refreshControl={<RefreshControl refreshing={isFetching && !isLoading} onRefresh={refetch} />}
      >
        {isLoading ? (
          <View style={styles.loader}><ActivityIndicator size="large" color={colors.primary} /></View>
        ) : trucks && trucks.length > 0 ? (
          trucks.map((t) => (
            <TruckCard
              key={t.device.id}
              truck={t}
              onPress={() => navigation.navigate('TruckDetail', { truckId: t.device.id, plate: t.device.plate })}
            />
          ))
        ) : (
          <View style={styles.loader}>
            <Text style={styles.empty}>No trucks yet.{'\n'}Add a device to start tracking.</Text>
          </View>
        )}
      </ScrollView>
    </SafeAreaView>
  );
}

function Pill({ label, color }: { label: string; color: string }) {
  return (
    <View style={styles.pill}>
      <View style={[styles.pillDot, { backgroundColor: color }]} />
      <Text style={styles.pillLabel}>{label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.bg },
  header: {
    flexDirection: 'row', alignItems: 'center',
    paddingHorizontal: spacing['2xl'], paddingTop: spacing.lg, paddingBottom: spacing.lg,
    backgroundColor: colors.surface,
  },
  greeting: { fontSize: fontSize.bodyLg, color: colors.textSecondary, marginBottom: 2 },
  title: { fontSize: fontSize.display, fontWeight: fontWeight.bold, color: colors.textPrimary },
  avatar: {
    width: 48, height: 48, borderRadius: 24, backgroundColor: colors.primary,
    alignItems: 'center', justifyContent: 'center',
  },
  avatarText: { color: colors.primaryText, fontSize: fontSize.subtitle + 2, fontWeight: fontWeight.bold },
  summary: {
    flexDirection: 'row', gap: spacing.sm, backgroundColor: colors.surface,
    paddingHorizontal: spacing['2xl'], paddingBottom: spacing.md,
  },
  pill: {
    flexDirection: 'row', alignItems: 'center', gap: spacing.sm - 2,
    backgroundColor: colors.surfaceAlt, paddingHorizontal: spacing.md, paddingVertical: 6, borderRadius: radius.pill,
  },
  pillDot: { width: 8, height: 8, borderRadius: 999 },
  pillLabel: { fontSize: fontSize.small, fontWeight: fontWeight.semibold, color: colors.textPrimary },
  separator: { height: 1, backgroundColor: colors.border },
  list: { padding: spacing.lg, gap: spacing.md },
  loader: { paddingTop: spacing['4xl'], alignItems: 'center' },
  empty: { color: colors.textSecondary, textAlign: 'center', fontSize: fontSize.bodyLg },
});
