import { renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useOIDCConfig } from './useOIDCConfig';

describe('useOIDCConfig', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('reads enabled and the provider name from the API', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue({ ok: true, json: () => Promise.resolve({ enabled: true, provider_name: 'Authentik' }) }),
    );
    const { result } = renderHook(() => useOIDCConfig());
    await waitFor(() => {
      expect(result.current.enabled).toBe(true);
    });
    expect(result.current.provider_name).toBe('Authentik');
  });

  it('falls back to SSO and stays disabled when the request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const { result } = renderHook(() => useOIDCConfig());
    await new Promise((r) => setTimeout(r, 0));
    expect(result.current).toEqual({ enabled: false, provider_name: 'SSO' });
  });
});
