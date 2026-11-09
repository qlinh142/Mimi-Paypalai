import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      // "prompt": bản mới tải ngầm, người dùng bấm "Tải lại" khi sẵn sàng (không mất form đang nhập).
      registerType: "prompt",
      includeAssets: ["icons/mimi.svg", "icons/apple-touch-icon.png"],
      manifest: {
        name: "Mimi",
        short_name: "Mimi",
        lang: "en",
        start_url: "/",
        scope: "/",
        display: "standalone",
        background_color: "#FFFFFF",
        theme_color: "#FFFFFF",
        icons: [
          { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
          { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png" },
          { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
        ],
      },
      workbox: {
        // Chỉ cache app shell. Dữ liệu API được TanStack Query lưu vào IndexedDB,
        // service worker không bao giờ cache /api và trang xử lý đăng nhập của Firebase.
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/api\//, /^\/__\//],
        globPatterns: ["**/*.{js,css,html,svg,png,woff2}"],
        // Màn Phân tích chi tiêu (AG Studio, ~4,7 MB) chỉ tải khi mở màn, không nằm trong app shell.
        globIgnores: ["**/AnalyticsScreen-*.js", "**/AnalyticsScreen-*.css"],
      },
    }),
  ],
  server: {
    port: 5173,
    // Khi chạy backend Go ở máy: go run ./cmd/mimi serve-api (cổng 8080)
    proxy: { "/api": "http://localhost:8080" },
  },
  build: { target: "es2022", sourcemap: true },
});
