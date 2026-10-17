import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/setupTests.ts',
    // Playwright E2E tests live in e2e/ and are run separately
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.{test,spec}.{ts,tsx}', 'src/setupTests.ts', 'src/types/generated/**'],
      reporter: ['text-summary', 'json-summary', 'html'],
      reportsDirectory: 'coverage',
      // Current values rounded down. Raise them when coverage rises; never lower them.
      thresholds: { statements: 28, branches: 81, functions: 35, lines: 28 },
    },
    // Node >= 22 ships its own experimental localStorage global, which is
    // undefined without --localstorage-file and shadows jsdom's working one
    pool: 'forks',
    poolOptions: {
      forks: {
        execArgv: ['--no-experimental-webstorage'],
      },
    },
  },
});
