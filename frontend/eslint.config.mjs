// Flat config (ESLint 9). Every rule is an error: a warning gets ignored by everyone
// (docs/adr/0002-rules-are-machine-checks.md).
import js from '@eslint/js';
import comments from '@eslint-community/eslint-plugin-eslint-comments/configs';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {
    ignores: [
      'src/types/generated/**',
      'build/**',
      'coverage/**',
      'public/**',
      'playwright-report/**',
      'test-results/**',
    ],
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      ...tseslint.configs.strictTypeChecked,
      jsxA11y.flatConfigs.recommended,
      comments.recommended,
    ],
    plugins: { 'react-hooks': reactHooks },
    languageOptions: {
      globals: globals.browser,
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
    linterOptions: { reportUnusedDisableDirectives: 'error' },
    rules: {
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'error',
      '@eslint-community/eslint-comments/require-description': 'error',
      '@eslint-community/eslint-comments/no-unlimited-disable': 'error',
      // One clock read, in src/utils/now.ts (docs/adr/0009-injected-clock.md)
      'no-restricted-syntax': [
        'error',
        {
          selector: "NewExpression[callee.name='Date'][arguments.length=0]",
          message:
            'Do not read the clock directly: use now() from src/utils/now.ts. See docs/adr/0009-injected-clock.md.',
        },
        {
          selector: "CallExpression[callee.name='Date']",
          message:
            'Date() without new reads the clock: use now() from src/utils/now.ts. See docs/adr/0009-injected-clock.md.',
        },
        {
          selector: "CallExpression[callee.object.name='Date'][callee.property.name='now']",
          message:
            'Do not read the clock directly: use now() from src/utils/now.ts. See docs/adr/0009-injected-clock.md.',
        },
      ],
    },
  },
  {
    // Tests may read the clock
    files: ['src/**/*.test.{ts,tsx}'],
    rules: { 'no-restricted-syntax': 'off' },
  },
);
