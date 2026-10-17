import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import { loginUser, logoutUser, isAuthenticated, isAdmin, InvalidSecondFactorError, LoginLockedError } from './auth';

const json = (status: number, body: unknown) =>
  Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }));

describe('loginUser', () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  test('a successful login remembers the user and returns their preferences', async () => {
    fetchMock
      .mockReturnValueOnce(json(200, { language: 'de', date_format: 'dmy' }))
      .mockReturnValueOnce(json(200, { ID: 7, Username: 'alice', is_admin: true }));

    expect(await loginUser('alice', 'pw')).toEqual({ language: 'de', date_format: 'dmy' });
    expect(isAuthenticated()).toBe(true);
    expect(isAdmin()).toBe(true);
  });

  test('a user whose profile cannot be loaded is not treated as logged in', async () => {
    fetchMock.mockReturnValueOnce(json(200, {})).mockReturnValueOnce(json(500, {}));

    await loginUser('alice', 'pw');
    expect(isAuthenticated()).toBe(false);
  });

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

describe('logoutUser', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  test('logs out locally even when the server cannot be reached', async () => {
    localStorage.setItem('user_info', JSON.stringify({ user_id: 1, username: 'alice', is_admin: true }));
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));

    await logoutUser();
    expect(isAuthenticated()).toBe(false);
    expect(isAdmin()).toBe(false);
  });
});

describe('isAdmin', () => {
  afterEach(() => {
    localStorage.clear();
  });

  test('corrupt cached user info grants no admin rights', () => {
    localStorage.setItem('user_info', '{not json');
    expect(isAdmin()).toBe(false);
  });
});
