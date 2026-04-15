/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
    globals: true,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:4242",
      "/ws": {
        target: "ws://localhost:4242",
        ws: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
