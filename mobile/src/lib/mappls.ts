/**
 * Mappls SDK initialization.
 *
 * Call `initMappls()` exactly once at app startup, before any MapView mounts.
 * The Mappls RN SDK is loaded lazily (require) so the app still boots if the
 * native module isn't linked yet — `getMappls()` returns null in that case
 * and screens fall back to a placeholder view.
 *
 * On iOS the SDK reads credentials from bundled .i.olf/.i.conf license files,
 * so the JS-level setMapSDKKey/etc. calls are not needed (and don't exist on
 * the TurboModule). We still attempt them for Android compatibility.
 */
import { Platform } from 'react-native';
import { env } from './env';

type MapplsGL = {
  setMapSDKKey?: (k: string) => void;
  setRestAPIKey?: (k: string) => void;
  setAtlasClientId?: (k: string) => void;
  setAtlasClientSecret?: (k: string) => void;
  MapView: React.ComponentType<any>;
  Camera: React.ComponentType<any>;
  PointAnnotation: React.ComponentType<any>;
  UserLocation?: React.ComponentType<any>;
};

let cached: MapplsGL | null | undefined;

function load(): MapplsGL | null {
  if (cached !== undefined) return cached;
  try {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    const mod = require('mappls-map-react-native');
    cached = mod.default ?? mod;
  } catch {
    cached = null;
  }
  return cached!;
}

let initialized = false;

export function initMappls(): boolean {
  if (initialized) return true;
  const gl = load();
  if (!gl) return false;
  try {
    if (typeof gl.setMapSDKKey === 'function') {
      gl.setMapSDKKey(env.MAPPLS_MAP_SDK_KEY);
      gl.setRestAPIKey?.(env.MAPPLS_REST_API_KEY);
      gl.setAtlasClientId?.(env.MAPPLS_CLIENT_ID);
      gl.setAtlasClientSecret?.(env.MAPPLS_CLIENT_SECRET);
    } else if (Platform.OS === 'ios') {
      // iOS reads keys from bundled .i.olf / .i.conf license files
    } else {
      // eslint-disable-next-line no-console
      console.warn('[mappls] setMapSDKKey not available — map may not authenticate');
    }
    initialized = true;
    return true;
  } catch (err) {
    // eslint-disable-next-line no-console
    console.warn('[mappls] init failed:', err);
    return false;
  }
}

export function getMappls(): MapplsGL | null {
  const gl = load();
  return gl ?? null;
}

export function isMapplsReady(): boolean {
  return initialized;
}
