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
import React, { useCallback, useEffect, useRef, useState } from 'react';
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
import { getIdToken, FirebaseAuthTypes } from '@react-native-firebase/auth';

import { colors, fontSize, fontWeight, radius, spacing } from '../theme';
import { exchangeFirebaseToken } from '../lib/api';
import { useAuthStore } from '../store/auth';
import { OTPInput } from '../components/OTPInput';
import { haptics } from '../lib/haptics';
import { digitsOnly, formatPhone, isValidPhone, toE164India } from '../lib/phone';
import { requestSmsCode } from '../lib/smsCode';

/** Seconds before another SMS can be requested. Not in the mockup. */
const RESEND_COOLDOWN_SEC = 30;

export function LoginScreen() {
  const setSession = useAuthStore((s) => s.setSession);

  const [phone, setPhone] = useState('');
  const [confirm, setConfirm] = useState<FirebaseAuthTypes.ConfirmationResult | null>(null);
  const [otp, setOtp] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resendIn, setResendIn] = useState(0);

  const phoneInputRef = useRef<TextInput>(null);

  const coolingDown = resendIn > 0;
  useEffect(() => {
    if (!coolingDown) return;
    const id = setInterval(() => {
      setResendIn(s => (s <= 1 ? 0 : s - 1));
    }, 1000);
    return () => clearInterval(id);
  }, [coolingDown]);

  async function sendCode(forceResend: boolean) {
    setError(null);
    if (!isValidPhone(phone)) {
      setError('Please enter a 10-digit mobile number.');
      haptics.error();
      return;
    }
    try {
      setBusy(true);
      haptics.tap();
      const c = await requestSmsCode(toE164India(phone), forceResend);
      setConfirm(c);
      setOtp('');
      setResendIn(RESEND_COOLDOWN_SEC);
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

  function handleGetOTP() {
    return sendCode(false);
  }

  function handleResend() {
    if (resendIn > 0 || busy) return;
    return sendCode(true);
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
          {confirm ? (
            <Text style={styles.sent} maxFontSizeMultiplier={1.4}>
              Sent to +91 {formatPhone(phone)}
            </Text>
          ) : null}

          <View style={{ height: confirm ? 36 : 40 }} />

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
                  <Text style={[styles.btnText, styles.btnTextLogin]} maxFontSizeMultiplier={1.3}>Get OTP</Text>
                )}
              </Pressable>
              <Text style={styles.hint} maxFontSizeMultiplier={1.4}>We'll send a 6-digit code by SMS.</Text>
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
                onPress={() => { setConfirm(null); setOtp(''); setError(null); setResendIn(0); }}
                style={({ pressed }) => [styles.linkBtn, pressed && styles.pressed]}
                hitSlop={12}
                accessibilityRole="button"
                accessibilityLabel="Change phone number"
              >
                <Text style={styles.linkText} maxFontSizeMultiplier={1.4}>Change number</Text>
              </Pressable>

              <Pressable
                onPress={handleResend}
                disabled={busy || resendIn > 0}
                accessibilityRole="button"
                accessibilityLabel={resendIn > 0 ? `Resend OTP in ${resendIn} seconds` : 'Resend OTP'}
                accessibilityState={{ disabled: busy || resendIn > 0 }}
                style={({ pressed }) => [styles.resendBtn, pressed && resendIn === 0 && styles.pressed]}
              >
                <Text style={[styles.resendText, (busy || resendIn > 0) && styles.resendTextWait]} maxFontSizeMultiplier={1.4}>
                  {resendIn > 0 ? `Resend in ${resendIn}s` : "Didn't get it? Resend OTP"}
                </Text>
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
  content: { flex: 1, paddingHorizontal: spacing['2xl'], paddingTop: 28, paddingBottom: spacing.lg },
  iconBox: {
    width: 96, height: 96, backgroundColor: colors.primary, borderRadius: radius['2xl'],
    alignItems: 'center', justifyContent: 'center',
    shadowColor: '#1E40AF',
    shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.22,
    shadowRadius: 20,
    elevation: 6,
  },
  iconEmoji: { fontSize: 52 },
  title: {
    marginTop: 28,
    fontSize: fontSize.displayLg,
    fontWeight: fontWeight.bold,
    letterSpacing: -1.2,
    lineHeight: 40,
    color: colors.textPrimary,
  },
  subtitle: { fontSize: fontSize.subtitle, lineHeight: 24, color: colors.textSecondary, marginTop: 6 },
  sent: { fontSize: fontSize.body, color: colors.textSecondary, marginTop: 6 },
  label: { fontSize: fontSize.bodyLg, fontWeight: fontWeight.semibold, color: colors.textPrimary, marginBottom: spacing.md },
  input: {
    flexDirection: 'row', alignItems: 'center', backgroundColor: colors.surfaceAlt,
    borderRadius: radius.lg, borderWidth: 2, borderColor: colors.border,
    paddingHorizontal: spacing.xl, minHeight: 64, gap: spacing.md,
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
  hint: { marginTop: spacing.md, fontSize: 14, lineHeight: 20, color: colors.textSecondary, textAlign: 'center' },
  btn: {
    marginTop: spacing['3xl'], backgroundColor: colors.primary, borderRadius: radius.lg,
    height: 56, alignItems: 'center', justifyContent: 'center',
  },
  btnPressed: { opacity: 0.9 },
  btnDisabled: { opacity: 0.7 },
  btnText: { color: '#fff', fontSize: fontSize.title, fontWeight: fontWeight.bold },
  btnTextLogin: { letterSpacing: -0.3 },
  linkBtn: { alignItems: 'center', justifyContent: 'center', height: 48, marginTop: spacing.sm },
  linkText: { fontSize: fontSize.bodyLg, color: colors.textSecondary, fontWeight: fontWeight.medium },
  resendBtn: { alignItems: 'center', justifyContent: 'center', height: 44 },
  resendText: { fontSize: fontSize.body, fontWeight: fontWeight.semibold, color: colors.primary },
  resendTextWait: { color: colors.textSecondary },
  pressed: { opacity: 0.7 },
  help: { textAlign: 'center', color: colors.textSecondary, fontSize: fontSize.bodyLg, marginTop: spacing.lg },
});
