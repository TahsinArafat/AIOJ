/**
 * Typed helpers for the real AIOJ API.
 *
 * Every e2e spec uses these instead of hand-rolling `fetch` so the golden path
 * asserts against the backend's actual contract (not the DOM's interpretation).
 *
 * All routes are served by nginx on the sim origin and proxied to the backend.
 */

import { verifyEmailViaApi } from './mail';
import { csrfHeaders } from './csrf';

const BASE = (process.env.E2E_BASE_URL ?? 'http://localhost:8081').replace(/\/$/, '');

/** Shared terminal verdicts — anything else means the decider is still working. */
export const TERMINAL_VERDICTS = ['ac', 'wa', 'tle', 'mle', 're', 'ce', 'pe', 'ole'] as const;
export type Verdict = (typeof TERMINAL_VERDICTS)[number];

const SEED_ADMIN = process.env.SEED_ADMIN_USERNAME ?? 'ai';
const SEED_PASS = process.env.SEED_ADMIN_PASSWORD ?? 'Aioj-Sim-Admin-2026!';

/** A username unique enough that repeated suite runs never collide. */
export function uniq(prefix = 'sim'): string {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

export interface Tokens {
  access_token: string;
  refresh_token: string;
  user: { id: string; username: string; role: string };
}

async function send<T>(
  method: string,
  path: string,
  {
    token,
    body,
    form,
  }: { token?: string; body?: unknown; form?: FormData } = {},
): Promise<{ status: number; data: T }> {
  const init: RequestInit = { method };
  const headers: Record<string, string> = {};

  if (token) headers['Authorization'] = `Bearer ${token}`;
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  } else if (form !== undefined) {
    init.body = form;
  }
  // Unauthenticated writes need the double-submit pair; Bearer calls bypass CSRF.
  if (method !== 'GET' && method !== 'HEAD' && !token) {
    Object.assign(headers, await csrfHeaders());
  }
  init.headers = headers;

  const res = await fetch(`${BASE}${path}`, init);
  const text = await res.text();
  let data: unknown = text;
  const ct = res.headers.get('content-type') ?? '';
  if (ct.includes('application/json') && text) {
    try {
      data = JSON.parse(text);
    } catch {
      /* keep raw text */
    }
  }
  return { status: res.status, data: data as T };
}

// --- health ---------------------------------------------------------------

export async function health(): Promise<{ status: string }> {
  const r = await send<{ status: string }>('GET', '/api/health');
  if (r.status !== 200) throw new Error(`health failed: ${r.status} ${JSON.stringify(r.data)}`);
  return r.data;
}

// --- auth -----------------------------------------------------------------

export async function registerRaw(
  username: string,
  email: string,
  password: string,
): Promise<Tokens> {
  const r = await send<Tokens>('POST', '/api/auth/register', {
    body: { username, email, password },
  });
  if (r.status !== 201 && r.status !== 200) {
    throw new Error(`register failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data;
}

/**
 * Register AND verify, for helpers that just need a usable account.
 *
 * Submissions require a verified email (handler/submission.go), so any spec
 * that registers a throwaway user needs this rather than registerRaw.
 * Prefer the golden path when the mail leg itself is under test.
 */
export async function register(
  username: string,
  email: string,
  password: string,
): Promise<Tokens> {
  const tokens = await registerRaw(username, email, password);
  await verifyEmailViaApi(email);
  return tokens;
}

// The inbox client lives in ./mail; re-exported here so specs have one import.
export { clearInbox, messagesFor, verifyEmailViaApi, waitForLink } from './mail';

export async function login(username: string, password: string): Promise<Tokens> {
  const r = await send<Tokens>('POST', '/api/auth/login', {
    body: { username, password },
  });
  if (r.status !== 200) {
    throw new Error(`login failed for "${username}": ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data;
}

/** Log in as the admin the seeder created (role=admin is promoted by the seeder). */
export function loginAsSeedAdmin(): Promise<Tokens> {
  return login(SEED_ADMIN, SEED_PASS);
}

// --- problems -------------------------------------------------------------

export interface Problem {
  id: string;
  slug: string;
  title: string;
}

export async function getProblemBySlug(slug: string, token?: string): Promise<Problem> {
  const r = await send<Problem>('GET', `/api/problems/${slug}`, { token });
  if (r.status !== 200) {
    throw new Error(`get problem "${slug}" failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data;
}

// --- submissions ----------------------------------------------------------

export interface Submission {
  id: string;
  status: string;
  verdict?: string;
  time_used?: number;
  memory_used?: number;
  score?: number;
}

export const HELLO_CPP = [
  '#include <iostream>',
  'int main() {',
  '    std::cout << "Hello, AIOJ!" << "\\n";',
  '}',
].join('\n');

export async function submit(
  token: string,
  problemId: string,
  language = 'cpp-gpp-64',
  sourceCode = HELLO_CPP,
  contestId?: string,
): Promise<Submission> {
  const r = await send<Submission>('POST', '/api/submissions', {
    token,
    body: { problem_id: problemId, language, source_code: sourceCode, contest_id: contestId },
  });
  if (r.status !== 201 && r.status !== 200) {
    throw new Error(`submit failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data;
}

export async function getSubmission(token: string, id: string): Promise<Submission> {
  const r = await send<Submission>('GET', `/api/submissions/${id}`, { token });
  if (r.status !== 200) {
    throw new Error(`get submission "${id}" failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data;
}

/**
 * Poll the real API until the decider reaches a terminal verdict.
 * The worker compiles and runs inside go-judge, so this can take seconds.
 */
export async function pollForVerdict(
  token: string,
  id: string,
  opts: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<Submission> {
  const timeoutMs = opts.timeoutMs ?? 90_000;
  const intervalMs = opts.intervalMs ?? 1_000;
  const deadline = Date.now() + timeoutMs;

  let sub = await getSubmission(token, id);
  while (!TERMINAL_VERDICTS.includes(sub.status.toLowerCase() as Verdict)) {
    if (Date.now() > deadline) {
      throw new Error(`submission ${id} did not reach a verdict in ${timeoutMs}ms (last: ${JSON.stringify(sub)})`);
    }
    await new Promise((r) => setTimeout(r, intervalMs));
    sub = await getSubmission(token, id);
  }
  return sub;
}

// --- contests -------------------------------------------------------------

export interface Contest {
  id: string;
  title: string;
  slug?: string;
  format?: string;
  description?: string;
  visible: boolean;
}

export interface ScoreboardEntry {
  rank: number;
  user_id: string;
  username: string;
  total_solved: number;
  total_penalty: number;
  total_score: number;
  problems: Record<string, { solved: boolean; attempts: number; time: number; score: number }>;
}

export interface Scoreboard {
  entries: ScoreboardEntry[];
  problems: Array<{ problem_id: string; index: number }>;
}

/**
 * Create a running ACM contest over the given problems.
 *
 * `start_time` is backdated and `end_time` is in the future so the contest is
 * live for the duration of the test: handler/submission.go rejects submissions
 * outside the window, which would otherwise make this spec time-of-day flaky.
 */
export async function createContest(
  token: string,
  opts: { title: string; problemIds: string[]; format?: string },
): Promise<Contest> {
  const now = Date.now();
  const r = await send<Contest>('POST', '/api/contests', {
    token,
    body: {
      title: opts.title,
      type: 'acm',
      format: opts.format ?? 'acm',
      start_time: new Date(now - 60 * 60 * 1000).toISOString(),
      end_time: new Date(now + 2 * 60 * 60 * 1000).toISOString(),
      visible: true,
      // Without this the registration handler answers 400 "registration not
      // required for this contest" — participants are only tracked when a
      // contest opts into registration.
      registration_required: true,
      problem_ids: opts.problemIds,
    },
  });
  if (r.status !== 201 && r.status !== 200) {
    throw new Error(`create contest failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data;
}

/** Fetch the contest representation returned by the public management API. */
export async function getContest(contestId: string, token?: string): Promise<Contest> {
  const r = await send<{ contest: Contest }>('GET', `/api/contests/${contestId}`, { token });
  if (r.status !== 200) {
    throw new Error(`get contest failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data.contest;
}

/** Grant a contest role to a user. */
export async function addContestPermission(
  token: string,
  contestId: string,
  userId: string,
  accessLevel: 'manager' | 'judge' | 'tester',
): Promise<void> {
  const r = await send<unknown>('POST', `/api/contests/${contestId}/permissions`, {
    token,
    body: { user_id: userId, access_level: accessLevel },
  });
  if (r.status !== 200 && r.status !== 201) {
    throw new Error(`add contest permission failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
}

export interface ContestUpdateResult {
  status: number;
  data: unknown;
}

/**
 * Update contest settings and return the HTTP status so tests can assert both
 * successful manager writes and forbidden non-manager writes through one seam.
 */
export async function updateContest(
  token: string,
  contestId: string,
  changes: { title?: string; description?: string; visible?: boolean },
): Promise<ContestUpdateResult> {
  return send<unknown>('PUT', `/api/contests/${contestId}`, {
    token,
    body: changes,
  });
}

/** Sign a user up for a contest (POST /api/contests/{id}/register). */
export async function registerForContest(token: string, contestId: string): Promise<void> {
  const r = await send<{ registered: boolean }>('POST', `/api/contests/${contestId}/register`, {
    token,
  });
  if (r.status !== 200 && r.status !== 201) {
    throw new Error(`register for contest failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
}

export async function getScoreboard(contestId: string, token?: string): Promise<Scoreboard> {
  const r = await send<Scoreboard>('GET', `/api/contests/${contestId}/scoreboard`, { token });
  if (r.status !== 200) {
    throw new Error(`scoreboard failed: ${r.status} ${JSON.stringify(r.data)}`);
  }
  return r.data;
}
