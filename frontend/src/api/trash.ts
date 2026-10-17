import { apiFetch, API_BASE_URL, getAuthHeaders } from './client';
import { handleResponse } from './errorHandling';

export interface TrashedContact {
  id: number;
  firstname: string;
  lastname: string;
  nickname: string;
  email: string;
  photo_thumbnail: string;
  deleted_at: string;
  /** When the automatic purge removes the contact for good. */
  purge_at: string;
}

export interface TrashResponse {
  contacts: TrashedContact[];
  retention_days: number;
}

export async function getTrash(): Promise<TrashResponse> {
  const response = await apiFetch(`${API_BASE_URL}/trash/contacts`, {
    method: 'GET',
    headers: getAuthHeaders(),
  });
  const data = await handleResponse(response, 'Unable to load the trash.');
  return { contacts: data?.['contacts'] || [], retention_days: data?.['retention_days'] ?? 30 };
}

export async function restoreTrashedContact(id: number): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/trash/contacts/${id}/restore`, {
    method: 'POST',
    headers: getAuthHeaders(),
  });
  await handleResponse(response, 'Unable to restore the contact.');
}

export async function purgeTrashedContact(id: number): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/trash/contacts/${id}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });
  await handleResponse(response, 'Unable to delete the contact permanently.');
}

/** Permanently deletes everything in the trash and returns how many contacts were removed. */
export async function emptyTrash(): Promise<number> {
  const response = await apiFetch(`${API_BASE_URL}/trash/contacts`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });
  const data = await handleResponse(response, 'Unable to empty the trash.');
  return data?.['purged'] ?? 0;
}
