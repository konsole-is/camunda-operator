import path from "node:path";
import { fileURLToPath } from "node:url";
import { RuleTester } from "eslint";
import tseslint from "typescript-eslint";
import { describe, it } from "vitest";
import rule from "./no-server-only-in-client.mjs";

RuleTester.describe = describe;
RuleTester.it = it;
RuleTester.itOnly = it.only;

const srcDir = fileURLToPath(
  new URL("../test/fixtures/server-only-rule/src", import.meta.url),
);
const filename = path.join(srcDir, "components", "panel.tsx");
const options = [{ srcDir }];
const client = (body: string) => `"use client";\n${body}\n`;
const refused = [{ messageId: "serverOnlyImport" }];

const tester = new RuleTester({
  languageOptions: {
    parser: tseslint.parser,
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

tester.run("no-server-only-in-client", rule, {
  valid: [
    {
      name: "a server component imports a server-only module",
      filename,
      options,
      code: `import { secretName } from "@/lib/secrets";`,
    },
    {
      name: "a client file imports an isomorphic module",
      filename,
      options,
      code: client(`import { merge } from "@/lib/merge";`),
    },
    {
      name: "a client file imports only types from a server-only module",
      filename,
      options,
      code: client(`import type { SecretName } from "@/lib/secrets";`),
    },
    {
      name: "a client file re-exports only types from a server-only module",
      filename,
      options,
      code: client(`export type { SecretName } from "@/lib/secrets";`),
    },
    {
      name: "a client file imports a package",
      filename,
      options,
      code: client(`import { useState } from "react";`),
    },
    {
      name: "a use client string after the first statement is not a directive",
      filename,
      options,
      code: `import { secretName } from "@/lib/secrets";\n"use client";`,
    },
  ],
  invalid: [
    {
      name: "an alias import",
      filename,
      options,
      code: client(`import { secretName } from "@/lib/secrets";`),
      errors: refused,
    },
    {
      name: "a relative import",
      filename,
      options,
      code: client(`import { secretName } from "../lib/secrets";`),
      errors: refused,
    },
    {
      name: "an import of a directory index",
      filename,
      options,
      code: client(`import { listPods } from "@/lib/k8s";`),
      errors: refused,
    },
    {
      name: "comments above the server-only marker",
      filename,
      options,
      code: client(`import { token } from "@/lib/annotated";`),
      errors: refused,
    },
    {
      name: "a side-effect import",
      filename,
      options,
      code: client(`import "@/lib/secrets";`),
      errors: refused,
    },
    {
      name: "a named re-export",
      filename,
      options,
      code: client(`export { secretName } from "@/lib/secrets";`),
      errors: refused,
    },
    {
      name: "a re-export of all names",
      filename,
      options,
      code: client(`export * from "@/lib/secrets";`),
      errors: refused,
    },
    {
      name: "a dynamic import",
      filename,
      options,
      code: client(`export const load = () => import("@/lib/secrets");`),
      errors: refused,
    },
    {
      name: "a single-quoted directive",
      filename,
      options,
      code: `'use client';\nimport { secretName } from "@/lib/secrets";\n`,
      errors: refused,
    },
  ],
});
