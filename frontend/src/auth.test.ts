import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import { loginUser, InvalidSecondFactorError, LoginLockedError } from './auth';

const json = (status: number, body: unknown) =>
  Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }));

describe('loginUser', () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  test('reports that a code is needed and does not fetch the user', async () => {
    fetchMock.mockReturnValueOnce(json(200, { totp_required: true }));
    expect(await loginUser('a', 'b')).toEqual({ totpRequired: true });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  test('sends the code or the recovery code', async () => {
    fetchMock.mockReturnValueOnce(json(401, { error: { code: 'INVALID_TOTP_CODE' } }));
    await expect(loginUser('a', 'b', { code: '123456' })).rejects.toBeInstanceOf(InvalidSecondFactorError);
    expect(JSON.parse(fetchMock.mock.calls[0]?.[1].body)).toEqual({
      identifier: 'a',
      password: 'b',
      totp_code: '123456',
    });

    fetchMock.mockReturnValueOnce(json(401, { error: { code: 'INVALID_CREDENTIALS' } }));
    await expect(loginUser('a', 'b', { recoveryCode: 'X' })).rejects.toThrow('Login failed');
    expect(JSON.parse(fetchMock.mock.calls[1]?.[1].body)).toEqual({
      identifier: 'a',
      password: 'b',
      recovery_code: 'X',
    });
  });

  test('a lockout is its own error', async () => {
    fetchMock.mockReturnValueOnce(json(429, { error: 'Account temporarily locked' }));
    await expect(loginUser('a', 'b')).rejects.toBeInstanceOf(LoginLockedError);
  });
});
