import type { TFunction } from 'i18next';

// Keys whose value is one of our own codes (translated); other values are user data
// (a token or file name) and are shown as they are.
const CODED_KEYS = new Set(['reason', 'method', 'format', 'changed']);

/**
 * Turns an audit entry's detail into a readable, translated line. The server stores details
 * as ";"-separated "key:value" segments, e.g. "user:alice#3;changed:email,password".
 */
export function describeAuditDetail(detail: string, t: TFunction): string {
  return detail
    .split(';')
    .filter(Boolean)
    .map((segment) => {
      const i = segment.indexOf(':');
      if (i < 0) return t(`auditLog.values.${segment}`, { defaultValue: segment });
      const key = segment.slice(0, i);
      const value = segment.slice(i + 1);
      const label = t(`auditLog.keys.${key}`, { defaultValue: key });
      const code = (v: string) => t(`auditLog.values.${v}`, { defaultValue: v });
      let shown = value;
      if (key === 'changed') {
        shown = value.split(',').map(code).join(', ');
      } else if (key === 'user') {
        const hash = value.lastIndexOf('#');
        shown = hash > 0 ? `${value.slice(0, hash)} (#${value.slice(hash + 1)})` : value;
      } else if (CODED_KEYS.has(key)) {
        shown = code(value);
      }
      return `${label}: ${shown}`;
    })
    .join(' · ');
}
