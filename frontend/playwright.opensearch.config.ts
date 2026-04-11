import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for integration tests that run against the real
 * OpenSearch-backed API server (no route mocking).
 *
 * The backend-opensearch server is expected to already be running on
 * http://localhost:8080 before these tests start (started by CI).
 *
 * Usage:
 *   npm run test:e2e:opensearch
 */
export default defineConfig({
  testDir: './e2e',
  testMatch: '**/opensearch-integration.spec.ts',
  timeout: 60_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run dev -- --port 5173 --strictPort',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
    env: {
      // Point the frontend at the real OpenSearch backend
      VITE_API_URL: process.env.VITE_API_URL ?? 'http://localhost:8080',
    },
  },
});
