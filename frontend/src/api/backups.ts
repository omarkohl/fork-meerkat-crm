import { apiFetch, API_BASE_URL, getAuthHeaders, parseErrorResponse } from './client';
import { handleResponse } from './errorHandling';
import { downloadFileFromResponse } from './export';

export interface Backup {
  name: string;
  size: number;
  created_at: string;
}

export interface BackupList {
  backups: Backup[];
  /** Whether the scheduled backup runs. Admins can always create one by hand. */
  enabled: boolean;
  /** Time of day (HH:MM, reminder timezone) of the scheduled backup. */
  time: string;
  retention_count: number;
  /** Where the backups are stored on the server. */
  directory: string;
}

export async function getBackups(): Promise<BackupList> {
  const response = await apiFetch(`${API_BASE_URL}/admin/backups`, {
    method: 'GET',
    headers: getAuthHeaders(),
  });
  const data = await handleResponse(response, 'Unable to load the backups.');
  return {
    backups: data?.['backups'] || [],
    enabled: !!data?.['enabled'],
    time: data?.['time'] ?? '',
    retention_count: data?.['retention_count'] ?? 0,
    directory: data?.['directory'] ?? '',
  };
}

export async function createBackup(): Promise<Backup> {
  const response = await apiFetch(`${API_BASE_URL}/admin/backups`, {
    method: 'POST',
    headers: getAuthHeaders(),
  });
  return (await handleResponse(response, 'Unable to create the backup.')) as Backup;
}

export async function downloadBackup(name: string): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/admin/backups/${encodeURIComponent(name)}`, {
    method: 'GET',
    headers: getAuthHeaders(),
  });
  if (!response.ok) {
    throw await parseErrorResponse(response);
  }
  await downloadFileFromResponse(response, name);
}

export async function deleteBackup(name: string): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/admin/backups/${encodeURIComponent(name)}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });
  await handleResponse(response, 'Unable to delete the backup.');
}
