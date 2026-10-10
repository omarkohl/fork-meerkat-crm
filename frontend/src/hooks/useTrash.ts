import { useState, useEffect, useCallback } from 'react';
import { isAuthenticated } from '../auth';
import { getTrash, TrashedContact } from '../api/trash';
import { handleFetchError } from '../utils/errorHandler';

interface UseTrashResult {
  contacts: TrashedContact[];
  retentionDays: number;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

export function useTrash(): UseTrashResult {
  const [contacts, setContacts] = useState<TrashedContact[]>([]);
  const [retentionDays, setRetentionDays] = useState(30);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchTrash = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      if (!isAuthenticated()) {
        throw new Error('No authentication token found');
      }
      const data = await getTrash();
      setContacts(data.contacts);
      setRetentionDays(data.retention_days);
    } catch (err) {
      setError(handleFetchError(err, 'fetching trash'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTrash();
  }, [fetchTrash]);

  return { contacts, retentionDays, loading, error, refetch: fetchTrash };
}
