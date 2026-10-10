import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, within, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import i18n from './i18n/config';
import { SnackbarProvider } from './context/SnackbarContext';
import BackupsPage from './BackupsPage';
import * as backupsApi from './api/backups';

vi.mock('./api/backups', () => ({
  getBackups: vi.fn(),
  createBackup: vi.fn(),
  downloadBackup: vi.fn(),
  deleteBackup: vi.fn(),
}));
vi.mock('./auth', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./auth')>()),
  isAuthenticated: () => true,
  isAdmin: () => true,
}));

const first = { name: 'meerkat-backup-20300102-030405.tar.gz', size: 2048, created_at: '2030-01-02T03:04:05Z' };
const second = {
  name: 'meerkat-backup-20300101-030405.tar.gz',
  size: 3 * 1024 * 1024,
  created_at: '2030-01-01T03:04:05Z',
};
const list = (backups: backupsApi.Backup[], enabled = true): backupsApi.BackupList => ({
  backups,
  enabled,
  time: '02:30',
  retention_count: 7,
  directory: '/app/data/backups',
});

function renderPage() {
  return render(
    <MemoryRouter>
      <SnackbarProvider>
        <BackupsPage />
      </SnackbarProvider>
    </MemoryRouter>,
  );
}

describe('BackupsPage', () => {
  afterEach(cleanup);
  beforeEach(async () => {
    vi.resetAllMocks();
    await i18n.changeLanguage('en');
  });

  test('lists backups, the schedule, and warns that all users are included', async () => {
    vi.mocked(backupsApi.getBackups).mockResolvedValue(list([first, second]));
    renderPage();

    expect(await screen.findByText(/meerkat-backup-20300102-030405\.tar\.gz · 2\.0 KB/)).toBeInTheDocument();
    expect(screen.getByText(/meerkat-backup-20300101-030405\.tar\.gz · 3\.0 MB/)).toBeInTheDocument();
    expect(screen.getByText(/data of ALL users/)).toBeInTheDocument();
    expect(screen.getByText(/every day at 02:30; the 7 most recent are kept/)).toBeInTheDocument();
    expect(screen.getByText(/\/app\/data\/backups/)).toBeInTheDocument();
  });

  test('shows an empty state and a note when automatic backups are off', async () => {
    vi.mocked(backupsApi.getBackups).mockResolvedValue(list([], false));
    renderPage();

    expect(await screen.findByText('There are no backups yet.')).toBeInTheDocument();
    expect(screen.getByText(/Automatic backups are turned off/)).toBeInTheDocument();
  });

  test('creates a backup and reloads the list', async () => {
    vi.mocked(backupsApi.getBackups)
      .mockResolvedValueOnce(list([]))
      .mockResolvedValueOnce(list([first]));
    vi.mocked(backupsApi.createBackup).mockResolvedValue(first);
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: 'Create backup now' }));

    await waitFor(() => expect(backupsApi.createBackup).toHaveBeenCalled());
    expect(await screen.findByText(/meerkat-backup-20300102-030405\.tar\.gz/)).toBeInTheDocument();
  });

  test('downloads a backup by name', async () => {
    vi.mocked(backupsApi.getBackups).mockResolvedValue(list([first]));
    vi.mocked(backupsApi.downloadBackup).mockResolvedValue();
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: `Download ${first.name}` }));

    await waitFor(() => expect(backupsApi.downloadBackup).toHaveBeenCalledWith(first.name));
  });

  test('deleting asks for confirmation first', async () => {
    vi.mocked(backupsApi.getBackups)
      .mockResolvedValueOnce(list([first]))
      .mockResolvedValueOnce(list([]));
    vi.mocked(backupsApi.deleteBackup).mockResolvedValue();
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: `Delete ${first.name}` }));
    const dialog = await screen.findByRole('dialog');
    expect(backupsApi.deleteBackup).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }));
    await waitFor(() => expect(backupsApi.deleteBackup).toHaveBeenCalledWith(first.name));
    expect(await screen.findByText('There are no backups yet.')).toBeInTheDocument();
  });

  test('cancelling the delete dialog keeps the backup', async () => {
    vi.mocked(backupsApi.getBackups).mockResolvedValue(list([first]));
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: `Delete ${first.name}` }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(backupsApi.deleteBackup).not.toHaveBeenCalled();
  });

  test('shows an error when the list cannot be loaded', async () => {
    vi.mocked(backupsApi.getBackups).mockRejectedValue(new Error('boom'));
    renderPage();
    expect(await screen.findAllByRole('alert')).not.toHaveLength(0);
  });
});
