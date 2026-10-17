import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import i18n from '../i18n/config';
import { SnackbarProvider } from '../context/SnackbarContext';
import IcsFeedSettings from './IcsFeedSettings';
import * as icsApi from '../api/icsFeed';

vi.mock('../api/icsFeed', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/icsFeed')>();
  return { ...actual, getIcsFeed: vi.fn(), createIcsFeed: vi.fn(), revokeIcsFeed: vi.fn() };
});

const TOKEN = 'A'.repeat(43);

function renderIt() {
  return render(
    <SnackbarProvider>
      <IcsFeedSettings />
    </SnackbarProvider>,
  );
}

describe('IcsFeedSettings', () => {
  beforeEach(async () => {
    vi.resetAllMocks();
    await i18n.changeLanguage('en');
  });

  test('buildIcsFeedUrl returns an absolute URL ending in .ics', () => {
    const url = icsApi.buildIcsFeedUrl(TOKEN);
    expect(url).toMatch(/^https?:\/\//);
    expect(url).toContain(`/api/v1/ics/${TOKEN}.ics`);
  });

  test('generates a feed and shows the URL once', async () => {
    vi.mocked(icsApi.getIcsFeed).mockResolvedValue({ enabled: false, created_at: null, last_fetched_at: null });
    vi.mocked(icsApi.createIcsFeed).mockResolvedValue({
      enabled: true,
      created_at: '2030-01-01T00:00:00Z',
      last_fetched_at: null,
      token: TOKEN,
    });
    renderIt();

    fireEvent.click(await screen.findByRole('button', { name: 'Generate feed URL' }));

    const dialog = await screen.findByRole('dialog');
    const field = within(dialog).getByRole('textbox', { name: 'Feed URL' }) as HTMLInputElement;
    expect(field.value).toContain(`/ics/${TOKEN}.ics`);
    expect(within(dialog).getByRole('button', { name: 'Copy' })).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole('button', { name: /Done/ }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    // The URL is gone and only regenerate/revoke remain.
    expect(screen.queryByDisplayValue(new RegExp(TOKEN))).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Regenerate URL' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Revoke feed' })).toBeInTheDocument();
  });

  test('revoking asks for confirmation and then calls the API', async () => {
    vi.mocked(icsApi.getIcsFeed).mockResolvedValue({
      enabled: true,
      created_at: '2030-01-01T00:00:00Z',
      last_fetched_at: null,
    });
    vi.mocked(icsApi.revokeIcsFeed).mockResolvedValue();
    renderIt();

    fireEvent.click(await screen.findByRole('button', { name: 'Revoke feed' }));
    expect(icsApi.revokeIcsFeed).not.toHaveBeenCalled();
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Revoke' }));

    await waitFor(() => {
      expect(icsApi.revokeIcsFeed).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByRole('button', { name: 'Generate feed URL' })).toBeInTheDocument();
  });

  test('regenerating asks for confirmation and shows the new URL', async () => {
    vi.mocked(icsApi.getIcsFeed).mockResolvedValue({
      enabled: true,
      created_at: '2030-01-01T00:00:00Z',
      last_fetched_at: null,
    });
    vi.mocked(icsApi.createIcsFeed).mockResolvedValue({
      enabled: true,
      created_at: '2030-01-02T00:00:00Z',
      last_fetched_at: null,
      token: 'B'.repeat(43),
    });
    renderIt();

    fireEvent.click(await screen.findByRole('button', { name: 'Regenerate URL' }));
    expect(icsApi.createIcsFeed).not.toHaveBeenCalled();
    const confirm = await screen.findByRole('dialog');
    fireEvent.click(within(confirm).getByRole('button', { name: 'Regenerate' }));

    const field = (await screen.findByRole('textbox', { name: 'Feed URL' })) as HTMLInputElement;
    expect(field.value).toContain('B'.repeat(43));
  });

  test('shows when the feed was last fetched, or that it was not yet', async () => {
    vi.mocked(icsApi.getIcsFeed).mockResolvedValue({
      enabled: true,
      created_at: '2030-01-01T00:00:00Z',
      last_fetched_at: null,
    });
    const first = renderIt();
    expect(await screen.findByText('Not fetched yet.')).toBeInTheDocument();
    first.unmount();

    vi.mocked(icsApi.getIcsFeed).mockResolvedValue({
      enabled: true,
      created_at: '2030-01-01T00:00:00Z',
      last_fetched_at: '2030-02-03T04:05:00Z',
    });
    renderIt();
    const expected = new Date('2030-02-03T04:05:00Z').toLocaleString();
    expect(await screen.findByText(`Last fetched: ${expected}.`)).toBeInTheDocument();
    expect(screen.queryByText('Not fetched yet.')).not.toBeInTheDocument();
  });

  test('a new feed has not been fetched yet', async () => {
    vi.mocked(icsApi.getIcsFeed).mockResolvedValue({ enabled: false, created_at: null, last_fetched_at: null });
    vi.mocked(icsApi.createIcsFeed).mockResolvedValue({
      enabled: true,
      created_at: '2030-01-01T00:00:00Z',
      last_fetched_at: null,
      token: TOKEN,
    });
    renderIt();
    fireEvent.click(await screen.findByRole('button', { name: 'Generate feed URL' }));
    expect(await screen.findByText('Not fetched yet.')).toBeInTheDocument();
  });
});
