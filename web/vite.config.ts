import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { configDefaults, defineConfig } from 'vitest/config'

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
  // Playwright runs e2e/ in Docker; Vitest runs only the unit tests.
  test: { environment: 'jsdom', setupFiles: './src/test/setup.ts', css: false, exclude: [...configDefaults.exclude, 'e2e/**'] },
})
