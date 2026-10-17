#!/usr/bin/env node
// Seeds a running dev backend (make dev) with a demo user and a few contacts.
// Safe to run again: existing user and contacts are kept.
// Usage: make seed   (BACKEND_PORT / SEED_API_URL override the target)
const base = process.env.SEED_API_URL ?? `http://localhost:${process.env.BACKEND_PORT ?? 8080}/api/v1`;
const USER = { username: 'demo', email: 'demo@example.com', password: 'DemoPassword123!' };
const CONTACTS = [
  {
    firstname: 'Alice',
    lastname: 'Johnson',
    emails: [{ type: 'home', value: 'alice@example.com' }],
    birthday: '1990-03-15',
    circles: ['Friends', 'Work'],
  },
  { firstname: 'Bob', lastname: 'Smith', phones: [{ type: 'mobile', value: '+1 555-0102' }], circles: ['Family'] },
  {
    firstname: 'Carol',
    lastname: 'Williams',
    emails: [{ type: 'home', value: 'carol@example.com' }],
    birthday: '1985-07-22',
    circles: ['Friends'],
  },
];

const json = { 'Content-Type': 'application/json' };
const post = (path, body, headers = {}) =>
  fetch(`${base}${path}`, { method: 'POST', headers: { ...json, ...headers }, body: JSON.stringify(body) });

async function main() {
  const health = await fetch(`${base.replace(/\/api\/v1$/, '')}/health`).catch(() => null);
  if (!health?.ok) throw new Error(`No backend at ${base}. Start it with "make dev" first.`);

  const reg = await post('/register', USER);
  if (reg.ok) console.log(`Registered user ${USER.username}`);
  else if (reg.status === 409) console.log(`User ${USER.username} exists`);
  else throw new Error(`Register failed: ${reg.status} ${await reg.text()} (is DISABLE_REGISTRATION set?)`);

  const login = await post('/login', { identifier: USER.username, password: USER.password });
  if (!login.ok) throw new Error(`Login failed: ${login.status} ${await login.text()}`);
  const cookie = login.headers
    .getSetCookie()
    .map((c) => c.split(';')[0])
    .join('; ');
  const auth = { Cookie: cookie };

  let failed = false;
  for (const contact of CONTACTS) {
    const name = `${contact.firstname} ${contact.lastname}`;
    const found = await fetch(`${base}/contacts?search=${encodeURIComponent(name)}&limit=5`, { headers: auth });
    if (!found.ok) throw new Error(`Contact search failed: ${found.status} ${await found.text()}`);
    const list = (await found.json()).contacts ?? [];
    if (list.some((c) => c.firstname === contact.firstname && c.lastname === contact.lastname)) {
      console.log(`  Exists: ${name}`);
      continue;
    }
    const res = await post('/contacts', contact, auth);
    if (!res.ok) failed = true;
    console.log(res.ok ? `  Created: ${name}` : `  Failed: ${name}: ${res.status} ${await res.text()}`);
  }
  if (failed) throw new Error('Some contacts could not be created');
  console.log(`Log in as ${USER.username} / ${USER.password}`);
}

main().catch((e) => {
  console.error(e.message);
  process.exit(1);
});
