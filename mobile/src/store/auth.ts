/**
 * Zustand auth store with AsyncStorage persistence.
 *
 * The store is created synchronously, but the persisted token is loaded
 * asynchronously. Call `hydrateAuth()` once at app start and `await` it
 * before mounting the navigator — otherwise the navigator will briefly
 * show the Login screen before flipping to Home.
 *
 * We deliberately implement persistence by hand (rather than zustand/middleware)
 * to keep dependencies lean and the rehydration flow explicit.
 */
import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';

const STORAGE_KEY = '@truck-tracker/auth/v1';

interface PersistedSession {
  apiToken: string;
  phone: string;
  expiresAt: number; // ms since epoch
}

interface AuthState {
  apiToken: string | null;
  phone: string | null;
  expiresAt: number | null;
  hydrated: boolean;

  setSession: (apiToken: string, expiresAt: string, phone: string) => void;
  clear: () => void;
  isAuthenticated: () => boolean;

  /** Internal — set by hydrateAuth() once AsyncStorage has been read. */
  _markHydrated: (session: PersistedSession | null) => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  apiToken: null,
  phone: null,
  expiresAt: null,
  hydrated: false,

  setSession: (apiToken, expiresAt, phone) => {
    const exp = new Date(expiresAt).getTime();
    set({ apiToken, expiresAt: exp, phone });
    // fire-and-forget save
    AsyncStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ apiToken, phone, expiresAt: exp } satisfies PersistedSession),
    ).catch(() => {});
  },

  clear: () => {
    set({ apiToken: null, phone: null, expiresAt: null });
    AsyncStorage.removeItem(STORAGE_KEY).catch(() => {});
  },

  isAuthenticated: () => {
    const { apiToken, expiresAt } = get();
    if (!apiToken || !expiresAt) return false;
    return Date.now() < expiresAt;
  },

  _markHydrated: (session) => {
    if (session && Date.now() < session.expiresAt) {
      set({
        apiToken: session.apiToken,
        phone: session.phone,
        expiresAt: session.expiresAt,
        hydrated: true,
      });
    } else {
      set({ hydrated: true });
    }
  },
}));

/**
 * Read AsyncStorage and seed the auth store. Resolves once the store reflects
 * the persisted state (or marks itself hydrated with no session).
 */
export async function hydrateAuth(): Promise<void> {
  try {
    const raw = await AsyncStorage.getItem(STORAGE_KEY);
    if (!raw) {
      useAuthStore.getState()._markHydrated(null);
      return;
    }
    const session = JSON.parse(raw) as PersistedSession;
    useAuthStore.getState()._markHydrated(session);
  } catch {
    // Corrupt blob — wipe and continue as unauthenticated.
    await AsyncStorage.removeItem(STORAGE_KEY).catch(() => {});
    useAuthStore.getState()._markHydrated(null);
  }
}
