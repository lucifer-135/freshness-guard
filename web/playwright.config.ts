import { defineConfig, devices } from '@playwright/test'

// Smoke tests against the e2e stack (Bifrost with the plugin, fake models and
// fake Tavily). They run in Docker: docker compose ... run --rm webtest.
// The "screenshots" project is separate: it takes the README images against
// the real stack and is never part of the default run.
export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: 0,
  workers: 1,
  use: {
    baseURL: `${process.env.FG_URL ?? 'http://localhost:8091'}/freshness/ui/`,
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', testIgnore: /screenshots\.spec\.ts/, use: { ...devices['Desktop Chrome'] } },
    {
      name: 'screenshots',
      testMatch: /screenshots\.spec\.ts/,
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 }, colorScheme: 'light' },
    },
  ],
})
