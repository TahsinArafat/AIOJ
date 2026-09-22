/**
 * Typed helpers for the real AIOJ API.
 *
 * Every e2e spec uses these instead of hand-rolling `fetch` so the golden path
 * asserts against the backend's actual contract (not the DOM's interpretation).
 *
 * All routes are served by nginx on the sim origin and proxied to the backend.
 */

const BASE = (process.env.E2E_BASE_URL ?? 'http://localhost:8081').replace(/\/$/, '');

/** Shared terminal verdicts — anything else means the decider is still working. */
export const TERMINAL_VERDICTS = ['ac', 'wa', 'tle', 'mle', 're', 'ce', 'pe', 'ole'] as const;
export type Verdict = (typeof TERMINAL_VERDICTS)[number];

const SEED_ADMIN = process.env.SEED_ADMIN_USERNAME ?? 'ai';
const SEED_PASS = process.env.SEED_ADMIN_PASSWORD ?? 'aiseedpass';

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

export async function register(
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
  await verifyEmailFromCatcher(email);
  return r.data;
}

/**
 * Pull the verification link from the in-memory mailcatcher and call the
 * verify endpoint so new e2e users can submit (submissions require a
 * verified email).
 */
export async function verifyEmailFromCatcher(email: string): Promise<void> {
  const list = await send<{ data?: Array<{ To?: string[]; to?: string[]; Body?: string; body?: string; Subject?: string }> }>(
    'GET',
    '/api/dev/mail',
  );
  if (list.status !== 200) {
    // catcher disabled — leave the user unverified (submit will 403)
    return;
  }
  const msgs = list.data?.data ?? [];
  const mine = msgs.filter((m) => {
    const to = (m.To ?? m.to ?? []).map((s) => s.toLowerCase());
    return to.includes(email.toLowerCase());
  });
  const last = mine[mine.length - 1];
  if (!last) return;
  const body = last.Body ?? last.body ?? '';
  const match = body.match(/verify-email\?token=([0-9a-f]+)/i);
  if (!match) return;
  await send('GET', `/api/auth/verify-email/${match[1]}`);
}

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
): Promise<Submission> {
  const r = await send<Submission>('POST', '/api/submissions', {
    token,
    body: { problem_id: problemId, language, source_code: sourceCode },
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
