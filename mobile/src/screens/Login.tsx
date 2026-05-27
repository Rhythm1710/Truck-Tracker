/**
 * Login screen — Phone + OTP via Firebase Phone Auth.
 *
 * Two states in one screen:
 *   1. Enter phone → press "Get OTP" → Firebase sends SMS, returns confirmation.
 *   2. Enter 6-digit OTP → confirm → exchange Firebase ID token for our API token.
 */
import React, { useState } from 'react';
import { View, Text, TextInput, Pressable, StyleSheet, KeyboardAvoidingView, Platform, ActivityIndicator, Alert } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import auth, { FirebaseAuthTypes } from '@react-native-firebase/auth';

import { colors, fontSize, fontWeight, radius, spacing } from '../theme';
import { exchangeFirebaseToken } from '../lib/api';
import { useAuthStore } from '../store/auth';

export function LoginScreen() {
  const setSession = useAuthStore((s) => s.setSession);

  const [phone, setPhone] = useState('');
  const [confirm, setConfirm] = useState<FirebaseAuthTypes.ConfirmationResult | null>(null);
  const [otp, setOtp] = useState('');
  const [busy, setBusy] = useState(false);

  async function handleGetOTP() {
    if (phone.replace(/\D/g, '').length !== 10) {
      Alert.alert('Invalid number', 'Please enter a 10-digit mobile number.');
      return;
    }
    try {
      setBusy(true);
      const c = await auth().signInWithPhoneNumber('+91' + phone.replace(/\D/g, ''));
      setConfirm(c);
    } catch (err) {
      Alert.alert('Could not send OTP', String((err as Error).message ?? err));
    } finally {
      setBusy(false);
    }
  }

  async function handleVerify() {
    if (!confirm) return;
    if (otp.length !== 6) {
      Alert.alert('Invalid code', 'Please enter the 6-digit OTP.');
      return;
    }
    try {
      setBusy(true);
      const cred = await confirm.confirm(otp);
      if (!cred) throw new Error('Could not confirm OTP');
      const idToken = await cred.user.getIdToken();
      const session = await exchangeFirebaseToken(idToken);
      setSession(session.apiToken, session.expiresAt, cred.user.phoneNumber ?? '');
    } catch (err) {
      Alert.alert('Verification failed', String((err as Error).message ?? err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <SafeAreaView style={styles.safe} edges={['top', 'bottom']}>
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={{ flex: 1 }}>
        <View style={styles.content}>
          <View style={styles.iconBox}>
            <Text style={styles.iconEmoji}>🚚</Text>
          </View>

          <Text style={styles.title}>Truck Tracker</Text>
          <Text style={styles.subtitle}>
            {confirm ? 'Enter the OTP we just sent' : 'Sign in to see your trucks'}
          </Text>

          <View style={{ height: spacing['4xl'] + spacing.lg }} />

          {!confirm ? (
            <>
              <Text style={styles.label}>Mobile Number</Text>
              <View style={styles.input}>
                <Text style={styles.countryCode}>+91</Text>
                <View style={styles.inputDivider} />
                <TextInput
                  value={phone}
                  onChangeText={setPhone}
                  placeholder="98765 43210"
                  placeholderTextColor={colors.textSecondary}
                  keyboardType="phone-pad"
                  maxLength={12}
                  style={styles.inputField}
                />
              </View>

              <Pressable onPress={handleGetOTP} disabled={busy} style={({ pressed }) => [styles.btn, pressed && styles.btnPressed]}>
                {busy ? <ActivityIndicator color={colors.primaryText} /> : <Text style={styles.btnText}>Get OTP</Text>}
              </Pressable>
            </>
          ) : (
            <>
              <Text style={styles.label}>OTP</Text>
              <View style={styles.input}>
                <TextInput
                  value={otp}
                  onChangeText={(t) => setOtp(t.replace(/\D/g, ''))}
                  placeholder="6-digit code"
                  placeholderTextColor={colors.textSecondary}
                  keyboardType="number-pad"
                  maxLength={6}
                  style={[styles.inputField, { paddingLeft: 0, letterSpacing: 4 }]}
                />
              </View>

              <Pressable onPress={handleVerify} disabled={busy} style={({ pressed }) => [styles.btn, pressed && styles.btnPressed]}>
                {busy ? <ActivityIndicator color={colors.primaryText} /> : <Text style={styles.btnText}>Verify & Continue</Text>}
              </Pressable>

              <Pressable onPress={() => { setConfirm(null); setOtp(''); }} style={styles.linkBtn}>
                <Text style={styles.linkText}>Change number</Text>
              </Pressable>
            </>
          )}

          <View style={{ flex: 1 }} />
          <Text style={styles.help}>Need help? Call +91 98XXX XXXXX</Text>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
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
  btn: {
    marginTop: spacing['3xl'], backgroundColor: colors.primary, borderRadius: radius.lg,
    paddingVertical: spacing.xl + 2, alignItems: 'center',
  },
  btnPressed: { opacity: 0.9 },
  btnText: { color: colors.primaryText, fontSize: fontSize.title, fontWeight: fontWeight.bold },
  linkBtn: { alignItems: 'center', paddingVertical: spacing.lg },
  linkText: { fontSize: fontSize.body, color: colors.textSecondary },
  help: { textAlign: 'center', color: colors.textSecondary, fontSize: fontSize.bodyLg, marginTop: spacing.lg },
});
