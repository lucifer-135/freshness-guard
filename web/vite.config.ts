import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  base: '/freshness/ui/',
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(import.meta.dirname, 'src') } },
  server: {
    proxy: {
      // Only the plugin API is proxied; Vite itself serves the app under /freshness/ui/.
      '^/freshness/(?!ui)': { target: process.env.FG_API_URL ?? 'http://localhost:8090' },
    },
  },
  test: { environment: 'jsdom', setupFiles: './src/test/setup.ts', css: false },
})
