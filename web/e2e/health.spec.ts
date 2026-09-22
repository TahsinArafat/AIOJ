import { test, expect } from '@playwright/test';
import { health } from './lib/api';

/**
 * Wave 0 health gate — proves nginx proxies `/api/*` to the backend and the
 * backend can reach Postgres/Redis. Verified against live `main` (nginx.conf
 * `location /api/ { proxy_pass http://backend:8080; }`).
 */
test('backend health is ok through nginx', async ({ request }) => {
  const r = await request.get('/api/health');
  expect(r.status()).toBe(200);
  expect(await r.json()).toMatchObject({ status: 'ok' });
});

test('health helper agrees with the raw endpoint', async () => {
  await expect(health()).resolves.toMatchObject({ status: 'ok' });
});

test('the SPA serves the app shell', async ({ page }) => {
  const res = await page.goto('/');
  expect(res?.status()).toBe(200);
  // index.html mounts a #root; the app shell must not be a 404 page.
  await expect(page.locator('#root')).toBeAttached();
});
