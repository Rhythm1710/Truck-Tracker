import { getAuth } from '@react-native-firebase/auth';

/**
 * Ask Firebase to send a phone-auth SMS.
 *
 * Use the namespaced Auth method. The modular `signInWithPhoneNumber`
 * helper only forwards (auth, phone, appVerifier), so a forceResend flag
 * passed after that is dropped and Android never receives it. On Android
 * the native method is signInWithPhoneNumber(phone, forceResend). iOS's
 * native method has no force-resend argument; calling this again still
 * starts a new verification.
 */
export function requestSmsCode(e164: string, forceResend: boolean) {
  return getAuth().signInWithPhoneNumber(e164, forceResend);
}
