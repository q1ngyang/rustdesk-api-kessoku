import { defineConfig } from "vite";

export default defineConfig({
  build: {
    target: "es2022",
    sourcemap: true,
    assetsInlineLimit: 0,
  },
  server: {
    strictPort: true,
  },
});
