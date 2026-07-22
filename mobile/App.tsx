/**
 * App entry — providers + navigation + auth gate.
 *
 * Boot sequence:
 *   1. Initialize the Mappls SDK (no-op if keys missing).
 *   2. Hydrate persisted auth from AsyncStorage.
 *   3. Mount navigator — no flash of Login between hydrate and Home.
 *
 * The splash screen during step 2 is a branded view (icon + wordmark + spinner)
 * so it feels intentional instead of "loading…".
 */
import React, { useEffect, useState } from 'react';
import { View, Text, ActivityIndicator, StatusBar, StyleSheet } from 'react-native';
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import auth from '@react-native-firebase/auth';
import { LoginScreen } from './src/screens/Login';
import { HomeScreen } from './src/screens/Home';
import { TruckDetailScreen } from './src/screens/TruckDetail';
import { useAuthStore, hydrateAuth } from './src/store/auth';
import { initMappls } from './src/lib/mappls';
import { assertEnv } from './src/lib/env';
import { LiveProvider } from './src/lib/LiveProvider';
import { colors, fontSize, fontWeight, radius, spacing } from './src/theme';

if (__DEV__) {
  auth().settings.appVerificationDisabledForTesting = true;
}

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
});

export type RootStackParamList = {
  Login: undefined;
  Home: undefined;
  TruckDetail: { truckId: string; plate: string };
};

const Stack = createNativeStackNavigator<RootStackParamList>();

export default function App() {
  const [ready, setReady] = useState(false);

  useEffect(() => {
    assertEnv();
    initMappls();
    hydrateAuth().finally(() => setReady(true));
  }, []);

  const isAuthed = useAuthStore((s) => s.isAuthenticated());

  if (!ready) return <Splash />;

  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <LiveProvider enabled={isAuthed}>
          <StatusBar barStyle="dark-content" backgroundColor="transparent" translucent />
          <NavigationContainer>
            <Stack.Navigator screenOptions={{ headerShown: false, animation: 'fade' }}>
              {!isAuthed ? (
                <Stack.Screen name="Login" component={LoginScreen} />
              ) : (
                <>
                  <Stack.Screen name="Home" component={HomeScreen} />
                  <Stack.Screen
                    name="TruckDetail"
                    component={TruckDetailScreen}
                    options={{ animation: 'slide_from_right' }}
                  />
                </>
              )}
            </Stack.Navigator>
          </NavigationContainer>
        </LiveProvider>
      </QueryClientProvider>
    </SafeAreaProvider>
  );
}

function Splash() {
  return (
    <View style={styles.splash}>
      <View style={styles.iconBox}>
        <Text style={styles.iconEmoji}>🚚</Text>
      </View>
      <Text style={styles.wordmark}>Truck Tracker</Text>
      <ActivityIndicator size="small" color={colors.textSecondary} style={styles.spinner} />
    </View>
  );
}

const styles = StyleSheet.create({
  splash: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surface,
    gap: spacing.lg,
  },
  iconBox: {
    width: 88,
    height: 88,
    backgroundColor: colors.primary,
    borderRadius: radius['2xl'],
    alignItems: 'center',
    justifyContent: 'center',
  },
  iconEmoji: { fontSize: 48 },
  wordmark: {
    fontSize: fontSize.display,
    fontWeight: fontWeight.bold,
    color: colors.textPrimary,
  },
  spinner: { marginTop: spacing['2xl'] },
});
