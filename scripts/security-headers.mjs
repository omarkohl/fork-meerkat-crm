// Checks that the two nginx deployments (frontend container, all-in-one image) send the same
// security headers. The headers live in one file, frontend/security-headers.conf, which both
// configs `include` in every location that serves the SPA. Proxied locations send none of their
// own, so the backend's API headers (default-src 'none') pass through unchanged and nothing is
// sent twice.
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

export const root = resolve(import.meta.dirname, '..')
export const SNIPPET_PATH = join(root, 'frontend', 'security-headers.conf')
export const INCLUDE_LINE = 'include /etc/nginx/security-headers.conf;'

export const REQUIRED_HEADERS = [
  'Content-Security-Policy',
  'X-Frame-Options',
  'X-Content-Type-Options',
  'Referrer-Policy',
  'Permissions-Policy',
  'Cross-Origin-Opener-Policy',
  'Cross-Origin-Resource-Policy',
  'Strict-Transport-Security',
]

const stripComments = text => text.replace(/#[^\n]*/g, '')

// Headers of the snippet as [name, value] pairs. A value starting with `$` is an nginx
// variable (HSTS: empty unless the request came over HTTPS).
export function parseSnippet(text) {
  const out = []
  for (const m of stripComments(text).matchAll(/add_header\s+(\S+)\s+("([^"]*)"|(\$\w+))\s+always\s*;/g)) {
    out.push([m[1], m[3] ?? m[4]])
  }
  return out
}

export function loadSnippetHeaders() {
  return parseSnippet(readFileSync(SNIPPET_PATH, 'utf8'))
}

// Splits a config into its location blocks: { path, body }.
export function parseLocations(conf) {
  const text = stripComments(conf)
  const locations = []
  const re = /location\s+([^{]+?)\s*\{/g
  let m
  while ((m = re.exec(text))) {
    let depth = 1
    let i = re.lastIndex
    while (depth > 0 && i < text.length) {
      if (text[i] === '{') depth++
      else if (text[i] === '}') depth--
      i++
    }
    locations.push({ path: m[1].trim(), body: text.slice(re.lastIndex, i - 1) })
    re.lastIndex = i
  }
  return locations
}

export function checkSnippet(text) {
  const problems = []
  const headers = parseSnippet(text)
  for (const name of REQUIRED_HEADERS) {
    if (!headers.some(([n]) => n.toLowerCase() === name.toLowerCase())) problems.push(`snippet lacks ${name}`)
  }
  const csp = headers.find(([n]) => n === 'Content-Security-Policy')?.[1] ?? ''
  const directive = name => csp.split(';').map(d => d.trim()).find(d => d.startsWith(name + ' ')) ?? ''
  for (const d of ["default-src 'self'", "object-src 'none'", "base-uri 'self'", "frame-ancestors 'none'"]) {
    if (!csp.split(';').map(x => x.trim()).includes(d)) problems.push(`CSP lacks ${d}`)
  }
  if (/'unsafe-(inline|eval)'/.test(directive('script-src'))) problems.push('CSP script-src allows unsafe code')
  if (!directive('script-src').includes("'self'")) problems.push("CSP script-src lacks 'self'")
  return problems
}

export function checkConfig(name, conf) {
  const problems = []
  const bare = stripComments(conf)
  for (const m of bare.matchAll(/add_header\s+(\S+)/g)) {
    if (m[1].toLowerCase() !== 'cache-control') problems.push(`${name}: add_header ${m[1]} outside the snippet`)
  }
  if (!/map\s+\$http_x_forwarded_proto\s+\$forwarded_proto/.test(bare)) problems.push(`${name}: forwarded_proto map missing`)
  if (!/map\s+\$forwarded_proto\s+\$hsts_header/.test(bare)) problems.push(`${name}: hsts_header map missing`)
  const locations = parseLocations(conf)
  if (locations.length === 0) problems.push(`${name}: no locations found`)
  for (const { path, body } of locations) {
    const proxied = /proxy_pass\b/.test(body)
    const included = body.includes(INCLUDE_LINE)
    if (proxied && included) problems.push(`${name}: proxied location ${path} must not add headers (backend sends its own)`)
    if (proxied && /\badd_header\b/.test(body)) problems.push(`${name}: proxied location ${path} has add_header`)
    if (!proxied && !/^\s*return\b/m.test(body) && !included) problems.push(`${name}: location ${path} serves content without the security headers`)
  }
  return problems
}

export function checkDockerfile(name, text, copySource) {
  return text.includes(`COPY ${copySource} /etc/nginx/security-headers.conf`)
    ? []
    : [`${name}: does not copy ${copySource} to /etc/nginx/security-headers.conf`]
}

// Everything that must hold for both deployment shapes. Returns a list of problems.
export function checkAll() {
  const read = p => readFileSync(join(root, p), 'utf8')
  return [
    ...checkSnippet(read('frontend/security-headers.conf')),
    ...checkConfig('frontend/nginx.conf', read('frontend/nginx.conf')),
    ...checkConfig('docker/nginx.conf', read('docker/nginx.conf')),
    ...checkDockerfile('frontend/Dockerfile', read('frontend/Dockerfile'), 'security-headers.conf'),
    ...checkDockerfile('Dockerfile', read('Dockerfile'), 'frontend/security-headers.conf'),
  ]
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const problems = checkAll()
  for (const p of problems) console.error(`security-headers: ${p}`)
  if (problems.length) process.exit(1)
  console.log('security-headers: ok')
}
