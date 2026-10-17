import { describe, test, expect, vi } from 'vitest';
import { ApiError } from '../api/client';
import { handleError } from './errorHandler';

describe('handleError', () => {
  const show = (error: unknown) => {
    const showError = vi.fn<(message: string) => void>();
    handleError(error, { operation: 'saving contact', silent: true }, { showError });
    return showError.mock.calls[0]?.[0];
  };

  test('shows the field errors of a validation failure', () => {
    expect(show(new ApiError('Validation failed', 'VALIDATION_ERROR', 400, { email: 'Email is invalid' }))).toBe(
      'Email is invalid',
    );
  });

  test('shows the message of a plain error or string', () => {
    expect(show(new Error('Network down'))).toBe('Network down');
    expect(show('Upload too large')).toBe('Upload too large');
  });

  test('shows a generic message for anything else', () => {
    expect(show(undefined)).toBe('An unexpected error occurred');
    expect(show({ status: 500 })).toBe('An unexpected error occurred');
  });
});
