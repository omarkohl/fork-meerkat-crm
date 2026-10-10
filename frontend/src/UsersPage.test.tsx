import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import i18n from './i18n/config';
import { SnackbarProvider } from './context/SnackbarContext';
import UsersPage from './UsersPage';
import * as adminApi from './api/admin';
import * as auth from './auth';

vi.mock('./api/admin', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/admin')>()),
  getUsers: vi.fn(),
  getSystemStatus: vi.fn(),
  updateUser: vi.fn(),
  deleteUser: vi.fn(),
  triggerReminders: vi.fn(),
}));
vi.mock('./auth', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./auth')>()),
  isAuthenticated: () => true,
  isAdmin: vi.fn(),
}));

const status = (over: Partial<adminApi.SystemStatus> = {}): adminApi.SystemStatus => ({
  private_urls_allowed: true,
  user_count: 3,
  private_url_warning: true,
  setting: 'BLOCK_PRIVATE_URLS',
  features: ['webhooks', 'caldav'],
  ...over,
});

function renderPage() {
  return render(
    <MemoryRouter>
      <SnackbarProvider>
        <UsersPage />
      </SnackbarProvider>
    </MemoryRouter>,
  );
}

describe('UsersPage private URL banner', () => {
  beforeEach(async () => {
    vi.resetAllMocks();
    localStorage.clear();
    vi.mocked(auth.isAdmin).mockReturnValue(true);
    vi.mocked(adminApi.getUsers).mockResolvedValue({ users: [], total: 0, page: 1, limit: 25 } as never);
    await i18n.changeLanguage('en');
  });

  test('shows the banner for an admin when the server warns', async () => {
    vi.mocked(adminApi.getSystemStatus).mockResolvedValue(status());
    renderPage();
    expect(await screen.findByText(/BLOCK_PRIVATE_URLS/)).toBeInTheDocument();
  });

  test('is hidden when there is no warning', async () => {
    vi.mocked(adminApi.getSystemStatus).mockResolvedValue(status({ private_url_warning: false }));
    renderPage();
    await waitFor(() => expect(adminApi.getSystemStatus).toHaveBeenCalled());
    await waitFor(() => expect(adminApi.getUsers).toHaveBeenCalled());
    expect(screen.queryByText(/BLOCK_PRIVATE_URLS/)).not.toBeInTheDocument();
  });

  test('is not requested for a non-admin', async () => {
    vi.mocked(auth.isAdmin).mockReturnValue(false);
    vi.mocked(adminApi.getSystemStatus).mockResolvedValue(status());
    renderPage();
    await new Promise((r) => setTimeout(r, 20));
    expect(adminApi.getSystemStatus).not.toHaveBeenCalled();
    expect(screen.queryByText(/BLOCK_PRIVATE_URLS/)).not.toBeInTheDocument();
  });

  test('dismiss hides it, stays hidden for the same situation and returns when it changes', async () => {
    vi.mocked(adminApi.getSystemStatus).mockResolvedValue(status());
    const first = renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /close/i }));
    expect(screen.queryByText(/BLOCK_PRIVATE_URLS/)).not.toBeInTheDocument();
    first.unmount();

    // Same situation: still dismissed
    renderPage().unmount();
    await waitFor(() => expect(adminApi.getSystemStatus).toHaveBeenCalledTimes(2));
    expect(screen.queryByText(/BLOCK_PRIVATE_URLS/)).not.toBeInTheDocument();

    // A feature more allows private URLs: shown again
    vi.mocked(adminApi.getSystemStatus).mockResolvedValue(status({ features: ['webhooks', 'caldav', 'monica'] }));
    renderPage();
    expect(await screen.findByText(/BLOCK_PRIVATE_URLS/)).toBeInTheDocument();
  });

  test('signature changes with user count bucket and features, not with order', () => {
    const base = adminApi.ssrfBannerSignature(status());
    expect(adminApi.ssrfBannerSignature(status({ features: ['caldav', 'webhooks'] }))).toBe(base);
    expect(adminApi.ssrfBannerSignature(status({ user_count: 8 }))).not.toBe(base);
    expect(adminApi.ssrfBannerSignature(status({ user_count: 5 }))).toBe(
      adminApi.ssrfBannerSignature(status({ user_count: 4 })),
    );
  });
});
