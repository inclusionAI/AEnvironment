import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

export default defineConfig({
  base: "/console/",
  plugins: [react()],
  server: {
    proxy: {
      "/env": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
})
