import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: false,
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
      "@sorolens/ui": path.resolve(__dirname, "../../packages/ui/src/index.ts"),
      "@sorolens/xdr": path.resolve(__dirname, "../../packages/xdr/src/index.ts"),
    },
  },
});
