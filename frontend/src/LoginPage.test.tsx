import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import i18n from './i18n/config';
import LoginPage from './LoginPage';
import * as auth from './auth';

vi.mock('./auth', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./auth')>()),
  loginUser: vi.fn(),
  isAuthenticated: () => true,
}));
vi.mock('./hooks/useOIDCConfig', () => ({ useOIDCConfig: () => ({ enabled: false, provider_name: '' }) }));

const renderIt = () =>
  render(
    <MemoryRouter>
      <LoginPage />
    </MemoryRouter>,
  );

function fillPassword() {
  fireEvent.change(screen.getByLabelText(/Username or Email/), { target: { value: 'alice' } });
  fireEvent.change(screen.getByLabelText(/^Password/), { target: { value: 'pw-12345678' } });
  fireEvent.click(screen.getByRole('button', { name: 'Login' }));
}

describe('LoginPage two-factor step', () => {
  beforeEach(async () => {
    vi.resetAllMocks();
    await i18n.changeLanguage('en');
  });

  test('asks for the code after the password and sends both', async () => {
    vi.mocked(auth.loginUser)
      .mockResolvedValueOnce({ totpRequired: true })
      .mockResolvedValueOnce({ language: 'en', date_format: 'eu' });
    renderIt();
    fillPassword();
    expect(await screen.findByRole('heading', { name: 'Two-factor authentication' })).toBeInTheDocument();
    expect(auth.loginUser).toHaveBeenCalledWith('alice', 'pw-12345678', undefined);

    fireEvent.change(screen.getByLabelText(/^Authentication code/), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify' }));
    await waitFor(() => expect(auth.loginUser).toHaveBeenLastCalledWith('alice', 'pw-12345678', { code: '123456' }));
  });

  test('a wrong code shows a message and a recovery code can be used instead', async () => {
    vi.mocked(auth.loginUser)
      .mockResolvedValueOnce({ totpRequired: true })
      .mockRejectedValueOnce(new auth.InvalidSecondFactorError())
      .mockResolvedValueOnce({});
    renderIt();
    fillPassword();
    fireEvent.change(await screen.findByLabelText(/^Authentication code/), { target: { value: '000000' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify' }));
    expect(await screen.findByText(/That code is not valid/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Use a recovery code' }));
    fireEvent.change(screen.getByLabelText(/^Recovery code/), { target: { value: 'AAAAA-BBBBB' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify' }));
    await waitFor(() =>
      expect(auth.loginUser).toHaveBeenLastCalledWith('alice', 'pw-12345678', { recoveryCode: 'AAAAA-BBBBB' }),
    );
  });

  test('a locked account says so', async () => {
    vi.mocked(auth.loginUser)
      .mockResolvedValueOnce({ totpRequired: true })
      .mockRejectedValueOnce(new auth.LoginLockedError());
    renderIt();
    fillPassword();
    fireEvent.change(await screen.findByLabelText(/^Authentication code/), { target: { value: '000000' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify' }));
    expect(await screen.findByText(/Too many failed attempts/)).toBeInTheDocument();
  });
});
