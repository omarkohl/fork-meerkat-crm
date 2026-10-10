import { describe, test, expect, beforeAll } from 'vitest';
import i18n from '../i18n/config';
import { describeAuditDetail } from './auditDetail';

describe('describeAuditDetail', () => {
  beforeAll(async () => {
    await i18n.changeLanguage('en');
  });
  const t = i18n.t.bind(i18n);

  test('translates codes', () => {
    expect(describeAuditDetail('reason:unknown_account', t)).toBe('Reason: Unknown account');
    expect(describeAuditDetail('reason:wrong_password_locked', t)).toBe('Reason: Wrong password, account locked');
    expect(describeAuditDetail('format:csv', t)).toBe('Format: CSV');
    expect(describeAuditDetail('method:oidc', t)).toBe('Method: Single sign-on (OIDC)');
  });

  test('shows user data as it is and splits several segments', () => {
    expect(describeAuditDetail('token:csv', t)).toBe('API token: csv');
    expect(describeAuditDetail('backup:meerkat-backup-1.tar.gz', t)).toBe('Backup: meerkat-backup-1.tar.gz');
    expect(describeAuditDetail('user:alice#3;changed:email,password', t)).toBe(
      'User: alice (#3) · Changed: E-mail, Password',
    );
  });

  test('is empty for no detail and keeps unknown codes readable', () => {
    expect(describeAuditDetail('', t)).toBe('');
    expect(describeAuditDetail('reason:something_new', t)).toBe('Reason: something_new');
  });

  test('is translated in every locale', async () => {
    for (const lang of ['de', 'it', 'es', 'fr']) {
      await i18n.changeLanguage(lang);
      expect(describeAuditDetail('reason:unknown_account', t), lang).not.toBe('Reason: Unknown account');
      expect(describeAuditDetail('changed:password', t), lang).not.toContain('auditLog.');
    }
    await i18n.changeLanguage('en');
  });
});
