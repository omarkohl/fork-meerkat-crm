// Module-graph rules for src/ (docs/adr/0010-target-layering.md). `make arch` runs them.
// Violations that exist today are recorded per (from, to) pair in
// .dependency-cruiser-known-violations.json; the reason per rule class is in
// docs/development/frontend.md. New violations fail; fixed ones must be removed from the
// file (scripts/arch-frontend.mjs checks both).
/** @type {import('dependency-cruiser').IConfiguration} */
module.exports = {
  forbidden: [
    {
      name: 'no-circular',
      severity: 'error',
      comment: 'Circular imports make load order matter and hide the real layering.',
      from: {},
      to: { circular: true },
    },
    {
      name: 'no-unresolvable',
      severity: 'error',
      comment: 'An import that does not resolve is a bug waiting for the build.',
      from: {},
      to: { couldNotResolve: true },
    },
    {
      name: 'pages-use-api-through-hooks',
      severity: 'error',
      comment:
        'A page (src/*Page.tsx) gets server data through a hook in src/hooks, not by calling src/api itself. See docs/adr/0010-target-layering.md.',
      from: { path: '^src/[A-Za-z]+Page\\.tsx$' },
      to: { path: '^src/api/' },
    },
    {
      name: 'api-client-is-private-to-api',
      severity: 'error',
      comment: 'src/api/client.ts is the HTTP plumbing of src/api. Everyone else uses the resource modules.',
      from: { pathNot: '^src/api/' },
      to: { path: '^src/api/client\\.ts$' },
    },
    {
      name: 'api-barrel-hides-client',
      severity: 'error',
      comment:
        'src/api/index.ts must not re-export client.ts: that would let everyone reach the HTTP plumbing through the barrel.',
      from: { path: '^src/api/index\\.ts$' },
      to: { path: '^src/api/client\\.ts$' },
    },
  ],
  options: {
    doNotFollow: { path: 'node_modules' },
    tsPreCompilationDeps: true,
    tsConfig: { fileName: 'tsconfig.json' },
    // Tests and setup may reach anywhere. vite-env.d.ts only references the
    // vite/client types, which have no module to resolve.
    exclude: { path: '\\.test\\.(ts|tsx)$|^src/setupTests\\.ts$|^src/vite-env\\.d\\.ts$' },
    enhancedResolveOptions: { exportsFields: ['exports'], conditionNames: ['import', 'require', 'node', 'default'] },
  },
};
