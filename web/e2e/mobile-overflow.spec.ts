import { test, expect } from '@playwright/test';

/**
 * Horizontal-overflow regression gate.
 *
 * The app shell rendered every routed page as a flex item of a column flex
 * container (`<main class="... flex flex-col">`), and almost every page root
 * carries `mx-auto`. Auto cross-axis margins cancel `align-items: stretch`, so
 * each page root fell back to fit-content sizing and floored at the min-content
 * width of its widest `whitespace-nowrap` table (609px on /contests). That
 * pushed the document to 691px on a 390px phone, so every page scrolled
 * sideways. `min-w-0` alone does not fix it — fit-content is computed from
 * min/max-content independently of `min-width` — hence the explicit `w-full`
 * on the routed child in App.tsx.
 *
 * This lives in Playwright rather than vitest because jsdom has no layout
 * engine: only a real browser can report scrollWidth.
 */

const MOBILE = { width: 390, height: 844 };

/** Routes that render without a login. Auth-gated pages are covered by the
 *  same assertion in the authenticated spec, which injects a token. */
const PUBLIC_ROUTES = [
  '/',
  '/problems',
  '/problems/hello',
  '/contests',
  '/contests/12',
  '/contests/12/scoreboard',
  '/rankings',
  '/blog',
  '/gym',
  '/login',
  '/register',
  '/legal/privacy',
];

/**
 * The decisive signal: can the user actually scroll sideways? `body` carries
 * `overflow-x: hidden`, so `scrollWidth` alone can over-report — only a
 * successful `scrollTo` proves a real horizontal scroll region.
 */
async function assertNoHorizontalScroll(page: import('@playwright/test').Page) {
  const result = await page.evaluate(async () => {
    const doc = document.documentElement;
    const before = window.scrollX;
    window.scrollTo(10_000, 0);
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    const reached = window.scrollX;
    window.scrollTo(before, 0);
    return {
      scrollWidth: doc.scrollWidth,
      clientWidth: doc.clientWidth,
      reached,
    };
  });

  expect(
    result.reached,
    `page scrolls horizontally: scrollX reached ${result.reached} (scrollWidth ${result.scrollWidth} vs clientWidth ${result.clientWidth})`
  ).toBe(0);
  expect(result.scrollWidth).toBeLessThanOrEqual(result.clientWidth + 1);
}

test.describe('mobile horizontal overflow', () => {
  test.use({ viewport: MOBILE });

  for (const route of PUBLIC_ROUTES) {
    test(`${route} does not scroll sideways at 390px`, async ({ page }) => {
      const res = await page.goto(route);
      expect(res?.status()).toBe(200);
      // Let the page's own fetches settle so late-arriving tables are measured.
      await page.waitForLoadState('networkidle').catch(() => { });
      await assertNoHorizontalScroll(page);
    });
  }

  test('the cookie banner is clamped to the viewport', async ({ page }) => {
    await page.goto('/');
    await page.evaluate(() => localStorage.removeItem('cookie-consent'));
    await page.reload();
    const banner = page.getByRole('dialog', { name: 'Cookie consent' });
    await expect(banner).toBeVisible();
    const box = await banner.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.width).toBeLessThanOrEqual(MOBILE.width);
    await assertNoHorizontalScroll(page);
  });
});

test.describe('navbar breakpoint', () => {
  test('the full desktop nav is not shown until it fits', async ({ page }) => {
    // The desktop cluster needs ~964px. It used to switch on at `md` (768px),
    // which clipped the language selector, theme toggle and account menu off
    // the right edge of every 768–964px screen.
    await page.setViewportSize({ width: 820, height: 1180 });
    await page.goto('/');
    await page.waitForLoadState('networkidle').catch(() => { });

    await expect(page.getByRole('button', { name: 'Toggle menu' })).toBeVisible();
    await assertNoHorizontalScroll(page);
  });

  test('the desktop nav is shown at 1280px', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto('/');
    // The right cluster (search / language / theme / account) is the part that
    // was being clipped, so assert on it. "Login" is in that cluster and is
    // present whether or not this run is authenticated.
    const nav = page.locator('nav').first();
    await expect(nav.getByRole('link', { name: 'Login', exact: true })).toBeVisible();
    await expect(nav.getByRole('button', { name: 'Toggle menu' })).toBeHidden();
    await assertNoHorizontalScroll(page);
  });
});
