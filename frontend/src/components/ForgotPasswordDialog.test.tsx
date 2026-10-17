import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import i18n from '../i18n/config';
import ForgotPasswordDialog from './ForgotPasswordDialog';
import * as authApi from '../api/auth';

vi.mock('../api/auth', () => ({
  requestPasswordReset: vi.fn(),
  confirmPasswordReset: vi.fn(),
}));

const type = (label: RegExp, value: string) => {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
};

const requestToken = async () => {
  vi.mocked(authApi.requestPasswordReset).mockResolvedValue('Check your inbox.');
  type(/^Email/, 'alice@example.com');
  fireEvent.click(screen.getByRole('button', { name: 'Send reset email' }));
  expect(await screen.findByText('Check your inbox.')).toBeInTheDocument();
};

const choosePassword = (password: string, confirmation: string) => {
  type(/^Reset token/, 'tok-123');
  type(/^New password/, password);
  type(/^Confirm new password/, confirmation);
  fireEvent.click(screen.getByRole('button', { name: 'Reset password' }));
};

describe('ForgotPasswordDialog', () => {
  beforeEach(async () => {
    vi.resetAllMocks();
    await i18n.changeLanguage('en');
  });

  test('resets the password with the emailed token and returns to login', async () => {
    const onClose = vi.fn();
    vi.mocked(authApi.confirmPasswordReset).mockResolvedValue('Password changed.');
    render(<ForgotPasswordDialog open onClose={onClose} />);

    await requestToken();
    expect(screen.getByText(/We sent a token to alice@example.com/)).toBeInTheDocument();
    choosePassword('n3w-Secret!', 'n3w-Secret!');

    expect(await screen.findByText('Password changed.')).toBeInTheDocument();
    expect(authApi.confirmPasswordReset).toHaveBeenCalledWith('tok-123', 'n3w-Secret!');
    fireEvent.click(screen.getByRole('button', { name: 'Return to login' }));
    expect(onClose).toHaveBeenCalled();
  });

  test('passwords that do not match are not sent', async () => {
    render(<ForgotPasswordDialog open onClose={vi.fn()} />);

    await requestToken();
    choosePassword('n3w-Secret!', 'typo');

    expect(await screen.findByText('Passwords do not match.')).toBeInTheDocument();
    expect(authApi.confirmPasswordReset).not.toHaveBeenCalled();
  });

  test('a rejected token shows the reason and keeps the form for another try', async () => {
    vi.mocked(authApi.confirmPasswordReset).mockRejectedValue(new Error('Token expired'));
    render(<ForgotPasswordDialog open onClose={vi.fn()} />);

    await requestToken();
    choosePassword('n3w-Secret!', 'n3w-Secret!');

    expect(await screen.findByText('Token expired')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reset password' })).toBeEnabled();
  });
});
