/**
 * 6-cell OTP input.
 *
 * Visually displays 6 boxes — familiar UX for the user's father (matches
 * banking apps). Internally a single hidden TextInput captures the value,
 * which lets iOS QuickType / Android SMS Retriever autofill the entire
 * code in one shot.
 *
 * Behavior:
 *   - Auto-focuses on mount.
 *   - Calls onComplete(code) when the 6th digit lands (autosubmit).
 *   - `value` is fully controlled — caller owns state.
 */
import React, { useEffect, useRef } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { colors, fontSize, fontWeight, radius, spacing } from '../theme';

interface Props {
  value: string;
  onChange: (next: string) => void;
  onComplete?: (code: string) => void;
  cellCount?: number;
  autoFocus?: boolean;
}

export function OTPInput({
  value,
  onChange,
  onComplete,
  cellCount = 6,
  autoFocus = true,
}: Props) {
  const inputRef = useRef<TextInput>(null);

  useEffect(() => {
    if (autoFocus) {
      // Small delay to coexist with screen mount animations.
      const t = setTimeout(() => inputRef.current?.focus(), 100);
      return () => clearTimeout(t);
    }
  }, [autoFocus]);

  // Autosubmit on last digit — fire once per "complete" event.
  const completedRef = useRef(false);
  useEffect(() => {
    if (value.length === cellCount && !completedRef.current) {
      completedRef.current = true;
      onComplete?.(value);
    } else if (value.length < cellCount) {
      completedRef.current = false;
    }
  }, [value, cellCount, onComplete]);

  const handleChange = (raw: string) => {
    const digits = raw.replace(/\D/g, '').slice(0, cellCount);
    onChange(digits);
  };

  const focused = value.length;

  return (
    <Pressable onPress={() => inputRef.current?.focus()} style={styles.row}>
      {Array.from({ length: cellCount }).map((_, i) => {
        const ch = value[i] ?? '';
        const isActive = i === focused;
        return (
          <View
            key={i}
            style={[
              styles.cell,
              ch && styles.cellFilled,
              isActive && styles.cellActive,
            ]}
            accessibilityRole="text"
            accessibilityLabel={`OTP digit ${i + 1}${ch ? `: ${ch}` : ' empty'}`}
          >
            <Text style={styles.digit} maxFontSizeMultiplier={1.3}>{ch}</Text>
          </View>
        );
      })}

      {/* The actual input is offscreen but focusable — it owns keyboard/autofill */}
      <TextInput
        ref={inputRef}
        value={value}
        onChangeText={handleChange}
        keyboardType="number-pad"
        textContentType="oneTimeCode" // iOS QuickType autofill from Messages
        autoComplete="sms-otp"        // Android SMS Retriever
        importantForAutofill="yes"
        maxLength={cellCount}
        style={styles.hidden}
        caretHidden
      />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', justifyContent: 'space-between', gap: spacing.sm },
  cell: {
    flex: 1,
    height: 60,
    borderRadius: radius.md,
    borderWidth: 2,
    borderColor: colors.divider,
    backgroundColor: colors.surfaceAlt,
    alignItems: 'center',
    justifyContent: 'center',
  },
  cellFilled: { borderColor: colors.textPrimary, backgroundColor: '#fff' },
  cellActive: {
    borderColor: colors.primary,
    backgroundColor: '#fff',
    shadowColor: '#1E40AF',
    shadowOffset: { width: 0, height: 0 },
    shadowOpacity: 0.15,
    shadowRadius: 3,
    elevation: 2,
  },
  digit: {
    fontSize: fontSize.display,
    fontWeight: fontWeight.bold,
    letterSpacing: -0.5,
    color: colors.textPrimary,
  },
  hidden: {
    position: 'absolute',
    width: 1,
    height: 1,
    opacity: 0,
    color: 'transparent',
  },
});
