import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import i18n from '../i18n/config';
import { SnackbarProvider } from '../context/SnackbarContext';
import SessionsSettings from './SessionsSettings';
import * as sessionsApi from '../api/sessions';

vi.mock('../api/sessions', () => ({
  getSessions: vi.fn(),
  revokeSession: vi.fn(),
  signOutEverywhere: vi.fn(),
}));
vi.mock('../auth', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../auth')>()),
  logoutUser: vi.fn().mockResolvedValue(undefined),
}));

const session = (over: Partial<sessionsApi.Session>): sessionsApi.Session => ({
  id: 'a'.repeat(32),
  user_agent: 'Firefox on Linux',
  ip: '203.0.113.7',
  created_at: '2030-01-01T10:00:00Z',
  last_seen_at: '2030-01-02T10:00:00Z',
  expires_at: '2030-02-01T10:00:00Z',
  current: false,
  ...over,
});

const renderIt = () =>
  render(
    <SnackbarProvider>
      <SessionsSettings />
    </SnackbarProvider>,
  );

describe('SessionsSettings', () => {
  beforeEach(async () => {
    vi.resetAllMocks();
    await i18n.changeLanguage('en');
  });

  test('lists sessions, marks the current one and offers sign out only for the others', async () => {
    vi.mocked(sessionsApi.getSessions).mockResolvedValue([
      session({ id: 'b'.repeat(32), user_agent: 'Safari on iPhone', current: true }),
      session({}),
    ]);
    renderIt();
    expect(await screen.findByText('Safari on iPhone')).toBeInTheDocument();
    expect(screen.getByText('This device')).toBeInTheDocument();
    expect(screen.getByText('Firefox on Linux')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign out Firefox on Linux' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Sign out Safari on iPhone' })).not.toBeInTheDocument();
  });

  test('signing out another session calls the API and reloads', async () => {
    vi.mocked(sessionsApi.getSessions)
      .mockResolvedValueOnce([
        session({ id: 'b'.repeat(32), user_agent: 'Safari on iPhone', current: true }),
        session({}),
      ])
      .mockResolvedValueOnce([session({ id: 'b'.repeat(32), user_agent: 'Safari on iPhone', current: true })]);
    vi.mocked(sessionsApi.revokeSession).mockResolvedValue();
    renderIt();
    fireEvent.click(await screen.findByRole('button', { name: 'Sign out Firefox on Linux' }));
    await waitFor(() => {
      expect(sessionsApi.revokeSession).toHaveBeenCalledWith('a'.repeat(32));
    });
    await waitFor(() => expect(screen.queryByText('Firefox on Linux')).not.toBeInTheDocument());
  });

  test('sign out everywhere asks first', async () => {
    vi.mocked(sessionsApi.getSessions).mockResolvedValue([session({ current: true })]);
    vi.mocked(sessionsApi.signOutEverywhere).mockResolvedValue();
    // jsdom cannot navigate; only the call is checked
    renderIt();
    fireEvent.click(await screen.findByRole('button', { name: 'Sign out everywhere' }));
    expect(sessionsApi.signOutEverywhere).not.toHaveBeenCalled();
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Sign out everywhere' }));
    await waitFor(() => {
      expect(sessionsApi.signOutEverywhere).toHaveBeenCalled();
    });
  });

  test('shows an error when loading fails', async () => {
    vi.mocked(sessionsApi.getSessions).mockRejectedValue(new Error('boom'));
    renderIt();
    expect(await screen.findByText('boom')).toBeInTheDocument();
  });
});
