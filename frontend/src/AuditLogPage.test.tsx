import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import i18n from './i18n/config';
import AuditLogPage from './AuditLogPage';
import * as adminApi from './api/admin';
import * as auth from './auth';

vi.mock('./api/admin', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/admin')>()),
  getAuditLog: vi.fn(),
}));
vi.mock('./auth', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./auth')>()),
  isAdmin: vi.fn(),
}));

const entry = (over: Partial<adminApi.AuditEntry>): adminApi.AuditEntry => ({
  id: 1,
  created_at: '2030-01-01T10:00:00Z',
  user_id: 1,
  username: 'alice',
  event: 'login',
  ip: '203.0.113.7',
  user_agent: 'Firefox',
  detail: '',
  ...over,
});

const page = (entries: adminApi.AuditEntry[]): adminApi.AuditLogPage => ({
  entries,
  total: entries.length,
  page: 1,
  limit: 25,
  events: ['login', 'login_failed'],
  retention_days: 365,
});

const renderIt = () =>
  render(
    <MemoryRouter>
      <AuditLogPage />
    </MemoryRouter>,
  );

describe('AuditLogPage', () => {
  beforeEach(async () => {
    vi.resetAllMocks();
    vi.mocked(auth.isAdmin).mockReturnValue(true);
    await i18n.changeLanguage('en');
  });

  test('lists entries with translated event names and the retention', async () => {
    vi.mocked(adminApi.getAuditLog).mockResolvedValue(
      page([entry({}), entry({ id: 2, event: 'login_failed', username: '', detail: 'reason:unknown_account' })]),
    );
    renderIt();
    expect(await screen.findByText('Failed login')).toBeInTheDocument();
    expect(screen.getByText('Reason: Unknown account')).toBeInTheDocument();
    expect(screen.getAllByText('Firefox')).toHaveLength(2);
    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('Entries are kept for 365 days.')).toBeInTheDocument();
  });

  test('filters by user name and event', async () => {
    vi.mocked(adminApi.getAuditLog).mockResolvedValue(page([entry({})]));
    renderIt();
    await screen.findByText('alice');

    fireEvent.change(screen.getByLabelText('User'), { target: { value: 'ali' } });
    await waitFor(() =>
      expect(adminApi.getAuditLog).toHaveBeenLastCalledWith(expect.objectContaining({ username: 'ali', page: 1 })),
    );

    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Event' }));
    fireEvent.click(await screen.findByRole('option', { name: 'Failed login' }));
    await waitFor(() =>
      expect(adminApi.getAuditLog).toHaveBeenLastCalledWith(
        expect.objectContaining({ event: 'login_failed', username: 'ali' }),
      ),
    );
  });

  test('shows an empty state and does not load for non-admins', async () => {
    vi.mocked(adminApi.getAuditLog).mockResolvedValue(page([]));
    renderIt();
    expect(await screen.findByText('No entries.')).toBeInTheDocument();

    vi.mocked(adminApi.getAuditLog).mockClear();
    vi.mocked(auth.isAdmin).mockReturnValue(false);
    renderIt();
    await new Promise((r) => setTimeout(r, 300));
    expect(adminApi.getAuditLog).not.toHaveBeenCalled();
  });
});
