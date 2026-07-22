/**
 * Home — list of trucks with live status, summary counts, connectivity banner.
 *
 * Render perf:
 *   - FlatList with `keyExtractor` → row recycling.
 *   - TruckCard is React.memo'd so unmoved cards skip re-rendering when one
 *     truck reports.
 *   - `counts` memoized on the trucks array reference.
 */
import React, { useCallback, useMemo } from 'react';
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
  ActivityIndicator,
  Pressable,
  Alert,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { getAuth, signOut } from '@react-native-firebase/auth';

import type { RootStackParamList } from '../../App';
import { listTrucks } from '../lib/api';
import { statusFromPosition } from '../lib/types';
import type { TruckSummary } from '../lib/types';
import { useLive } from '../lib/LiveProvider';
import { useAuthStore } from '../store/auth';
import { TruckCard } from '../components/TruckCard';
import { haptics } from '../lib/haptics';
import { colors, fontSize, fontWeight, minTouchTarget, radius, spacing } from '../theme';

type Props = NativeStackScreenProps<RootStackParamList, 'Home'>;

export function HomeScreen({ navigation }: Props) {
  const clearSession = useAuthStore((s) => s.clear);
  const { status: liveStatus } = useLive();

  const { data: trucks, isLoading, refetch, isFetching } = useQuery({
    queryKey: ['trucks'],
    queryFn: listTrucks,
  });

  const handleLogout = useCallback(() => {
    haptics.tap();
    Alert.alert('Sign out?', 'You\'ll need to verify your phone again to come back.', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Sign Out',
        style: 'destructive',
        onPress: async () => {
          try { await signOut(getAuth()); } catch { /* noop */ }
          clearSession();
        },
      },
    ]);
  }, [clearSession]);

  const handleCardPress = useCallback(
    (t: TruckSummary) => navigation.navigate('TruckDetail', {
      truckId: t.device.id,
      plate: t.device.plate,
    }),
    [navigation],
  );

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

  const renderItem = useCallback(
    ({ item }: { item: TruckSummary }) => (
      <TruckCard truck={item} onPress={() => handleCardPress(item)} />
    ),
    [handleCardPress],
  );

  return (
    <SafeAreaView style={styles.safe} edges={['top']}>
      <ConnectionBanner status={liveStatus} />

      <View style={styles.header}>
        <View style={{ flex: 1 }}>
          <Text style={styles.greeting} maxFontSizeMultiplier={1.4}>Namaste 🙏</Text>
          <Text style={styles.title} maxFontSizeMultiplier={1.3}>My Trucks</Text>
        </View>

        <Pressable
          onPress={handleLogout}
          accessibilityRole="button"
          accessibilityLabel="Sign out"
          accessibilityHint="Signs you out of the app"
          hitSlop={12}
          style={({ pressed }) => [styles.logoutBtn, pressed && styles.logoutBtnPressed]}
          android_ripple={{ color: '#ffffff44', borderless: true }}
        >
          <Text style={styles.logoutText} maxFontSizeMultiplier={1.3}>Sign out</Text>
        </Pressable>
      </View>

      <View style={styles.summary}>
        <Pill label={`${counts.running} Running`} color={colors.statusRunning} />
        <Pill label={`${counts.idle} Idle`} color={colors.statusIdle} />
        <Pill label={`Total ${counts.total}`} color={colors.textSecondary} />
      </View>

      <View style={styles.separator} />

      <FlatList
        data={trucks ?? []}
        keyExtractor={(t) => t.device.id}
        renderItem={renderItem}
        contentContainerStyle={styles.list}
        ItemSeparatorComponent={Separator}
        refreshControl={
          <RefreshControl
            refreshing={isFetching && !isLoading}
            onRefresh={() => { haptics.tap(); refetch(); }}
            tintColor={colors.primary}
            colors={[colors.primary]}
          />
        }
        ListEmptyComponent={
          isLoading ? (
            <View style={styles.loader}>
              <ActivityIndicator size="large" color={colors.primary} />
            </View>
          ) : (
            <EmptyState />
          )
        }
      />
    </SafeAreaView>
  );
}

// ---------------------------------------------------------------------------
// sub-components
// ---------------------------------------------------------------------------

function ConnectionBanner({ status }: { status: ReturnType<typeof useLive>['status'] }) {
  if (status === 'open') return null;

  let label = '';
  // Explicit type widening — `colors.*` literal types would otherwise lock
  // bg to the first assignment.
  let bg: string = colors.connWarn;
  switch (status) {
    case 'connecting': label = 'Connecting…'; bg = colors.connWarn; break;
    case 'paused':     label = 'Paused (app in background)'; bg = colors.textSecondary; break;
    case 'offline':    label = 'Offline — retrying…'; bg = colors.connOffline; break;
  }

  return (
    <View style={[styles.banner, { backgroundColor: bg }]} accessibilityLiveRegion="polite">
      <Text style={styles.bannerText} maxFontSizeMultiplier={1.4}>{label}</Text>
    </View>
  );
}

function Pill({ label, color }: { label: string; color: string }) {
  return (
    <View style={styles.pill}>
      <View style={[styles.pillDot, { backgroundColor: color }]} />
      <Text style={styles.pillLabel} maxFontSizeMultiplier={1.4}>{label}</Text>
    </View>
  );
}

function Separator() {
  return <View style={{ height: spacing.md }} />;
}

function EmptyState() {
  return (
    <View style={styles.empty}>
      <View style={styles.emptyIcon}>
        <Text style={styles.emptyEmoji}>🚛</Text>
      </View>
      <Text style={styles.emptyTitle} maxFontSizeMultiplier={1.3}>No trucks yet</Text>
      <Text style={styles.emptyBody} maxFontSizeMultiplier={1.3}>
        Once a Pictor tracker connects, you'll see it here.
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.bg },

  banner: {
    paddingHorizontal: spacing['2xl'],
    paddingVertical: spacing.sm,
  },
  bannerText: { color: '#fff', fontSize: fontSize.body, fontWeight: fontWeight.semibold, textAlign: 'center' },

  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing['2xl'],
    paddingTop: spacing.lg,
    paddingBottom: spacing.lg,
    backgroundColor: colors.surface,
  },
  greeting: { fontSize: fontSize.bodyLg, color: colors.textSecondary, marginBottom: 2 },
  title: { fontSize: fontSize.display, fontWeight: fontWeight.bold, color: colors.textPrimary },

  logoutBtn: {
    minHeight: minTouchTarget,
    minWidth: 88,
    paddingHorizontal: spacing.lg,
    borderRadius: radius.pill,
    backgroundColor: colors.primary,
    alignItems: 'center',
    justifyContent: 'center',
  },
  logoutBtnPressed: { opacity: 0.9 },
  logoutText: { color: colors.primaryText, fontSize: fontSize.body, fontWeight: fontWeight.bold },

  summary: {
    flexDirection: 'row',
    gap: spacing.sm,
    backgroundColor: colors.surface,
    paddingHorizontal: spacing['2xl'],
    paddingBottom: spacing.md,
  },
  pill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm - 2,
    backgroundColor: colors.surfaceAlt,
    paddingHorizontal: spacing.md,
    paddingVertical: 6,
    borderRadius: radius.pill,
  },
  pillDot: { width: 8, height: 8, borderRadius: 999 },
  pillLabel: { fontSize: fontSize.body, fontWeight: fontWeight.semibold, color: colors.textPrimary },

  separator: { height: 1, backgroundColor: colors.border },

  list: { padding: spacing.lg, flexGrow: 1 },
  loader: { paddingTop: spacing['4xl'], alignItems: 'center' },

  empty: {
    alignItems: 'center',
    paddingTop: spacing['4xl'],
    paddingHorizontal: spacing['2xl'],
    gap: spacing.md,
  },
  emptyIcon: {
    width: 96,
    height: 96,
    borderRadius: radius['2xl'],
    backgroundColor: colors.surfaceAlt,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: spacing.md,
  },
  emptyEmoji: { fontSize: 56 },
  emptyTitle: { fontSize: fontSize.title, fontWeight: fontWeight.bold, color: colors.textPrimary },
  emptyBody: { fontSize: fontSize.body, color: colors.textSecondary, textAlign: 'center', lineHeight: 22 },
});
