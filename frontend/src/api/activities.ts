// Activities-related API calls
import { apiFetch, API_BASE_URL, getAuthHeaders, parseErrorResponse } from './client';
import type { components, operations } from '../types/generated/api';

// The API sends null for a contact without circles; the UI treats that as absent.
export type ActivityContact = Omit<components['schemas']['ContactRefResponse'], 'circles'> & { circles?: string[] };
export type Activity = Omit<components['schemas']['ActivityResponse'], 'contacts'> & { contacts?: ActivityContact[] };
type ActivitiesListResponse = operations['list-activities']['responses']['200']['content']['application/json'];
export interface ActivitiesResponse extends Omit<ActivitiesListResponse, 'activities'> {
  activities: Activity[];
}
interface ContactActivitiesResponse {
  activities: Activity[];
}
type ActivityBody = components['schemas']['ActivityBody'];
type SavedActivityResponse = operations['create-activity']['responses']['200']['content']['application/json'];

export interface GetActivitiesParams {
  page?: number;
  limit?: number;
  includeContacts?: boolean;
  search?: string;
  fromDate?: string;
  toDate?: string;
}

// Get all activities
export async function getActivities(params: GetActivitiesParams): Promise<ActivitiesResponse> {
  const { page = 1, limit = 25, includeContacts = false } = params;
  const search = params.search?.trim();

  const queryParams = new URLSearchParams({
    page: page.toString(),
    limit: limit.toString(),
  });

  if (includeContacts) {
    queryParams.append('include', 'contacts');
  }

  if (search) {
    queryParams.append('search', search);
  }

  if (params.fromDate) {
    queryParams.append('fromDate', params.fromDate);
  }

  if (params.toDate) {
    queryParams.append('toDate', params.toDate);
  }

  const response = await apiFetch(`${API_BASE_URL}/activities?${queryParams.toString()}`, {
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as ActivitiesResponse;
}

// Get activities for a contact
export async function getContactActivities(contactId: string | number): Promise<ContactActivitiesResponse> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/activities`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as ContactActivitiesResponse;
}

// Get single activity
export async function getActivity(id: string | number): Promise<Activity> {
  const response = await apiFetch(`${API_BASE_URL}/activities/${id}`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as Activity;
}

// Create activity
export async function createActivity(data: ActivityBody): Promise<SavedActivityResponse> {
  const response = await apiFetch(`${API_BASE_URL}/activities`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as SavedActivityResponse;
}

// Update activity
export async function updateActivity(id: string | number, data: ActivityBody): Promise<Activity> {
  const response = await apiFetch(`${API_BASE_URL}/activities/${id}`, {
    method: 'PUT',
    headers: getAuthHeaders(),
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as Activity;
}

// Delete activity
export async function deleteActivity(id: string | number): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/activities/${id}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }
}
