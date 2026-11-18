/**
 * Truck Detail — real Mappls map with animated camera following the live
 * position, custom truck marker, recenter button.
 *
 * If the Mappls SDK isn't installed or keys are missing, falls back to a
 * neutral green tile so the rest of the UI still works during dev.
 */
import React, { useCallback, useEffect, useRef, useState, useReducer } from 'react';
import { View, Text, Pressable, StyleSheet, ActivityIndicator, type LayoutChangeEvent } from 'react-native';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';
import { useQuery } from '@tanstack/react-query';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

import type { RootStackParamList } from '../../App';
import { getTruck } from '../lib/api';
import { statusFromPosition, formatRelativeTime } from '../lib/types';
import { useLivePositions } from '../hooks/useLivePositions';
import { getMappls, isMapplsReady } from '../lib/mappls';
import { StatusPill } from '../components/StatusPill';
import { colors, fontSize, fontWeight, minTouchTarget, radius, spacing } from '../theme';
import { haptics } from '../lib/haptics';

type Props = NativeStackScreenProps<RootStackParamList, 'TruckDetail'>;

// Default center if we have no position yet (Delhi).
const DEFAULT_CENTER: [number, number] = [77.209, 28.6139];
const DEFAULT_ZOOM = 14;

export function TruckDetailScreen({ route, navigation }: Props) {
  const { truckId, plate } = route.params;
  const insets = useSafeAreaInsets();
  const [sheetHeight, setSheetHeight] = useState(0);

  useLivePositions();

  // Tick every 30s so relative timestamps ("2 min ago") stay current.
  const [, tick] = useReducer(n => n + 1, 0);
  useEffect(() => {
    const id = setInterval(tick, 30_000);
    return () => clearInterval(id);
  }, []);

  const { data: truck, isLoading } = useQuery({
    queryKey: ['truck', truckId],
    queryFn: () => getTruck(truckId),
    refetchInterval: 30_000, // safety net if WS drops
  });

  const status = statusFromPosition(truck?.latest);

  // Pass lat/lng as primitives so the Map's useEffect doesn't re-run when
  // the parent re-renders with a fresh array reference.
  const lat = truck?.latest?.lat;
  const lng = truck?.latest?.lng;
  const heading = truck?.latest?.heading ?? 0;
  const speed = truck?.latest?.speed ?? 0;
  const ignition = truck?.latest?.ignition;

  const onSheetLayout = useCallback((e: LayoutChangeEvent) => {
    const next = e.nativeEvent.layout.height;
    setSheetHeight(prev => (prev === next ? prev : next));
  }, []);

  return (
    <View style={styles.root}>
      <Map
        lat={lat}
        lng={lng}
        heading={heading}
        moving={speed > 5}
        fabBottom={sheetHeight > 0 ? sheetHeight + spacing.md : null}
      />

      {/* TOP BAR */}
      <SafeAreaView edges={['top']} style={styles.topBarWrap} pointerEvents="box-none">
        <View style={styles.topBar}>
          <Pressable
            onPress={() => { haptics.tap(); navigation.goBack(); }}
            style={({ pressed }) => [styles.backBtn, pressed && { opacity: 0.7 }]}
            hitSlop={12}
            accessibilityRole="button"
            accessibilityLabel="Back to trucks list"
            android_ripple={{ color: '#00000020', borderless: true }}
          >
            <Text style={styles.backArrow} maxFontSizeMultiplier={1.3}>←</Text>
          </Pressable>
          <View style={{ flex: 1 }}>
            <Text style={styles.topPlate} maxFontSizeMultiplier={1.3}>{plate}</Text>
            <Text style={styles.topSub} numberOfLines={1} maxFontSizeMultiplier={1.4}>
              {truck?.latest
                ? `${truck.latest.lat.toFixed(4)}, ${truck.latest.lng.toFixed(4)}`
                : 'Waiting for first position…'}
            </Text>
          </View>
        </View>
      </SafeAreaView>

      {/* BOTTOM SHEET */}
      <View
        style={[styles.sheet, { paddingBottom: spacing.lg + insets.bottom }]}
        onLayout={onSheetLayout}
      >
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
            <Text style={styles.ignition}>
              Ignition{' '}
              {ignition == null ? (
                <Text style={styles.ignitionUnknown}>—</Text>
              ) : (
                <Text style={ignition ? styles.ignitionOn : styles.ignitionOff}>
                  {ignition ? 'On' : 'Off'}
                </Text>
              )}
            </Text>
            <View style={styles.stats}>
              <Stat label="Speed" value={`${truck.latest ? Math.round(truck.latest.speed) : 0} km/h`} />
              <Stat label="Updated" value={formatRelativeTime(truck.latest?.at)} />
              <Stat label="Heading" value={formatHeading(truck.latest?.heading)} />
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

interface MapProps {
  lat?: number;
  lng?: number;
  heading: number;   // degrees, 0 = north
  moving: boolean;   // suppress rotation jitter at standstill
  /** Pixels from the bottom edge. Null until the sheet has been measured. */
  fabBottom: number | null;
}

function Map({ lat, lng, heading, moving, fabBottom }: MapProps) {
  const Mappls = getMappls();
  const cameraRef = useRef<any>(null);
  const markerRef = useRef<{ refresh?: () => void } | null>(null);
  const [followLive, setFollowLive] = useState(true);

  // Captured once. A fresh [lng, lat] array on later renders must not be
  // handed to Camera as a new defaultSettings identity.
  const defaultSettingsRef = useRef({
    centerCoordinate: (lat != null && lng != null ? [lng, lat] : DEFAULT_CENTER) as [number, number],
    zoomLevel: DEFAULT_ZOOM,
  });

  // Animate the camera ONLY when lat/lng actually change AND follow is on.
  // Primitives in the dep list mean React.useEffect compares by value.
  useEffect(() => {
    if (lat == null || lng == null || !followLive || !cameraRef.current) return;
    try {
      cameraRef.current.setCamera({
        centerCoordinate: [lng, lat],
        animationMode: 'flyTo',
        animationDuration: 800,
      });
    } catch {
      /* SDK not ready or stub */
    }
  }, [lat, lng, followLive]);

  // PointAnnotation on Android snapshots its child into a bitmap. Refresh
  // after heading changes so the rotation actually shows.
  useEffect(() => {
    const id = requestAnimationFrame(() => {
      markerRef.current?.refresh?.();
    });
    return () => cancelAnimationFrame(id);
  }, [heading, moving]);

  // User-gesture handler: disengage follow on pan/zoom so the map doesn't
  // snap back when the next position update arrives.
  const onRegionWillChange = useCallback((e: any) => {
    if (e?.properties?.isUserInteraction) {
      setFollowLive(false);
    }
  }, []);

  // No SDK / no keys → placeholder.
  if (!Mappls || !isMapplsReady()) {
    return (
      <View style={[StyleSheet.absoluteFill, styles.mapFallback]}>
        <Text style={styles.fallbackText}>
          Mappls SDK not configured.{'\n'}See mobile README.
        </Text>
      </View>
    );
  }

  const { MapView, Camera, PointAnnotation } = Mappls;
  const haveCoords = lat != null && lng != null;

  return (
    <>
      <MapView
        style={StyleSheet.absoluteFill}
        logoEnabled
        compassEnabled
        onRegionWillChange={onRegionWillChange}
      >
        <Camera
          ref={cameraRef}
          defaultSettings={defaultSettingsRef.current}
          animationMode="flyTo"
        />
        {haveCoords && (
          <PointAnnotation ref={markerRef} id="truck" coordinate={[lng!, lat!]}>
            <View style={styles.markerWrap}>
              <View
                style={[
                  styles.markerBubble,
                  moving && { transform: [{ rotate: `${heading}deg` }] },
                ]}
              >
                <Text style={styles.markerEmoji}>🚚</Text>
              </View>
              <View style={styles.markerTail} />
            </View>
          </PointAnnotation>
        )}
      </MapView>

      {/* Recenter / re-engage live-follow. Sits just above the sheet, whose
          height includes the home-indicator inset. */}
      {fabBottom != null && (
        <Pressable
          onPress={() => setFollowLive(true)}
          style={[styles.recenter, { bottom: fabBottom }, followLive && styles.recenterActive]}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={followLive ? 'Live follow on' : 'Recenter map on truck'}
        >
          <Text style={styles.recenterIcon}>◎</Text>
        </Pressable>
      )}
    </>
  );
}

function formatHeading(heading: number | undefined): string {
  if (heading == null || Number.isNaN(heading)) return '—';
  return `${Math.round(heading)}°`;
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.stat}>
      <Text style={styles.statLabel}>{label}</Text>
      <Text style={styles.statValue} numberOfLines={1}>{value}</Text>
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
    marginTop: spacing.md, marginHorizontal: spacing.lg,
    paddingHorizontal: spacing.md, paddingVertical: 10,
    backgroundColor: colors.surface, borderRadius: radius.xl,
    shadowColor: '#000', shadowOffset: { width: 0, height: 2 }, shadowOpacity: 0.08, shadowRadius: 8, elevation: 3,
  },
  backBtn: {
    width: minTouchTarget, height: minTouchTarget,
    borderRadius: 999, backgroundColor: colors.surfaceAlt,
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
    position: 'absolute', right: spacing.lg,
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
    paddingTop: 14,
    paddingHorizontal: spacing['2xl'],
    shadowColor: '#000', shadowOffset: { width: 0, height: -4 }, shadowOpacity: 0.12, shadowRadius: 24, elevation: 12,
  },
  grab: { alignSelf: 'center', width: 48, height: 5, borderRadius: 99, backgroundColor: colors.divider, marginBottom: 14 },
  sheetTop: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 },
  sheetPlate: {
    fontSize: fontSize.titleLg,
    fontWeight: fontWeight.bold,
    letterSpacing: -0.4,
    color: colors.textPrimary,
  },
  ignition: { fontSize: 14, fontWeight: fontWeight.medium, color: colors.textSecondary, marginBottom: 14 },
  ignitionOn: { fontWeight: fontWeight.semibold, color: colors.statusRunning },
  ignitionOff: { fontWeight: fontWeight.medium, color: colors.textSecondary },
  ignitionUnknown: { fontWeight: fontWeight.medium, color: colors.textSecondary },

  stats: { flexDirection: 'row', gap: 10 },
  stat: { flex: 1, backgroundColor: colors.surfaceAlt, borderRadius: 14, padding: spacing.md },
  statLabel: { fontSize: fontSize.caption, fontWeight: fontWeight.medium, color: colors.textSecondary, marginBottom: 4 },
  statValue: { fontSize: 17, fontWeight: fontWeight.bold, letterSpacing: -0.3, color: colors.textPrimary },
});
