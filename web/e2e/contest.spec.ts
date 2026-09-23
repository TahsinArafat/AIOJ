import { test, expect } from '@playwright/test';
import {
  HELLO_CPP,
  createContest,
  addContestPermission,
  getContest,
  getProblemBySlug,
  getScoreboard,
  loginAsSeedAdmin,
  pollForVerdict,
  register,
  registerForContest,
  submit,
  uniq,
  updateContest,
} from './lib/api';

/**
 * DoD 4: contest create / run / scoreboard driven through the real API.
 *
 * Before this file the suite had zero references to contests, so the contest
 * paths were entirely unproven by Playwright even though the UI and handlers
 * exist. Everything here goes over HTTP against the sim stack — no DB writes,
 * no fixtures — so it exercises the same surface a user does.
 *
 * The scoreboard assertion is deliberately about *computed standings*: the
 * entries must come back ranked with total_solved=1 for both contestants, which
 * only holds if the contest's problem_ids reached the format engine and the
 * real verdicts were read back.
 */

const SEED_SLUG = process.env.SEED_PROBLEM_SLUG ?? 'hello';

test('contest: create → register → submit → scoreboard reflects real verdicts', async () => {
  // The seeded admin is the only role allowed to create a contest.
  const admin = await loginAsSeedAdmin();
  const problem = await getProblemBySlug(SEED_SLUG, admin.access_token);

  const contest = await createContest(admin.access_token, {
    title: `Sim Contest ${uniq('c')}`,
    problemIds: [problem.id],
  });
  expect(contest.id).toBeTruthy();

  // Two real accounts. register() verifies each one, because submissions are
  // gated on a verified email.
  const alice = uniq('alice');
  const bob = uniq('bob');
  const aliceTokens = await register(alice, `${alice}@aioj.test`, 'Aioj-Sim-2026!');
  const bobTokens = await register(bob, `${bob}@aioj.test`, 'Aioj-Sim-2026!');

  await registerForContest(aliceTokens.access_token, contest.id);
  await registerForContest(bobTokens.access_token, contest.id);

  // Both solve the single contest problem through the real judge pipeline.
  for (const tokens of [aliceTokens, bobTokens]) {
    const sub = await submit(
      tokens.access_token,
      problem.id,
      'cpp-gpp-64',
      HELLO_CPP,
      contest.id,
    );
    const graded = await pollForVerdict(tokens.access_token, sub.id, { timeoutMs: 120_000 });
    expect(graded.status.toLowerCase(), `submission ${sub.id} should be AC`).toBe('ac');
  }

  // Standings are computed from the contest's problem_ids, not hardcoded.
  const board = await getScoreboard(contest.id);
  expect(board.problems.length).toBe(1);
  expect(board.problems[0].problem_id).toBe(problem.id);

  const byUsername = new Map(board.entries.map((e) => [e.username, e]));
  expect([...byUsername.keys()].sort()).toEqual([alice, bob].sort());

  for (const name of [alice, bob]) {
    const entry = byUsername.get(name)!;
    expect(entry.total_solved, `${name} should have solved the one contest problem`).toBe(1);
  }

  // ACM ranks on solved-then-penalty. Both solved exactly one problem, so they
  // must share a rank — a scoreboard that ranked them differently would mean
  // the penalty or ordering path is not reading the real submissions.
  expect(byUsername.get(alice)!.rank).toBe(byUsername.get(bob)!.rank);
});

/**
 * A wrong answer must not count as solved. Without this, the spec above would
 * still pass if the scoreboard credited every registered participant
 * regardless of verdict.
 */
test('contest scoreboard: a wrong answer does not count as solved', async () => {
  const admin = await loginAsSeedAdmin();
  const problem = await getProblemBySlug(SEED_SLUG, admin.access_token);
  const contest = await createContest(admin.access_token, {
    title: `Sim Contest ${uniq('c')}`,
    problemIds: [problem.id],
  });

  const carol = uniq('carol');
  const tokens = await register(carol, `${carol}@aioj.test`, 'Aioj-Sim-2026!');
  await registerForContest(tokens.access_token, contest.id);

  const sub = await submit(
    tokens.access_token,
    problem.id,
    'cpp-gpp-64',
    '#include <iostream>\nint main(){ std::cout << "wrong\\n"; }\n',
    contest.id,
  );
  const graded = await pollForVerdict(tokens.access_token, sub.id, { timeoutMs: 120_000 });
  expect(graded.status.toLowerCase()).not.toBe('ac');

  const board = await getScoreboard(contest.id);
  const entry = board.entries.find((e) => e.username === carol);
  expect(entry, 'the contestant should appear in standings').toBeTruthy();
  expect(entry!.total_solved).toBe(0);
});

test('contest management: a manager can update settings while a participant cannot', async () => {
  const admin = await loginAsSeedAdmin();
  const problem = await getProblemBySlug(SEED_SLUG, admin.access_token);
  const contest = await createContest(admin.access_token, {
    title: `Sim Management ${uniq('c')}`,
    problemIds: [problem.id],
  });

  const managerName = uniq('manager');
  const outsiderName = uniq('outsider');
  const manager = await register(managerName, `${managerName}@aioj.test`, 'Aioj-Sim-2026!');
  const outsider = await register(outsiderName, `${outsiderName}@aioj.test`, 'Aioj-Sim-2026!');

  await addContestPermission(admin.access_token, contest.id, manager.user.id, 'manager');

  const title = `Managed ${uniq('title')}`;
  const description = 'Updated through the real contest management API.';
  const updated = await updateContest(manager.access_token, contest.id, {
    title,
    description,
    visible: true,
  });
  expect(updated.status).toBe(200);

  // The change is readable through the ordinary public contest endpoint, not
  // only through the manager's write response.
  const publicContest = await getContest(contest.id);
  expect(publicContest.title).toBe(title);
  expect(publicContest.description).toBe(description);
  expect(publicContest.visible).toBe(true);

  const forbidden = await updateContest(outsider.access_token, contest.id, {
    title: 'Unauthorized update',
  });
  expect(forbidden.status).toBe(403);
});
