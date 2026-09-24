import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: false,
    // Unit tests live next to the code they cover (*.test.tsx). The Playwright
    // specs under tests/e2e are run by `pnpm test:e2e`, not vitest; without this
    // exclude vitest's default spec glob picks them up and errors with
    // "Playwright Test did not expect test() to be called here".
    include: ["**/*.test.{ts,tsx}"],
    exclude: ["**/node_modules/**", "**/.next/**", "tests/e2e/**"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
      "@sorolens/ui": path.resolve(__dirname, "../../packages/ui/src/index.ts"),
      "@sorolens/xdr": path.resolve(__dirname, "../../packages/xdr/src/index.ts"),
    },
  },
});
