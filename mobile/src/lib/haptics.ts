/**
 * Tiny haptic wrapper using React Native's built-in `Vibration` API.
 *
 * Why not `react-native-haptic-feedback`?
 *   - Built-in API needs no extra native linking.
 *   - For our purposes (subtle confirmation buzz on tap / success) the
 *     short pulse is indistinguishable from "light impact" haptics.
 *   - We can swap to the proper haptics lib later without changing callers.
 */
import { Vibration } from 'react-native';

export const haptics = {
  /** Quick tap feedback (10ms). Use on button presses. */
  tap() { Vibration.vibrate(10); },
  /** Success feedback — slightly longer. Use after OTP success, refresh. */
  success() { Vibration.vibrate(40); },
  /** Error feedback — double pulse. Use on validation failure. */
  error() { Vibration.vibrate([0, 30, 60, 30]); },
};
