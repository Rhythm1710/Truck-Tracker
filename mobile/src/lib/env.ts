/**
 * Mobile runtime config.
 *
 * For dev, edit the values below directly (do NOT commit secrets).
 * For production, replace this file with a build-time inject — e.g.
 * `react-native-config` — and import `Config.XXX` instead.
 *
 * Quick rule of thumb for API_BASE:
 *   - Android emulator   → http://10.0.2.2:8080
 *   - iOS simulator      → http://localhost:8080
 *   - Real device on LAN → http://<your-mac-LAN-ip>:8080
 */
export const env = {
  API_BASE: 'http://10.0.2.2:8080',

  // Paste the rotated Mappls keys here. NEVER commit this file with real values.
  MAPPLS_MAP_SDK_KEY: '',
  MAPPLS_REST_API_KEY: '',
  MAPPLS_CLIENT_ID: '',
  MAPPLS_CLIENT_SECRET: '',
};

export function assertEnv(): void {
  const missing: string[] = [];
  if (!env.MAPPLS_MAP_SDK_KEY) missing.push('MAPPLS_MAP_SDK_KEY');
  if (!env.MAPPLS_REST_API_KEY) missing.push('MAPPLS_REST_API_KEY');
  if (!env.MAPPLS_CLIENT_ID) missing.push('MAPPLS_CLIENT_ID');
  if (!env.MAPPLS_CLIENT_SECRET) missing.push('MAPPLS_CLIENT_SECRET');
  if (missing.length) {
    // Don't crash — log so dev can keep going against placeholder map.
    // eslint-disable-next-line no-console
    console.warn(`[env] missing Mappls keys: ${missing.join(', ')} — map will use fallback`);
  }
}
