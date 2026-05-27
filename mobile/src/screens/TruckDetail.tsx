/**
 * Truck Detail — real Mappls map with animated camera following the live
 * position, custom truck marker, recenter button.
 *
 * If the Mappls SDK isn't installed or keys are missing, falls back to a
 * neutral green tile so the rest of the UI still works during dev.
 */
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { View, Text, Pressable, StyleSheet, ActivityIndicator } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../../App';
import { getTruck } from '../lib/api';
import { statusFromPosition, formatRelativeTime } from '../lib/types';
import { useLivePositions } from '../hooks/useLivePositions';
import { getMappls, isMapplsReady } from '../lib/mappls';
import { StatusPill } from '../components/StatusPill';
import { colors, fontSize, fontWeight, radius, spacing } from '../theme';

type Props = NativeStackScreenProps<RootStackParamList, 'TruckDetail'>;

// Default center if we have no position yet (Delhi).
const DEFAULT_CENTER: [number, number] = [77.209, 28.6139];
const DEFAULT_ZOOM = 14;

export function TruckDetailScreen({ route, navigation }: Props) {
  const { truckId, plate } = route.params;

  useLivePositions();

  const { data: truck, isLoading } = useQuery({
    queryKey: ['truck', truckId],
    queryFn: () => getTruck(truckId),
    refetchInterval: 30_000, // safety net if WS drops
  });

  const status = statusFromPosition(truck?.latest);

  const coords: [number, number] | null = truck?.latest
    ? [truck.latest.lng, truck.latest.lat]
    : null;

  return (
    <View style={styles.root}>
      <Map coords={coords} />

      {/* TOP BAR */}
      <SafeAreaView edges={['top']} style={styles.topBarWrap} pointerEvents="box-none">
        <View style={styles.topBar}>
          <Pressable onPress={() => navigation.goBack()} style={styles.backBtn} hitSlop={8}>
            <Text style={styles.backArrow}>←</Text>
          </Pressable>
          <View style={{ flex: 1 }}>
            <Text style={styles.topPlate}>{plate}</Text>
            <Text style={styles.topSub} numberOfLines={1}>
              {truck?.latest
                ? `${truck.latest.lat.toFixed(4)}, ${truck.latest.lng.toFixed(4)}`
                : 'Waiting for first position…'}
            </Text>
          </View>
        </View>
      </SafeAreaView>

      {/* BOTTOM SHEET */}
      <View style={styles.sheet}>
        <View style={styles.grab} />
        {isLoading || !truck ? (
          <View style={{ alignItems: 'center', paddingVertical: spacing['3xl'] }}>
            <ActivityIndicator color={colors.primary} />
          </View>
        ) : (
          <>
            <View style={styles.sheetTop}>
              <Text style={styles.sheetPlate}>{truck.device.plate}</Text>
              <StatusPill status={status} />
            </View>
            <View style={styles.stats}>
              <Stat
                label="Speed"
                value={`${truck.latest ? Math.round(truck.latest.speed) : 0} km/h`}
                icon="💨"
              />
              <Stat label="Updated" value={formatRelativeTime(truck.latest?.at)} icon="🕐" />
              <Stat
                label="Heading"
                value={`${truck.latest ? Math.round(truck.latest.heading) : 0}°`}
                icon="🧭"
              />
            </View>
          </>
        )}
      </View>
    </View>
  );
}

// ---------------------------------------------------------------------------
// Map sub-component — isolates the Mappls bits so the parent stays clean
// and screen-level state (sheet, top bar) is independent of map readiness.
// ---------------------------------------------------------------------------

function Map({ coords }: { coords: [number, number] | null }) {
  const Mappls = getMappls();
  const cameraRef = useRef<any>(null);
  const [followLive, setFollowLive] = useState(true);

  // Animate the camera whenever a new coord lands AND we're in follow mode.
  useEffect(() => {
    if (!coords || !followLive || !cameraRef.current) return;
    try {
      cameraRef.current.setCamera({
        centerCoordinate: coords,
        zoomLevel: DEFAULT_ZOOM,
        animationMode: 'flyTo',
        animationDuration: 800,
      });
    } catch {
      /* SDK not ready or stub */
    }
  }, [coords, followLive]);

  // No SDK / no keys → placeholder.
  if (!Mappls || !isMapplsReady()) {
    return (
      <View style={[StyleSheet.absoluteFillObject, styles.mapFallback]}>
        <Text style={styles.fallbackText}>
          Mappls SDK not configured.{'\n'}See mobile README.
        </Text>
      </View>
    );
  }

  const { MapView, Camera, PointAnnotation } = Mappls;
  const center = coords ?? DEFAULT_CENTER;

  return (
    <>
      <MapView style={StyleSheet.absoluteFillObject} logoEnabled compassEnabled>
        <Camera
          ref={cameraRef}
          defaultSettings={{ centerCoordinate: center, zoomLevel: DEFAULT_ZOOM }}
          animationMode="flyTo"
        />
        {coords && (
          <PointAnnotation id="truck" coordinate={coords}>
            <View style={styles.markerWrap}>
              <View style={styles.markerBubble}>
                <Text style={styles.markerEmoji}>🚚</Text>
              </View>
              <View style={styles.markerTail} />
            </View>
          </PointAnnotation>
        )}
      </MapView>

      {/* Recenter / lock-follow toggle */}
      <Pressable
        onPress={() => setFollowLive(true)}
        style={[styles.recenter, followLive && styles.recenterActive]}
        hitSlop={8}
      >
        <Text style={styles.recenterIcon}>📍</Text>
      </Pressable>
    </>
  );
}

function Stat({ label, value, icon }: { label: string; value: string; icon: string }) {
  return (
    <View style={styles.stat}>
      <View style={styles.statRow}>
        <Text style={styles.statIcon}>{icon}</Text>
        <Text style={styles.statLabel}>{label}</Text>
      </View>
      <Text style={styles.statValue}>{value}</Text>
    </View>
  );
}

// ---------------------------------------------------------------------------
// styles
// ---------------------------------------------------------------------------

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: colors.bg },
  mapFallback: { backgroundColor: '#E6F0E6', alignItems: 'center', justifyContent: 'center' },
  fallbackText: { color: colors.textSecondary, textAlign: 'center', fontSize: fontSize.body },

  topBarWrap: { position: 'absolute', top: 0, left: 0, right: 0 },
  topBar: {
    flexDirection: 'row', alignItems: 'center', gap: spacing.md,
    margin: spacing.lg, paddingHorizontal: spacing.md + 2, paddingVertical: spacing.md + 2,
    backgroundColor: colors.surface, borderRadius: radius.xl,
    shadowColor: '#000', shadowOffset: { width: 0, height: 2 }, shadowOpacity: 0.08, shadowRadius: 8, elevation: 3,
  },
  backBtn: {
    width: 40, height: 40, borderRadius: 999, backgroundColor: colors.surfaceAlt,
    alignItems: 'center', justifyContent: 'center',
  },
  backArrow: { fontSize: 22, fontWeight: fontWeight.bold, color: colors.textPrimary },
  topPlate: { fontSize: fontSize.subtitle, fontWeight: fontWeight.bold, color: colors.textPrimary },
  topSub: { fontSize: fontSize.small, color: colors.textSecondary, marginTop: 1 },

  // marker
  markerWrap: { alignItems: 'center' },
  markerBubble: {
    backgroundColor: colors.primary, borderRadius: 24, borderWidth: 3, borderColor: '#fff',
    paddingHorizontal: 10, paddingVertical: 6,
    shadowColor: '#000', shadowOffset: { width: 0, height: 4 }, shadowOpacity: 0.25, shadowRadius: 8,
  },
  markerEmoji: { fontSize: 22 },
  markerTail: {
    width: 10, height: 10, backgroundColor: colors.primary,
    transform: [{ rotate: '45deg' }],
    marginTop: -4, borderRightWidth: 3, borderBottomWidth: 3, borderColor: '#fff',
  },

  // recenter FAB
  recenter: {
    position: 'absolute', right: spacing.lg, bottom: 300,
    width: 48, height: 48, borderRadius: 999, backgroundColor: colors.surface,
    alignItems: 'center', justifyContent: 'center',
    shadowColor: '#000', shadowOffset: { width: 0, height: 4 }, shadowOpacity: 0.15, shadowRadius: 12, elevation: 4,
  },
  recenterActive: { backgroundColor: '#E6EBFB' },
  recenterIcon: { fontSize: 22 },

  // sheet
  sheet: {
    position: 'absolute', bottom: 0, left: 0, right: 0,
    backgroundColor: colors.surface, borderTopLeftRadius: 28, borderTopRightRadius: 28,
    paddingTop: spacing.md + 2, paddingBottom: spacing['2xl'] + spacing.sm,
    paddingHorizontal: spacing['2xl'],
    shadowColor: '#000', shadowOffset: { width: 0, height: -4 }, shadowOpacity: 0.12, shadowRadius: 24, elevation: 12,
  },
  grab: { alignSelf: 'center', width: 48, height: 5, borderRadius: 999, backgroundColor: colors.divider, marginBottom: spacing.md },
  sheetTop: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: spacing.lg },
  sheetPlate: { fontSize: fontSize.titleLg, fontWeight: fontWeight.bold, color: colors.textPrimary },

  stats: { flexDirection: 'row', gap: spacing.sm + 2 },
  stat: { flex: 1, backgroundColor: colors.surfaceAlt, borderRadius: radius.md + 2, padding: spacing.md, gap: spacing.xs },
  statRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.xs },
  statIcon: { fontSize: 14 },
  statLabel: { fontSize: fontSize.caption, fontWeight: fontWeight.medium, color: colors.textSecondary },
  statValue: { fontSize: fontSize.subtitle + 2, fontWeight: fontWeight.bold, color: colors.textPrimary },
});
