import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    environment: "node",
    include: [
      "eslint/**/*.test.ts",
      "src/**/*.test.{ts,tsx}",
      "test/**/*.test.ts",
    ],
  },
});
