import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, within, fireEvent, cleanup } from '@testing-library/react';
import i18n from './i18n/config';
import { SnackbarProvider } from './context/SnackbarContext';
import { DateFormatProvider } from './DateFormatProvider';
import TrashPage from './TrashPage';
import * as trashApi from './api/trash';

vi.mock('./api/trash', () => ({
  getTrash: vi.fn(),
  restoreTrashedContact: vi.fn(),
  purgeTrashedContact: vi.fn(),
  emptyTrash: vi.fn(),
}));
vi.mock('./auth', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./auth')>()),
  isAuthenticated: () => true,
}));

const alice = {
  id: 1,
  firstname: 'Alice',
  lastname: 'Johnson',
  nickname: '',
  email: '',
  photo_thumbnail: '',
  deleted_at: '2030-01-01T10:00:00Z',
  purge_at: '2030-01-31T10:00:00Z',
};
const bob = { ...alice, id: 2, firstname: 'Bob', lastname: 'Smith' };

function renderPage() {
  return render(
    <SnackbarProvider>
      <DateFormatProvider>
        <TrashPage />
      </DateFormatProvider>
    </SnackbarProvider>,
  );
}

describe('TrashPage', () => {
  afterEach(cleanup);
  beforeEach(async () => {
    vi.resetAllMocks();
    await i18n.changeLanguage('en');
  });

  test('lists deleted contacts with the retention period', async () => {
    vi.mocked(trashApi.getTrash).mockResolvedValue({ contacts: [alice, bob], retention_days: 30 });
    renderPage();

    expect(await screen.findByText('Alice Johnson')).toBeInTheDocument();
    expect(screen.getByText('Bob Smith')).toBeInTheDocument();
    expect(screen.getByText(/stay here for 30 days/)).toBeInTheDocument();
    expect(screen.getAllByText(/Will be deleted permanently on .*2030/)).toHaveLength(2);
  });

  test('shows an empty state and disables Empty trash', async () => {
    vi.mocked(trashApi.getTrash).mockResolvedValue({ contacts: [], retention_days: 30 });
    renderPage();

    expect(await screen.findByText('The trash is empty.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Empty trash' })).toBeDisabled();
  });

  test('restores a contact and reloads the list', async () => {
    vi.mocked(trashApi.getTrash)
      .mockResolvedValueOnce({ contacts: [alice, bob], retention_days: 30 })
      .mockResolvedValueOnce({ contacts: [bob], retention_days: 30 });
    vi.mocked(trashApi.restoreTrashedContact).mockResolvedValue();
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: 'Restore Alice Johnson' }));

    await waitFor(() => expect(trashApi.restoreTrashedContact).toHaveBeenCalledWith(1));
    await waitFor(() => expect(screen.queryByText('Alice Johnson')).not.toBeInTheDocument());
    expect(screen.getByText('Bob Smith')).toBeInTheDocument();
  });

  test('purging asks for confirmation first', async () => {
    vi.mocked(trashApi.getTrash)
      .mockResolvedValueOnce({ contacts: [alice], retention_days: 30 })
      .mockResolvedValueOnce({ contacts: [], retention_days: 30 });
    vi.mocked(trashApi.purgeTrashedContact).mockResolvedValue();
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: 'Delete Alice Johnson permanently' }));
    const dialog = await screen.findByRole('dialog');
    expect(trashApi.purgeTrashedContact).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }));
    await waitFor(() => expect(trashApi.purgeTrashedContact).toHaveBeenCalledWith(1));
    expect(await screen.findByText('The trash is empty.')).toBeInTheDocument();
  });

  test('cancelling the purge dialog keeps the contact', async () => {
    vi.mocked(trashApi.getTrash).mockResolvedValue({ contacts: [alice], retention_days: 30 });
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: 'Delete Alice Johnson permanently' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trashApi.purgeTrashedContact).not.toHaveBeenCalled();
    expect(screen.getByText('Alice Johnson')).toBeInTheDocument();
  });

  test('empties the whole trash after confirmation', async () => {
    vi.mocked(trashApi.getTrash)
      .mockResolvedValueOnce({ contacts: [alice, bob], retention_days: 30 })
      .mockResolvedValueOnce({ contacts: [], retention_days: 30 });
    vi.mocked(trashApi.emptyTrash).mockResolvedValue(2);
    renderPage();

    await screen.findByText('Alice Johnson');
    fireEvent.click(screen.getByRole('button', { name: 'Empty trash' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText(/all 2 contacts/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }));

    await waitFor(() => expect(trashApi.emptyTrash).toHaveBeenCalled());
    expect(await screen.findByText('The trash is empty.')).toBeInTheDocument();
  });

  test('shows an error when the trash cannot be loaded', async () => {
    vi.mocked(trashApi.getTrash).mockRejectedValue(new Error('boom'));
    renderPage();
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });
});
