import { dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { FlatCompat } from "@eslint/eslintrc";

// ESLint 9 reads this "flat" file. Next.js still ships its rules in the older format, which FlatCompat translates.
const compat = new FlatCompat({ baseDirectory: dirname(fileURLToPath(import.meta.url)) });

const config = [
  ...compat.extends("next/core-web-vitals"),
  {
    // What is built or generated is not ours to lint.
    ignores: [".next/**", ".next-e2e/**", "node_modules/**", "coverage/**", "test-results/**", "playwright-report/**", "next-env.d.ts"],
  },
];

export default config;
