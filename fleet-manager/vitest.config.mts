import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { configDefaults, defineConfig } from "vitest/config";

// server-only throws unless Next.js loads it with the react-server
// condition. Vitest runs the server modules of src/lib in plain Node, so the
// tests load its empty module instead.
const serverOnlyStub = path.join(
  path.dirname(createRequire(import.meta.url).resolve("server-only")),
  "empty.js",
);

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "server-only": serverOnlyStub,
    },
  },
  test: {
    // A component test sets jsdom for its own file with the docblock
    // `// @vitest-environment jsdom`. Every other test runs in Node.
    environment: "node",
    projects: [
      {
        extends: true,
        test: {
          name: "unit",
          include: [
            "eslint/**/*.test.ts",
            "src/**/*.test.{ts,tsx}",
            "test/**/*.test.ts",
            "e2e/**/*.test.ts",
          ],
          exclude: [...configDefaults.exclude, "test/k8s/**"],
        },
      },
      {
        extends: true,
        test: {
          // These files run against one envtest API server, which the
          // global setup starts from KUBEBUILDER_ASSETS.
          name: "envtest",
          include: ["test/k8s/**/*.test.ts"],
          globalSetup: ["test/envtest/global-setup.ts"],
          testTimeout: 30_000,
        },
      },
    ],
  },
});
