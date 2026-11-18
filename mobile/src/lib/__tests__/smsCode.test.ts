import { beforeEach, describe, expect, it, jest } from '@jest/globals';
import * as auth from '@react-native-firebase/auth';

jest.mock('@react-native-firebase/auth', () => {
  const signInWithPhoneNumber = jest.fn(() => Promise.resolve({ verificationId: 'vid' }));
  return {
    getAuth: () => ({ signInWithPhoneNumber }),
    signInWithPhoneNumber,
  };
});

import { requestSmsCode } from '../smsCode';

const signInWithPhoneNumber = (auth as unknown as {
  signInWithPhoneNumber: jest.MockedFunction<(phone: string, forceResend: boolean) => Promise<unknown>>;
}).signInWithPhoneNumber;

beforeEach(() => {
  signInWithPhoneNumber.mockClear();
});

describe('requestSmsCode', () => {
  it('passes forceResend true when resending', async () => {
    await requestSmsCode('+919876543210', true);
    expect(signInWithPhoneNumber).toHaveBeenCalledTimes(1);
    expect(signInWithPhoneNumber).toHaveBeenCalledWith('+919876543210', true);
  });

  it('passes forceResend false for the first send', async () => {
    await requestSmsCode('+919876543210', false);
    expect(signInWithPhoneNumber).toHaveBeenCalledWith('+919876543210', false);
  });
});
