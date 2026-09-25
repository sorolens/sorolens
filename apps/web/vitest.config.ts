import { configDefaults, defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: false,
    setupFiles: ["./vitest.setup.ts"],
    // Playwright specs in tests/e2e run under `pnpm test:e2e`, not vitest.
    exclude: [...configDefaults.exclude, "tests/e2e/**"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
      "@sorolens/ui": path.resolve(__dirname, "../../packages/ui/src/index.ts"),
      "@sorolens/xdr": path.resolve(
        __dirname,
        "../../packages/xdr/src/index.ts"
      ),
    },
  },
});

