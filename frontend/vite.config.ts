import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// During development, /api is proxied to the Go backend so the frontend can use
// same-origin relative URLs (no CORS in dev). Override the target with
// VITE_PROXY_TARGET. In production the app talks to VITE_API_BASE_URL.
export default defineConfig(({ mode }) => {
  const proxyTarget = process.env.VITE_PROXY_TARGET ?? "http://localhost:8080";
  return {
    plugins: [react(), tailwindcss()],
    server: {
      port: 5173,
      proxy:
        mode === "development"
          ? {
              "/api": { target: proxyTarget, changeOrigin: true },
            }
          : undefined,
    },
  };
});
