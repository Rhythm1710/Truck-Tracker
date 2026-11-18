# Truck Tracker — Mobile App

React Native + TypeScript app that ships to both iOS and Android. Talks to the
Go backend over REST + WebSocket. Renders maps via Mappls SDK.

## Layout

```
App.tsx                       # navigation + providers
src/
  screens/
    Login.tsx                 # Phone + OTP via Firebase
    Home.tsx                  # truck cards + live status pills
    TruckDetail.tsx           # Mappls map + bottom info sheet
  components/
    OTPInput.tsx              # 6-cell OTP entry
    StatusPill.tsx            # Running / Idle / Stopped / Offline
    TruckCard.tsx             # the home-screen truck row
  hooks/
    useLivePositions.ts       # WebSocket subscriber → react-query cache
  lib/
    api.ts                    # typed REST client
    phone.ts                  # Indian 10-digit phone formatting
    types.ts                  # shared API types (mirror backend models)
    mappls.ts                 # Mappls SDK init, with a safe fallback
    haptics.ts
    LiveProvider.tsx          # one WebSocket for the whole app
  store/
    auth.ts                   # Zustand session, persisted with AsyncStorage
  theme.ts                    # colors, spacing, type scale
```

## Setup

```bash
# 1. Node ≥ 18 + RN env
npx react-native@latest init  # if starting fresh, but we're cloning
npm install

# 2. iOS pods
npm run pods

# 3. Firebase
#    Place GoogleService-Info.plist in ios/
#    Place google-services.json in android/app/
#    Enable Phone sign-in in Firebase console.

# 4. Mappls
#    Sign up at https://apis.mappls.com (free tier).
#    Place keys in ios/Info.plist + android/.../strings.xml per Mappls docs.
#    Or stub `mappls-map-react-native` for dev (we already do safely).

# 5. Runtime config
#    src/lib/env.ts holds API_BASE and the Mappls keys.
#    Empty placeholders are in git. Put real Mappls keys there locally
#    and do not commit them.
#    API_BASE:
#      Android emulator   → http://10.0.2.2:8080
#      iOS simulator      → http://localhost:8080
#      Physical device    → http://<your-mac-LAN-ip>:8080

# 6. Android debug keystore (not in git)
#    The first `npm run android` runs the ensureDebugKeystore Gradle task,
#    which creates android/app/debug.keystore with keytool when it is missing
#    (store password android, alias androiddebugkey, key password android).
#    To create it by hand instead:
keytool -genkeypair -keystore android/app/debug.keystore -storetype PKCS12 \
  -storepass android -alias androiddebugkey -keypass android \
  -keyalg RSA -keysize 2048 -validity 10000 \
  -dname "CN=Android Debug,O=Android,C=US"

# 7. Run
npm run android    # or
npm run ios
```

## Auth flow (mirrors backend)

1. User enters phone, presses Get OTP.
2. `@react-native-firebase/auth` sends SMS via Firebase.
3. User enters 6-digit code → Firebase returns a Firebase ID token.
4. App POSTs the ID token to `/v1/auth/exchange`; backend verifies it,
   returns an `apiToken` (HMAC-signed, 24h TTL).
5. App stores `apiToken`, phone, and expiry in the Zustand auth store.
   `setSession` writes that blob to AsyncStorage (`@truck-tracker/auth/v1`).
   `hydrateAuth()` runs before the navigator mounts, so a still-valid session
   opens Home instead of Login. REST calls send `Authorization: Bearer ...`,
   and the WebSocket URL sends `?token=...`.

## Live updates

`LiveProvider` opens one WebSocket to `/v1/live` for the signed-in app.
Every incoming `{ type: "position" }` message is patched into the
`['trucks']` and `['truck', id]` react-query caches — components re-render
automatically. `useLivePositions()` is a thin wrapper around that provider.

If the connection drops, it reconnects with exponential backoff capped at 30s.
The socket closes while the app is in the background and reopens when it
becomes active.

## Session persistence

Implemented in `src/store/auth.ts` with `@react-native-async-storage/async-storage`.
The store itself is synchronous. `hydrateAuth()` reads AsyncStorage once at
launch and is awaited before the navigator renders, so a saved session does
not flash the Login screen. An expired token or a corrupt blob is dropped
and the user signs in again.

## What's not built yet

- **Mappls MapView** falls back to a soft-green placeholder if the SDK isn't
  installed or keys are missing.
- **No history screen yet** — v1 is Login, Home, and Truck Detail.
