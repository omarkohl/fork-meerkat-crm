import { describe, test, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import RelationshipList from './RelationshipList';
import type { IncomingRelationship } from '../api/relationships';

vi.mock('../DateFormatProvider', () => ({
  useDateFormat: () => ({ formatBirthday: (b: string) => b, calculateAge: () => null }),
}));

const incoming = (ID: number, source?: { ID: number; firstname: string; lastname: string }): IncomingRelationship => ({
  ID,
  CreatedAt: '',
  UpdatedAt: '',
  DeletedAt: null,
  name: 'x',
  type: 'Friend',
  gender: '',
  birthday: '',
  contact_id: 1,
  related_contact_id: 2,
  ...(source ? { source_contact: source } : {}),
});

describe('RelationshipList incoming relationships', () => {
  test('a relationship whose source contact is gone is left out instead of breaking the list', () => {
    render(
      <MemoryRouter>
        <RelationshipList
          relationships={[]}
          incomingRelationships={[incoming(1), incoming(2, { ID: 7, firstname: 'Bob', lastname: 'Brown' })]}
          onEdit={vi.fn()}
          onDelete={vi.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText('Bob Brown')).toBeTruthy();
    expect(screen.getAllByRole('button')).toHaveLength(1);
  });
});
