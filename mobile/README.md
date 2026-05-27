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
    StatusPill.tsx            # Running / Idle / Stopped / Offline
    TruckCard.tsx             # the home-screen truck row
  hooks/
    useLivePositions.ts       # WebSocket subscriber → react-query cache
  lib/
    api.ts                    # typed REST client
    types.ts                  # shared API types (mirror backend models)
  store/
    auth.ts                   # Zustand store for apiToken + phone
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

# 5. Env
echo "EXPO_PUBLIC_API_BASE=http://10.0.2.2:8080" > .env
# 10.0.2.2 = host loopback on Android emulator
# Use http://localhost:8080 on iOS simulator
# Use http://<your-mac-LAN-ip>:8080 on a physical device

# 6. Run
npm run android    # or
npm run ios
```

## Auth flow (mirrors backend)

1. User enters phone, presses Get OTP.
2. `@react-native-firebase/auth` sends SMS via Firebase.
3. User enters 6-digit code → Firebase returns a Firebase ID token.
4. App POSTs the ID token to `/v1/auth/exchange`; backend verifies it,
   returns an `apiToken` (HMAC-signed, 24h TTL).
5. App stores `apiToken` in Zustand and sends it as `Authorization: Bearer ...`
   on every REST call, and as `?token=...` on the WebSocket URL.

## Live updates

`useLivePositions()` opens a WebSocket to `/v1/live`. Every incoming
`{ type: "position" }` message is patched into the `['trucks']` and
`['truck', id]` react-query caches — components re-render automatically.

If the connection drops, it reconnects with exponential backoff capped at 30s.

## What's stubbed

- **AsyncStorage persistence** in `src/store/auth.ts`. Currently the session
  is in-memory only — the user has to log in again after killing the app.
  Add `@react-native-async-storage/async-storage` and rehydrate on launch.
- **Mappls MapView** falls back to a soft-green `<View />` if the SDK isn't
  installed/configured. Configure keys and remove the try/catch.
- **No history screen yet** — design only has 3 screens for v1.
