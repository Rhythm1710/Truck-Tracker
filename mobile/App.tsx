/**
 * App entry — wires React Navigation + React Query + auth gate.
 *
 * On boot we:
 *  1. Initialize the Mappls SDK once (no-op if keys missing).
 *  2. Hydrate the persisted auth session from AsyncStorage.
 * Only then do we mount the navigator, so the user doesn't see a flicker
 * of the Login screen before being routed to Home.
 */
import React, { useEffect, useState } from 'react';
import { View, ActivityIndicator, StatusBar, StyleSheet } from 'react-native';
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
import { colors } from './src/theme';

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
    initMappls(); // ok if it returns false — TruckDetail falls back
    hydrateAuth().finally(() => setReady(true));
  }, []);

  const isAuthed = useAuthStore((s) => s.isAuthenticated());

  if (!ready) {
    return (
      <View style={styles.splash}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <StatusBar barStyle="dark-content" backgroundColor="transparent" translucent />
        <NavigationContainer>
          <Stack.Navigator screenOptions={{ headerShown: false }}>
            {!isAuthed ? (
              <Stack.Screen name="Login" component={LoginScreen} />
            ) : (
              <>
                <Stack.Screen name="Home" component={HomeScreen} />
                <Stack.Screen name="TruckDetail" component={TruckDetailScreen} />
              </>
            )}
          </Stack.Navigator>
        </NavigationContainer>
      </QueryClientProvider>
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  splash: { flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.surface },
});
