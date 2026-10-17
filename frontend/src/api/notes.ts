// Notes-related API calls
import { apiFetch, API_BASE_URL, getAuthHeaders, parseErrorResponse } from './client';
import type { components, operations } from '../types/generated/api';

export type Note = components['schemas']['NoteResponse'];
type NotesListResponse = operations['list-unassigned-notes']['responses']['200']['content']['application/json'];
type ContactNotesResponse = operations['list-contact-notes']['responses']['200']['content']['application/json'];
type NoteBody = components['schemas']['NoteBody'];
type SavedNoteResponse = operations['create-note']['responses']['200']['content']['application/json'];

export type NotesResponse = Partial<NotesListResponse> & ContactNotesResponse;

export interface GetNotesParams {
  page?: number;
  limit?: number;
  search?: string;
  fromDate?: string;
  toDate?: string;
}

// Get notes for a contact
export async function getContactNotes(contactId: string | number): Promise<NotesResponse> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/notes`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as NotesResponse;
}

// Get all unassigned notes
export async function getUnassignedNotes(params: GetNotesParams = {}): Promise<NotesResponse> {
  const { page = 1, limit = 25 } = params;
  const search = params.search?.trim();

  const queryParams = new URLSearchParams({
    page: page.toString(),
    limit: limit.toString(),
  });

  if (search) {
    queryParams.append('search', search);
  }

  if (params.fromDate) {
    queryParams.append('fromDate', params.fromDate);
  }

  if (params.toDate) {
    queryParams.append('toDate', params.toDate);
  }

  const response = await apiFetch(`${API_BASE_URL}/notes?${queryParams.toString()}`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as NotesListResponse;
}

// Get single note
export async function getNote(id: string | number): Promise<Note> {
  const response = await apiFetch(`${API_BASE_URL}/notes/${id}`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as Note;
}

// Create note for contact
export async function createNote(contactId: string | number, data: NoteBody): Promise<SavedNoteResponse> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/notes`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as SavedNoteResponse;
}

// Create unassigned note
export async function createUnassignedNote(data: NoteBody): Promise<SavedNoteResponse> {
  const response = await apiFetch(`${API_BASE_URL}/notes`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as SavedNoteResponse;
}

// Update note
export async function updateNote(id: string | number, data: NoteBody): Promise<SavedNoteResponse> {
  const response = await apiFetch(`${API_BASE_URL}/notes/${id}`, {
    method: 'PUT',
    headers: getAuthHeaders(),
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as SavedNoteResponse;
}

// Delete note
export async function deleteNote(id: string | number): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/notes/${id}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }
}
