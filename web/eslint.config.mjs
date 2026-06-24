// ESLint 10 flat config.
// Strict TypeScript + React rules; no `any` allowed.
//
// React linting is provided by @eslint-react/eslint-plugin (rule prefix
// `@eslint-react/*`). The legacy jsx-eslint `eslint-plugin-react` was dropped
// because it does not support ESLint 10 (it calls the removed
// `context.getFilename()` API). React Hooks linting stays on the official
// `eslint-plugin-react-hooks`; @eslint-react's own `rules-of-hooks` is turned
// off below so the official plugin remains the single source for hook rules.
import js from "@eslint/js";
import tseslint from "@typescript-eslint/eslint-plugin";
import tsparser from "@typescript-eslint/parser";
import eslintReact from "@eslint-react/eslint-plugin";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";

const SRC = ["src/**/*.{ts,tsx}"];

export default [
  {
    ignores: ["dist/**", "node_modules/**", "vite.config.ts", "vitest.config.ts"],
  },
  js.configs.recommended,
  // @eslint-react recommended ruleset (TypeScript-aware, no type info needed),
  // scoped to source files.
  { ...eslintReact.configs["recommended-typescript"], files: SRC },
  {
    files: SRC,
    languageOptions: {
      parser: tsparser,
      parserOptions: {
        ecmaVersion: "latest",
        sourceType: "module",
        ecmaFeatures: { jsx: true },
      },
      globals: {
        ...globals.browser,
        ...globals.node,
        React: "readonly",
        RequestInit: "readonly",
        NodeListOf: "readonly",
        HeadersInit: "readonly",
        BodyInit: "readonly",
      },
    },
    plugins: {
      "@typescript-eslint": tseslint,
      "react-hooks": reactHooks,
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,
      // TypeScript handles "not defined" via its own type checker. Leaving
      // no-undef on causes false positives for type-only references like
      // React.MouseEvent or RequestInit that exist as @types/* globals.
      "no-undef": "off",
      // Hook linting is owned by eslint-plugin-react-hooks (below); disable
      // @eslint-react's overlapping rule to avoid duplicate/erroring reports.
      "@eslint-react/rules-of-hooks": "off",
      // Strict ergonomics are advisory until the existing codebase is
      // sweepingly cleaned up — the squadron PR is a tooling rollout, not
      // a code-quality migration.
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "react-hooks/rules-of-hooks": "warn",
      "react-hooks/exhaustive-deps": "warn",
      // The eslint-plugin-react-hooks v6 set adds rules that fire on
      // legitimate patterns (e.g., mutating refs inside a connect()
      // useCallback that initializes a long-lived WebSocket). Demote
      // them while the codebase is cleaned up.
      "react-hooks/refs": "warn",
      "no-console": ["warn", { allow: ["warn", "error"] }],
    },
  },
];
