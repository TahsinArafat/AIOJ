import { test, expect } from '@playwright/test';
import {
  HELLO_CPP,
  getProblemBySlug,
  getSubmission,
  loginAsSeedAdmin,
  pollForVerdict,
  submit,
} from './lib/api';

/**
 * Wave 0 milestone — the golden path proves the whole judge pipeline works on
 * the sim stack: API → Redis queue → judge-worker → go-judge → verdict → DB.
 *
 * The problem and its testcases are created by `cmd/seed` (Task 2): slug
 * "hello", testcase 1.in empty / 1.out "Hello, AIOJ!".
 */
test('submit cpp to the seeded problem → AC, and the verdict survives a reload', async ({
  page,
}) => {
  const tokens = await loginAsSeedAdmin();
  const problem = await getProblemBySlug('hello', tokens.access_token);

  const sub = await submit(tokens.access_token, problem.id);
  expect(sub.id).toBeTruthy();
  // A fresh submission is queued, not graded synchronously.
  expect(sub.status.toLowerCase()).toBe('pending');

  // Poll the real API until a terminal verdict. The worker compiles a static
  // C++ binary and runs it inside go-judge — seconds, not milliseconds.
  const graded = await pollForVerdict(tokens.access_token, sub.id, { timeoutMs: 120_000 });
  expect(graded.status.toLowerCase()).toBe('ac');

  // The same verdict must survive a reload — no phantom AC.
  await page.addInitScript(
    (t) => {
      localStorage.setItem('access_token', t.access);
      localStorage.setItem('refresh_token', t.refresh);
    },
    { access: tokens.access_token, refresh: tokens.refresh_token },
  );
  await page.goto(`/submissions/${sub.id}`);
  await expect(page.getByText(/accepted/i).first()).toBeVisible({ timeout: 20_000 });
});

test('a wrong program does not get AC', async () => {
  const tokens = await loginAsSeedAdmin();
  const problem = await getProblemBySlug('hello', tokens.access_token);

  const sub = await submit(
    tokens.access_token,
    problem.id,
    'cpp-gpp-64',
    '#include <iostream>\nint main(){ std::cout << "definitely wrong\\n"; }\n',
  );
  const graded = await pollForVerdict(tokens.access_token, sub.id, { timeoutMs: 120_000 });
  expect(graded.status.toLowerCase()).not.toBe('ac');
});

test('the canonical AC source is the one judged above', async () => {
  // Guard against the helper drifting away from what the test claims to submit.
  expect(HELLO_CPP).toContain('Hello, AIOJ!');

  const tokens = await loginAsSeedAdmin();
  const problem = await getProblemBySlug('hello', tokens.access_token);
  const sub = await submit(tokens.access_token, problem.id, 'cpp-gpp-64', HELLO_CPP);
  const graded = await pollForVerdict(tokens.access_token, sub.id, { timeoutMs: 120_000 });

  // Independent confirmation the verdict is readable from the DB, not cached.
  const reread = await getSubmission(tokens.access_token, sub.id);
  expect(graded.status.toLowerCase()).toBe('ac');
  expect(reread.status.toLowerCase()).toBe('ac');
});
