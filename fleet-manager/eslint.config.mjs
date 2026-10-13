import path from "node:path";
import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import prettier from "eslint-config-prettier/flat";
import noServerOnlyInClient from "./eslint/no-server-only-in-client.mjs";

export default defineConfig([
  ...nextVitals,
  ...nextTs,
  prettier,
  {
    plugins: {
      fleet: { rules: { "no-server-only-in-client": noServerOnlyInClient } },
    },
    rules: {
      "fleet/no-server-only-in-client": [
        "error",
        { srcDir: path.join(import.meta.dirname, "src") },
      ],
      // The Kubernetes client and the informers of the app need Node.js APIs
      // that the Edge runtime does not have.
      "no-restricted-syntax": [
        "error",
        {
          selector:
            "ExportNamedDeclaration VariableDeclarator[id.name='runtime'] Literal[value='edge']",
          message:
            'The fleet manager runs on the Node.js runtime only. Remove runtime = "edge".',
        },
      ],
    },
  },
  globalIgnores([
    ".next/**",
    "next-env.d.ts",
    "test/fixtures/**",
    "src/generated/**",
    ".next-gallery/**",
    "ui-gallery/**",
    "test-results/**",
    "playwright-report/**",
  ]),
]);
