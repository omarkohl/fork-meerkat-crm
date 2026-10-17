import { apiFetch, API_BASE_URL, getAuthHeaders } from './client';
import { handleResponse } from './errorHandling';

export interface Session {
  id: string;
  user_agent: string;
  ip: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  /** The session making this request. */
  current: boolean;
}

export async function getSessions(): Promise<Session[]> {
  const response = await apiFetch(`${API_BASE_URL}/sessions`, {
    method: 'GET',
    headers: getAuthHeaders(),
  });
  const data = await handleResponse(response, 'Unable to load sessions.');
  return data?.['sessions'] ?? [];
}

export async function revokeSession(id: string): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/sessions/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });
  await handleResponse(response, 'Unable to sign the session out.');
}

/** Revokes every session, this one included. */
export async function signOutEverywhere(): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/sessions/revoke-all`, {
    method: 'POST',
    headers: getAuthHeaders(),
  });
  await handleResponse(response, 'Unable to sign out everywhere.');
}
