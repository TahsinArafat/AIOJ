import { test, expect } from '@playwright/test';
import { loginAsSeedAdmin, uniq } from './lib/api';

/**
 * Wave 0 auth gate.
 *
 * DOM NOTE (verified against web/src/pages/Register.tsx and Login.tsx on
 * 2026-06-13): the inputs carry **no placeholder** attributes — each is wrapped
 * in a <div> after a <label>. So we select by label text, which is also what a
 * real user does. Recorded here because the original implementation plan
 * guessed `getByPlaceholder` and would have failed.
 */

test('register via UI stores a real session', async ({ page }) => {
  const user = uniq();
  await page.goto('/register');

  await page.getByLabel('Username').fill(user);
  await page.getByLabel('Email').fill(`${user}@aioj.test`);
  // Must satisfy BOTH gates: Register.tsx enforces length >= 12 client-side,
  // and internal/auth.ValidatePasswordStrength additionally requires an upper,
  // lower, digit and symbol. A value that clears only the length check (e.g.
  // "strongpass1") is rejected by the API, so the UI never sets a session.
  await page.getByLabel('Password').fill('Aioj-Sim-2026!');
  await page.getByRole('button', { name: /register/i }).click();

  // On success the SPA calls setTokens(...) and navigates to "/". A hard
  // navigation is NOT required — poll localStorage as the app writes it.
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem('access_token')), { timeout: 15_000 })
    .toBeTruthy();
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem('refresh_token')), { timeout: 15_000 })
    .toBeTruthy();

  // And the token really is a session: a protected route loads.
  await page.goto('/problems');
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 15_000 });
});

test('login via UI as the seeded admin reaches an admin route', async ({ page }) => {
  // Confirm the seeder actually produced an admin before driving the UI.
  const tokens = await loginAsSeedAdmin();
  expect(tokens.user.role).toBe('admin');

  await page.goto('/login');
  await page.getByLabel('Username').fill(tokens.user.username);
  await page.getByLabel('Password').fill(process.env.SEED_ADMIN_PASSWORD ?? 'Aioj-Sim-Admin-2026!');
  await page.getByRole('button', { name: /^login$/i }).click();

  // Admin-only page heading proves the JWT role round-trips end to end.
  await page.goto('/admin');
  await expect(page.getByRole('heading', { name: /admin dashboard/i })).toBeVisible({
    timeout: 15_000,
  });
});

test('a bad password is rejected, not silently accepted', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('Username').fill(uniq());
  await page.getByLabel('Password').fill('definitely-wrong');
  await page.getByRole('button', { name: /^login$/i }).click();

  await expect(page.getByText(/login failed|invalid|incorrect|error/i).first()).toBeVisible({
    timeout: 15_000,
  });
  expect(await page.evaluate(() => localStorage.getItem('access_token'))).toBeNull();
});
