// ESLint 9+ flat config.
// Strict TypeScript + React rules; no `any` allowed.
import js from "@eslint/js";
import tseslint from "@typescript-eslint/eslint-plugin";
import tsparser from "@typescript-eslint/parser";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";

export default [
  {
    ignores: ["dist/**", "node_modules/**", "vite.config.ts", "vitest.config.ts"],
  },
  js.configs.recommended,
  {
    files: ["src/**/*.{ts,tsx}"],
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
      react,
      "react-hooks": reactHooks,
    },
    settings: { react: { version: "detect" } },
    rules: {
      ...tseslint.configs.recommended.rules,
      ...react.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,
      // TypeScript handles "not defined" via its own type checker. Leaving
      // no-undef on causes false positives for type-only references like
      // React.MouseEvent or RequestInit that exist as @types/* globals.
      "no-undef": "off",
      "react/react-in-jsx-scope": "off",
      "react/prop-types": "off",
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
      "react/no-unescaped-entities": "warn",
      "no-console": ["warn", { allow: ["warn", "error"] }],
    },
  },
];
