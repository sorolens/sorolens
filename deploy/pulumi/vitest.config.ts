import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    environment: "node",
    // Pulumi runtime mocks are process-global; keep each file isolated.
    pool: "forks",
    // Loading @pulumi/aws is slow on small machines.
    testTimeout: 60_000,
    hookTimeout: 60_000,
  },
});
