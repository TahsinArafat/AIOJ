import { defineConfig, devices } from '@playwright/test';

/**
 * Simulation harness config for the Full-Scale Completion program.
 *
 * The sim boots the *whole* product on the local Docker stack:
 *   docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim up -d --build
 * nginx then serves the Vite prod build on :8081 and proxies /api/* to the backend.
 *
 * Playwright runs on the host (it needs a real browser, not a container) and points at
 * that single origin, so every spec exercises the exact route a user would hit.
 */
export default defineConfig({
  testDir: './e2e',
  // The judge is real: compile + run inside go-judge. A cold worker needs time.
  timeout: 120_000,
  expect: { timeout: 15_000 },
  // One browser, one OJ — the sim shares a single database, so parallel runs would
  // step on each other's seeded state.
  fullyParallel: false,
  retries: 1,
  reporter: [['html', { outputFolder: 'e2e/artifacts/report' }], ['list']],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:8081',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
});
