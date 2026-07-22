/**
 * Login screen — Phone + OTP via Firebase Phone Auth.
 *
 * UX choices (per OPTIMIZATION-PLAN.md, sprint B):
 *   - Phone input auto-focuses on mount.
 *   - Phone formatted as user types: "98765 43210".
 *   - OTP entry uses a 6-cell visual with iOS QuickType / Android SMS Retriever autofill.
 *   - OTP auto-submits when the 6th digit lands.
 *   - Inline error display (instead of modal Alert) — accessible to older users.
 *   - Haptic feedback on key transitions.
 */
import React, { useCallback, useRef, useState } from 'react';
import {
  View,
  Text,
  TextInput,
  Pressable,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { getAuth, signInWithPhoneNumber, getIdToken, FirebaseAuthTypes } from '@react-native-firebase/auth';

import { colors, fontSize, fontWeight, radius, spacing } from '../theme';
import { exchangeFirebaseToken } from '../lib/api';
import { useAuthStore } from '../store/auth';
import { OTPInput } from '../components/OTPInput';
import { haptics } from '../lib/haptics';
import { digitsOnly, formatPhone, isValidPhone, toE164India } from '../lib/phone';

export function LoginScreen() {
  const setSession = useAuthStore((s) => s.setSession);

  const [phone, setPhone] = useState('');
  const [confirm, setConfirm] = useState<FirebaseAuthTypes.ConfirmationResult | null>(null);
  const [otp, setOtp] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const phoneInputRef = useRef<TextInput>(null);

  async function handleGetOTP() {
    setError(null);
    if (!isValidPhone(phone)) {
      setError('Please enter a 10-digit mobile number.');
      haptics.error();
      return;
    }
    try {
      setBusy(true);
      haptics.tap();
      const c = await signInWithPhoneNumber(getAuth(), toE164India(phone));
      setConfirm(c);
      haptics.success();
    } catch (err: any) {
      // eslint-disable-next-line no-console
      console.error('[OTP Error]', err?.code, err?.message);
      setError(humanizeAuthError(err) || 'Could not send OTP. Please try again.');
      haptics.error();
    } finally {
      setBusy(false);
    }
  }

  const handleVerify = useCallback(
    async (code?: string) => {
      const value = code ?? otp;
      if (!confirm) return;
      if (value.length !== 6) {
        setError('Please enter the 6-digit OTP.');
        haptics.error();
        return;
      }
      setError(null);
      try {
        setBusy(true);
        const cred = await confirm.confirm(value);
        if (!cred) throw new Error('Could not confirm OTP');
        const idToken = await getIdToken(cred.user);
        const session = await exchangeFirebaseToken(idToken);
        haptics.success();
        setSession(session.apiToken, session.expiresAt, cred.user.phoneNumber ?? '');
      } catch (err: any) {
        setError(humanizeAuthError(err) || 'That code didn\'t work. Please try again.');
        haptics.error();
      } finally {
        setBusy(false);
      }
    },
    [otp, confirm, setSession],
  );

  return (
    <SafeAreaView style={styles.safe} edges={['top', 'bottom']}>
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        style={{ flex: 1 }}
      >
        <View style={styles.content}>
          <View style={styles.iconBox} accessibilityRole="image" accessibilityLabel="Truck Tracker">
            <Text style={styles.iconEmoji} maxFontSizeMultiplier={1}>🚚</Text>
          </View>

          <Text style={styles.title} maxFontSizeMultiplier={1.3}>Truck Tracker</Text>
          <Text style={styles.subtitle} maxFontSizeMultiplier={1.4}>
            {confirm ? 'Enter the OTP we just sent' : 'Sign in to see your trucks'}
          </Text>

          <View style={{ height: spacing['4xl'] + spacing.lg }} />

          {!confirm ? (
            <>
              <Text style={styles.label} maxFontSizeMultiplier={1.4}>Mobile Number</Text>
              <View style={styles.input}>
                <Text style={styles.countryCode} maxFontSizeMultiplier={1.3}>+91</Text>
                <View style={styles.inputDivider} />
                <TextInput
                  ref={phoneInputRef}
                  value={formatPhone(phone)}
                  onChangeText={(t) => {
                    setError(null);
                    setPhone(digitsOnly(t).slice(0, 10));
                  }}
                  placeholder="98765 43210"
                  placeholderTextColor={colors.textSecondary}
                  keyboardType="phone-pad"
                  maxLength={11} // 10 digits + 1 space
                  autoFocus
                  autoComplete="tel"
                  textContentType="telephoneNumber"
                  style={styles.inputField}
                  accessibilityLabel="Mobile number, 10 digits"
                />
              </View>

              {error && <Text style={styles.error} accessibilityRole="alert">{error}</Text>}

              <Pressable
                onPress={handleGetOTP}
                disabled={busy}
                accessibilityRole="button"
                accessibilityLabel="Get OTP"
                accessibilityState={{ disabled: busy }}
                style={({ pressed }) => [styles.btn, pressed && styles.btnPressed, busy && styles.btnDisabled]}
                android_ripple={{ color: '#ffffff33' }}
              >
                {busy ? (
                  <ActivityIndicator color={colors.primaryText} />
                ) : (
                  <Text style={styles.btnText} maxFontSizeMultiplier={1.3}>Get OTP</Text>
                )}
              </Pressable>
            </>
          ) : (
            <>
              <Text style={styles.label} maxFontSizeMultiplier={1.4}>OTP</Text>
              <OTPInput
                value={otp}
                onChange={(v) => { setError(null); setOtp(v); }}
                onComplete={(code) => handleVerify(code)}
                autoFocus
              />

              {error && <Text style={styles.error} accessibilityRole="alert">{error}</Text>}

              <Pressable
                onPress={() => handleVerify()}
                disabled={busy}
                accessibilityRole="button"
                accessibilityLabel="Verify and continue"
                accessibilityState={{ disabled: busy }}
                style={({ pressed }) => [styles.btn, pressed && styles.btnPressed, busy && styles.btnDisabled]}
                android_ripple={{ color: '#ffffff33' }}
              >
                {busy ? (
                  <ActivityIndicator color={colors.primaryText} />
                ) : (
                  <Text style={styles.btnText} maxFontSizeMultiplier={1.3}>Verify & Continue</Text>
                )}
              </Pressable>

              <Pressable
                onPress={() => { setConfirm(null); setOtp(''); setError(null); }}
                style={styles.linkBtn}
                hitSlop={12}
                accessibilityRole="button"
                accessibilityLabel="Change phone number"
              >
                <Text style={styles.linkText} maxFontSizeMultiplier={1.4}>Change number</Text>
              </Pressable>
            </>
          )}

          <View style={{ flex: 1 }} />
          <Text style={styles.help} maxFontSizeMultiplier={1.4}>
            Need help? Call +91 98XXX XXXXX
          </Text>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

// Convert a Firebase auth error into a one-sentence, senior-friendly message.
function humanizeAuthError(err: any): string | null {
  const code = err?.code as string | undefined;
  switch (code) {
    case 'auth/invalid-phone-number':
      return 'That phone number doesn\'t look right.';
    case 'auth/too-many-requests':
      return 'Too many attempts. Please try again in a few minutes.';
    case 'auth/invalid-verification-code':
      return 'That code is incorrect. Please check and try again.';
    case 'auth/code-expired':
      return 'That code has expired. Please request a new one.';
    case 'auth/network-request-failed':
      return 'Network problem. Check your internet connection.';
    default:
      return err?.message ?? null;
  }
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.surface },
  content: { flex: 1, paddingHorizontal: spacing['2xl'], paddingTop: spacing['6xl'], paddingBottom: spacing['3xl'] },
  iconBox: {
    width: 96, height: 96, backgroundColor: colors.primary, borderRadius: radius['2xl'],
    alignItems: 'center', justifyContent: 'center',
  },
  iconEmoji: { fontSize: 52 },
  title: { marginTop: spacing['3xl'], fontSize: fontSize.displayLg, fontWeight: fontWeight.bold, color: colors.textPrimary },
  subtitle: { fontSize: fontSize.subtitle, color: colors.textSecondary, marginTop: spacing.xs },
  label: { fontSize: fontSize.subtitle - 2, fontWeight: fontWeight.semibold, color: colors.textPrimary, marginBottom: spacing.md },
  input: {
    flexDirection: 'row', alignItems: 'center', backgroundColor: colors.surfaceAlt,
    borderRadius: radius.lg, borderWidth: 2, borderColor: colors.border, paddingHorizontal: spacing.xl, paddingVertical: spacing.lg + 4,
    gap: spacing.md,
  },
  countryCode: { fontSize: fontSize.titleLg, fontWeight: fontWeight.semibold, color: colors.textPrimary },
  inputDivider: { width: 2, height: 28, backgroundColor: colors.divider },
  inputField: { flex: 1, fontSize: fontSize.titleLg, fontWeight: fontWeight.medium, color: colors.textPrimary, padding: 0 },
  error: {
    marginTop: spacing.md,
    color: colors.statusStopped,
    fontSize: fontSize.body,
    fontWeight: fontWeight.medium,
  },
  btn: {
    marginTop: spacing['3xl'], backgroundColor: colors.primary, borderRadius: radius.lg,
    paddingVertical: spacing.xl + 2, alignItems: 'center', minHeight: 56,
    justifyContent: 'center',
  },
  btnPressed: { opacity: 0.9, transform: [{ scale: 0.98 }] },
  btnDisabled: { opacity: 0.7 },
  btnText: { color: colors.primaryText, fontSize: fontSize.title, fontWeight: fontWeight.bold },
  linkBtn: { alignItems: 'center', paddingVertical: spacing.lg, minHeight: 48 },
  linkText: { fontSize: fontSize.bodyLg, color: colors.textSecondary, fontWeight: fontWeight.medium },
  help: { textAlign: 'center', color: colors.textSecondary, fontSize: fontSize.bodyLg, marginTop: spacing.lg },
});
