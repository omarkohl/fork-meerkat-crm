import { afterEach, describe, expect, test, vi } from 'vitest';
import { archiveContact, getCircles, unarchiveContact } from './contacts';

const respond = (body: unknown, status = 200) =>
  vi
    .fn()
    .mockResolvedValue(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }));

describe('contact lifecycle calls', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test('archiveContact posts to the archive endpoint and returns the contact', async () => {
    const fetchMock = respond({ ID: 7, firstname: 'Alice', archived: true });
    vi.stubGlobal('fetch', fetchMock);
    const contact = await archiveContact(7);
    expect(contact.archived).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toMatch(/\/contacts\/7\/archive$/);
    expect(init.method).toBe('POST');
  });

  test('unarchiveContact posts to the unarchive endpoint', async () => {
    const fetchMock = respond({ ID: 7, archived: false });
    vi.stubGlobal('fetch', fetchMock);
    expect((await unarchiveContact(7)).archived).toBe(false);
    expect((fetchMock.mock.calls[0] as [string])[0]).toMatch(/\/contacts\/7\/unarchive$/);
  });

  test('getCircles returns the names, and an empty list when the server sends null', async () => {
    vi.stubGlobal('fetch', respond(['Friends', 'Work']));
    expect(await getCircles()).toEqual(['Friends', 'Work']);
    vi.stubGlobal('fetch', respond(null));
    expect(await getCircles()).toEqual([]);
  });
});
