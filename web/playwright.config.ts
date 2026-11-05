import { defineConfig, devices } from '@playwright/test'

// Smoke tests against the e2e stack (Bifrost with the plugin, fake models and
// fake Tavily). They run in Docker: docker compose ... run --rm webtest.
export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: 0,
  workers: 1,
  use: {
    baseURL: `${process.env.FG_URL ?? 'http://localhost:8091'}/freshness/ui/`,
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
