/**
 * Mappls SDK initialization.
 *
 * Call `initMappls()` exactly once at app startup, before any MapView mounts.
 * The Mappls RN SDK is loaded lazily (require) so the app still boots if the
 * native module isn't linked yet — `getMappls()` returns null in that case
 * and screens fall back to a placeholder view.
 */
import { env } from './env';

type MapplsModule = {
  MapplsGL: {
    setMapSDKKey: (k: string) => void;
    setRestAPIKey: (k: string) => void;
    setAtlasClientId: (k: string) => void;
    setAtlasClientSecret: (k: string) => void;
    MapView: React.ComponentType<any>;
    Camera: React.ComponentType<any>;
    PointAnnotation: React.ComponentType<any>;
    UserLocation?: React.ComponentType<any>;
  };
};

let cached: MapplsModule | null | undefined;

function load(): MapplsModule | null {
  if (cached !== undefined) return cached;
  try {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    cached = require('mappls-map-react-native');
  } catch {
    cached = null;
  }
  return cached!;
}

let initialized = false;

export function initMappls(): boolean {
  if (initialized) return true;
  const m = load();
  if (!m) return false;
  if (!env.MAPPLS_MAP_SDK_KEY) return false;
  try {
    m.MapplsGL.setMapSDKKey(env.MAPPLS_MAP_SDK_KEY);
    m.MapplsGL.setRestAPIKey(env.MAPPLS_REST_API_KEY);
    m.MapplsGL.setAtlasClientId(env.MAPPLS_CLIENT_ID);
    m.MapplsGL.setAtlasClientSecret(env.MAPPLS_CLIENT_SECRET);
    initialized = true;
    return true;
  } catch (err) {
    // eslint-disable-next-line no-console
    console.warn('[mappls] init failed:', err);
    return false;
  }
}

export function getMappls(): MapplsModule['MapplsGL'] | null {
  const m = load();
  return m ? m.MapplsGL : null;
}

export function isMapplsReady(): boolean {
  return initialized;
}
