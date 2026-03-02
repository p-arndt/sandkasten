import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    globals: true,
    include: ["tests/**/*.test.ts"],
    passWithNoTests: false,
    coverage: {
      provider: "v8",
      reports: ["text", "lcov"],
    },

  },
  esbuild: {
    target: "es2022",
  },
});