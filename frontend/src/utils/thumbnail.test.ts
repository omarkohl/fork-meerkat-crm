import { describe, test, expect } from 'vitest';
import { thumbnailSrc } from './thumbnail';

describe('thumbnailSrc', () => {
  test('no thumbnail gives undefined', () => {
    expect(thumbnailSrc('')).toBeUndefined();
    expect(thumbnailSrc(undefined)).toBeUndefined();
    expect(thumbnailSrc(null)).toBeUndefined();
  });

  test('a path stays a path on the same origin', () => {
    expect(thumbnailSrc('/api/v1/contacts/5/profile_picture?thumbnail=true&v=abc')).toBe(
      '/api/v1/contacts/5/profile_picture?thumbnail=true&v=abc',
    );
  });

  test('a full URL is left alone', () => {
    expect(thumbnailSrc('https://cdn.example/x.jpg')).toBe('https://cdn.example/x.jpg');
  });
});
