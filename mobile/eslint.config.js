const tsParser = require('@typescript-eslint/parser');
const tsPlugin = require('@typescript-eslint/eslint-plugin');
const reactHooks = require('eslint-plugin-react-hooks');

// ESLint 9 requires a flat config. The React Native preset pulls in
// eslint-plugin-ft-flow, which crashes on ESLint 9 (context.getAllComments
// was removed), so this config stays small enough for `npm run lint` to run.
module.exports = [
  {
    ignores: ['node_modules/**', 'android/**', 'ios/**', 'coverage/**'],
  },
  {
    files: ['**/*.{js,jsx,ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaFeatures: { jsx: true },
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
      'react-hooks': reactHooks,
    },
    rules: {
      // Declared so existing eslint-disable comments name a real rule.
      '@typescript-eslint/no-var-requires': 'off',
      'react-hooks/exhaustive-deps': 'off',
      'no-console': 'off',
    },
  },
];
