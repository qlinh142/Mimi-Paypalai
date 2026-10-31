import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

/**
 * Bản prototype một file để xem trên điện thoại (bước 5 checklist): dữ liệu giả lập, HashRouter,
 * không service worker, mọi JS, CSS và font gói vào dist-proto/index.html.
 * Chạy: npm run build:proto
 */
export default defineConfig({
  plugins: [
    react(),
    // Không có service worker trong bản một file: thay virtual:pwa-register bằng hàm rỗng.
    {
      name: "proto-no-pwa",
      resolveId: (id) => (id === "virtual:pwa-register" ? "\0proto-pwa" : null),
      load: (id) => (id === "\0proto-pwa" ? "export function registerSW() { return () => Promise.resolve(); }" : null),
    },
  ],
  define: {
    "import.meta.env.VITE_API_MODE": JSON.stringify("mock"),
    "import.meta.env.VITE_ROUTER": JSON.stringify("hash"),
  },
  build: {
    outDir: "dist-proto",
    target: "es2022",
    assetsInlineLimit: 100_000_000,
    cssCodeSplit: false,
    modulePreload: false,
    rolldownOptions: { output: { codeSplitting: false } },
    chunkSizeWarningLimit: 5000,
  },
});
