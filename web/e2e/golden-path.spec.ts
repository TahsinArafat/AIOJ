import { test, expect } from '@playwright/test';
import {
  HELLO_CPP,
  clearInbox,
  getProblemBySlug,
  health,
  login,
  messagesFor,
  pollForVerdict,
  registerRaw,
  submit,
  uniq,
  waitForLink,
} from './lib/api';
import { tokenFromLink } from './lib/mail';

/**
 * The program's golden path, as one ordered chain (spec §6).
 *
 * Kept as a SINGLE test on purpose: the value is in the sequence
 * register → mail-verify → login → submit → AC, so splitting it into
 * independent tests would let each pass while the chain broke.
 *
 * This file is also the regression suite for the mail leg. Before it existed,
 * `register()` verified users via a helper that silently returned when the
 * message was missing — so a broken email template, a wrong MAIL_PUBLIC_URL,
 * or a renamed route could not fail the suite. Here the delivered message is
 * asserted, the link is followed in a real browser, and the UI must confirm.
 */
test('golden path: register → mail-verify → login → submit → AC', async ({ page }) => {
  // 1. The stack is up.
  await expect((await health()).status).toBe('ok');

  // Start from an empty inbox so every message we see is one we caused.
  await clearInbox();

  // 2. Register a fresh user.
  const username = uniq('gp');
  const email = `${username}@aioj.test`;
  const registered = await registerRaw(username, email, 'Aioj-Sim-2026!');
  expect(registered.access_token).toBeTruthy();
  expect(registered.user.role).toBe('user');

  // 3. A verification email is actually delivered, and carries a token.
  const delivered = await messagesFor(email);
  expect(delivered.length, 'a verification email should be delivered on register').toBeGreaterThan(0);

  const link = await waitForLink(email, 'verify-email');
  expect(tokenFromLink(link)).toBeTruthy();

  // 4. Following the delivered link in a browser verifies the account.
  //    This is the link the backend built, so a template or public-URL
  //    regression fails here rather than passing silently.
  await page.goto(link);
  await expect(page.getByRole('heading', { name: /email verified/i })).toBeVisible({
    timeout: 20_000,
  });

  // 5. Login works after verification.
  const session = await login(username, 'Aioj-Sim-2026!');
  expect(session.access_token).toBeTruthy();
  expect(session.user.username).toBe(username);

  // 6. Submit to the seeded problem; the real judge pipeline returns AC.
  const problem = await getProblemBySlug(process.env.SEED_PROBLEM_SLUG ?? 'hello', session.access_token);
  const sub = await submit(session.access_token, problem.id, 'cpp-gpp-64', HELLO_CPP);
  expect(sub.id).toBeTruthy();
  expect(sub.status.toLowerCase()).toBe('pending');

  const graded = await pollForVerdict(session.access_token, sub.id, { timeoutMs: 120_000 });
  expect(graded.status.toLowerCase()).toBe('ac');
});

/**
 * Proves the verification leg above is load-bearing rather than decorative:
 * without it the same submit is refused. If this ever passes without a
 * verified email, the golden path's mail step proves nothing.
 */
test('an unverified account cannot submit', async () => {
  const username = uniq('unverified');
  const email = `${username}@aioj.test`;
  const tokens = await registerRaw(username, email, 'Aioj-Sim-2026!'); // deliberately NOT verified

  const problem = await getProblemBySlug(process.env.SEED_PROBLEM_SLUG ?? 'hello', tokens.access_token);
  const res = await fetch(
    `${(process.env.E2E_BASE_URL ?? 'http://localhost:8081').replace(/\/$/, '')}/api/submissions`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${tokens.access_token}`,
      },
      body: JSON.stringify({
        problem_id: problem.id,
        language: 'cpp-gpp-64',
        source_code: HELLO_CPP,
      }),
    },
  );

  expect(res.status).toBe(403);
  await expect(res.json()).resolves.toMatchObject({
    error: expect.stringContaining('not verified'),
  });
});
