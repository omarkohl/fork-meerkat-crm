import { apiFetch, API_BASE_URL, getAuthHeaders, parseErrorResponse } from './client';

export interface TwoFactorStatus {
  enabled: boolean;
  recovery_codes_remaining: number;
  /** False for accounts without a password (single sign-on only). */
  available: boolean;
}

export interface TwoFactorSetup {
  /** Base32 key, for typing into the app. */
  secret: string;
  /** otpauth:// URI, what the QR code encodes. */
  otpauth_uri: string;
}

/** Password plus one authenticator code or recovery code. */
export interface TwoFactorConfirmation {
  password: string;
  code?: string;
  recovery_code?: string;
}

async function call<T>(path: string, method: string, body?: unknown): Promise<T> {
  const response = await apiFetch(`${API_BASE_URL}${path}`, {
    method,
    headers: getAuthHeaders(),
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    throw await parseErrorResponse(response);
  }
  return response.json();
}

export const getTwoFactorStatus = () => call<TwoFactorStatus>('/users/me/2fa', 'GET');

/** Creates a pending secret; two-factor stays off until enableTwoFactor succeeds. */
export const setupTwoFactor = () => call<TwoFactorSetup>('/users/me/2fa/setup', 'POST');

/** Returns the recovery codes. They are shown once and never again. */
export async function enableTwoFactor(password: string, code: string): Promise<string[]> {
  const data = await call<{ recovery_codes: string[] }>('/users/me/2fa/enable', 'POST', { password, code });
  return data.recovery_codes;
}

export async function disableTwoFactor(confirmation: TwoFactorConfirmation): Promise<void> {
  await call('/users/me/2fa/disable', 'POST', confirmation);
}

export async function regenerateRecoveryCodes(confirmation: TwoFactorConfirmation): Promise<string[]> {
  const data = await call<{ recovery_codes: string[] }>('/users/me/2fa/recovery-codes', 'POST', confirmation);
  return data.recovery_codes;
}

/** Admin only: switches off a user's two-factor and signs the user out everywhere. */
export async function resetUserTwoFactor(userId: number): Promise<void> {
  await call(`/admin/users/${userId}/2fa/reset`, 'POST');
}
