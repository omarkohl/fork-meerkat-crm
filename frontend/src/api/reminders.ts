// Reminder-related API calls
import { apiFetch, API_BASE_URL, getAuthHeaders, parseErrorResponse } from './client';
import type { components, operations } from '../types/generated/api';
import type { WithoutNull } from '../types/utils';

export type Reminder = WithoutNull<components['schemas']['ReminderResponse']>;
export type ReminderFormData = components['schemas']['ReminderBody'];

type CompletedReminderBody = operations['complete-reminder']['responses']['200']['content']['application/json'];
type RemindersListBody = operations['list-reminders']['responses']['200']['content']['application/json'];
type CompletionsListBody = operations['list-reminder-completions']['responses']['200']['content']['application/json'];

type CompletedReminder = Omit<CompletedReminderBody, 'reminder'> & { reminder?: Reminder };

export interface RemindersResponse extends Omit<RemindersListBody, 'reminders'> {
  reminders: Reminder[];
}

export type ReminderCompletion = WithoutNull<components['schemas']['CompletionResponse']>;

export interface CompletionsResponse extends Omit<CompletionsListBody, 'completions'> {
  completions: ReminderCompletion[];
}

// Get all reminders across all contacts
export async function getAllReminders(): Promise<Reminder[]> {
  const response = await apiFetch(`${API_BASE_URL}/reminders`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  const data = (await response.json()) as RemindersResponse;
  return data.reminders || [];
}

// Get upcoming reminders (next 7 days or at least next 10 reminders)
export async function getUpcomingReminders(): Promise<Reminder[]> {
  const response = await apiFetch(`${API_BASE_URL}/reminders/upcoming`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  const data = (await response.json()) as RemindersResponse;
  return data.reminders || [];
}

// Get reminders for a specific contact
export async function getRemindersForContact(contactId: number): Promise<Reminder[]> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/reminders`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  const data = (await response.json()) as RemindersResponse;
  return data.reminders || [];
}

// Get a single reminder
export async function getReminder(reminderId: number): Promise<Reminder> {
  const response = await apiFetch(`${API_BASE_URL}/reminders/${reminderId}`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as Reminder;
}

// Create a new reminder
export async function createReminder(contactId: number, reminderData: ReminderFormData): Promise<Reminder> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/reminders`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify(reminderData),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  const data = (await response.json()) as { reminder: Reminder };
  return data.reminder;
}

// Update an existing reminder
export async function updateReminder(reminderId: number, reminderData: Partial<ReminderFormData>): Promise<Reminder> {
  const response = await apiFetch(`${API_BASE_URL}/reminders/${reminderId}`, {
    method: 'PUT',
    headers: getAuthHeaders(),
    body: JSON.stringify(reminderData),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  const data = (await response.json()) as { reminder: Reminder };
  return data.reminder;
}

// Complete a reminder (marks as done and reschedules if recurring)
export async function completeReminder(reminderId: number): Promise<CompletedReminder> {
  const response = await apiFetch(`${API_BASE_URL}/reminders/${reminderId}/complete`, {
    method: 'POST',
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as CompletedReminder;
}

// Skip a reminder (reschedules recurring reminders without recording completion)
export async function skipReminder(reminderId: number): Promise<CompletedReminder> {
  const response = await apiFetch(`${API_BASE_URL}/reminders/${reminderId}/complete?skip=true`, {
    method: 'POST',
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as CompletedReminder;
}

// Delete a reminder
export async function deleteReminder(reminderId: number): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/reminders/${reminderId}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }
}

// Get reminder completions for a specific contact
export async function getCompletionsForContact(contactId: number): Promise<ReminderCompletion[]> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/reminder-completions`, {
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  const data = (await response.json()) as CompletionsResponse;
  return data.completions || [];
}

// Delete a reminder completion
export async function deleteCompletion(completionId: number): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/reminder-completions/${completionId}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }
}
