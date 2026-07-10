import { dirname } from "path";
import { fileURLToPath } from "url";
import { FlatCompat } from "@eslint/eslintrc";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const compat = new FlatCompat({
  baseDirectory: __dirname,
});

const eslintConfig = [
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  {
    ignores: [
      "node_modules/**",
      ".next/**",
      "out/**",
      "build/**",
      "next-env.d.ts",
      "**/__tests__/**",
      "**/*.test.*",
      "**/*.spec.*",
    ],
  },
  {
    rules: {
      "@typescript-eslint/no-explicit-any": "warn",
      "react/display-name": "warn",
      "react/no-unescaped-entities": "warn",
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" }
      ],
      // Env vars must resolve at runtime, not be inlined at build time.
      // See docs/adr/0001-runtime-env-configuration.md
      "no-restricted-syntax": [
        "error",
        {
          selector:
            "MemberExpression[object.object.name='process'][object.property.name='env'][property.name!='NODE_ENV']",
          message:
            "Do not read process.env. Add an accessor to lib/env.ts (backed by next-runtime-env) and call that. process.env is inlined at build time, which breaks one-image-many-environments. NODE_ENV is the only exception.",
        },
        {
          selector:
            "VariableDeclarator[init.object.name='process'][init.property.name='env']",
          message:
            "Do not destructure process.env. Add an accessor to lib/env.ts (backed by next-runtime-env) and call that.",
        },
      ],
    },
  },
  {
    // Test setup bridges process.env into the mocked env() accessor.
    files: ["vitest.setup.ts"],
    rules: { "no-restricted-syntax": "off" },
  },
];

export default eslintConfig;
