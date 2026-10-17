import { describe, test, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useTrash } from './useTrash';
import * as trashApi from '../api/trash';

vi.mock('../api/trash', () => ({ getTrash: vi.fn() }));
vi.mock('../auth', () => ({ isAuthenticated: () => true }));

const contact = {
  id: 1,
  firstname: 'Alice',
  lastname: 'Johnson',
  nickname: '',
  email: '',
  photo_thumbnail: '',
  deleted_at: '2030-01-01T10:00:00Z',
  purge_at: '2030-01-31T10:00:00Z',
};

describe('useTrash', () => {
  beforeEach(() => vi.resetAllMocks());

  test('loads the trash and the retention period', async () => {
    vi.mocked(trashApi.getTrash).mockResolvedValue({ contacts: [contact], retention_days: 14 });
    const { result } = renderHook(() => useTrash());

    expect(result.current.loading).toBe(true);
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.contacts).toEqual([contact]);
    expect(result.current.retentionDays).toBe(14);
    expect(result.current.error).toBeNull();
  });

  test('exposes an error and refetch recovers from it', async () => {
    vi.mocked(trashApi.getTrash).mockRejectedValueOnce(new Error('boom'));
    const { result } = renderHook(() => useTrash());
    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
    });
    expect(result.current.contacts).toEqual([]);

    vi.mocked(trashApi.getTrash).mockResolvedValue({ contacts: [contact], retention_days: 30 });
    await act(async () => {
      await result.current.refetch();
    });
    expect(result.current.error).toBeNull();
    expect(result.current.contacts).toHaveLength(1);
  });
});
