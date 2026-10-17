// Relationship-related API calls
import { apiFetch, API_BASE_URL, getAuthHeaders, parseErrorResponse } from './client';
import type { components, operations } from '../types/generated/api';
import type { WithoutNull } from '../types/utils';

type RelatedContact = components['schemas']['RelatedContactResponse'];
export type Relationship = WithoutNull<
  Omit<components['schemas']['RelationshipResponse'], 'related_contact' | 'source_contact' | 'related_contact_id'>
> & {
  related_contact_id: number | null;
  related_contact?: RelatedContact;
};
// The source contact is absent when it is no longer there (in the trash)
export type IncomingRelationship = Omit<Relationship, 'related_contact'> & {
  source_contact?: components['schemas']['SourceContactResponse'];
};
export type RelationshipFormData = Omit<components['schemas']['RelationshipBody'], 'related_contact_id'> & {
  related_contact_id?: number | null;
};

type RelationshipsListBody = operations['list-relationships']['responses']['200']['content']['application/json'];
type IncomingListBody = operations['list-incoming-relationships']['responses']['200']['content']['application/json'];

export interface RelationshipsResponse extends Omit<RelationshipsListBody, 'relationships'> {
  relationships: Relationship[];
}

export interface IncomingRelationshipsResponse extends Omit<IncomingListBody, 'incoming_relationships'> {
  incoming_relationships: IncomingRelationship[];
}

// Get all relationships for a contact
export async function getRelationships(contactId: number | string): Promise<RelationshipsResponse> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/relationships`, { headers: getAuthHeaders() });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as RelationshipsResponse;
}

// Get all incoming relationships for a contact (relationships pointing to this contact)
export async function getIncomingRelationships(contactId: number | string): Promise<IncomingRelationshipsResponse> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/incoming-relationships`, {
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as IncomingRelationshipsResponse;
}

// Create a new relationship
export async function createRelationship(
  contactId: number | string,
  data: RelationshipFormData,
): Promise<Relationship> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/relationships`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  const result = (await response.json()) as { relationship: Relationship };
  return result.relationship;
}

// Update an existing relationship
export async function updateRelationship(
  contactId: number | string,
  relationshipId: number,
  data: RelationshipFormData,
): Promise<Relationship> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/relationships/${relationshipId}`, {
    method: 'PUT',
    headers: getAuthHeaders(),
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return (await response.json()) as Relationship;
}

// Delete a relationship
export async function deleteRelationship(contactId: number | string, relationshipId: number): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/contacts/${contactId}/relationships/${relationshipId}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }
}

// Preset relationship types
export const RELATIONSHIP_TYPES = [
  'Child',
  'Parent',
  'Mother',
  'Father',
  'Spouse',
  'Partner',
  'Sibling',
  'Brother',
  'Sister',
  'Friend',
  'Best Friend',
  'Colleague',
  'Manager',
  'Employee',
  'Neighbor',
  'Other',
] as const;
