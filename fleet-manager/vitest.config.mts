import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    // A component test sets jsdom for its own file with the docblock
    // `// @vitest-environment jsdom`. Every other test runs in Node.
    environment: "node",
    include: [
      "eslint/**/*.test.ts",
      "src/**/*.test.{ts,tsx}",
      "test/**/*.test.ts",
      "e2e/**/*.test.ts",
    ],
  },
});
