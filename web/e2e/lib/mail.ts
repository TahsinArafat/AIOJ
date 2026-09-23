/**
 * Test inbox for the sim stack.
 *
 * The sim runs the backend with MAIL_DRIVER=catcher, so every message the
 * product "sends" is buffered in memory and served by the dev-only
 * `GET /api/dev/mail` endpoint (`DELETE` clears it). That endpoint is the
 * injection point the completion spec calls a "mail-api injected test inbox".
 *
 * The important property: the link used by the golden path is the one the
 * BACKEND actually built from its own template and public URL. A test that
 * minted its own token would keep passing while the shipped email was broken —
 * a wrong MAIL_PUBLIC_URL or a renamed route would go unnoticed. Reading the
 * delivered body instead makes template regressions fail the suite.
 */

import { csrfHeaders } from './csrf';

const BASE = (process.env.E2E_BASE_URL ?? 'http://localhost:8081').replace(/\/$/, '');

/** Shape of one buffered message. Field names follow internal/mail.Message. */
export interface InboxMessage {
  From?: string;
  To?: string[];
  Subject?: string;
  Body?: string;
}

export type LinkKind = 'verify-email' | 'reset-password';

async function listInbox(): Promise<InboxMessage[]> {
  const res = await fetch(`${BASE}/api/dev/mail`);
  if (res.status === 404) {
    throw new Error(
      'GET /api/dev/mail returned 404 — the backend is not running with MAIL_DRIVER=catcher. ' +
      'The sim stack should set MAIL_DRIVER=catcher (see docker-compose.yml).',
    );
  }
  if (!res.ok) throw new Error(`list inbox failed: ${res.status}`);
  const json = (await res.json()) as { data?: InboxMessage[] };
  return json.data ?? [];
}

/** Drop every buffered message so an assertion only sees mail it caused. */
export async function clearInbox(): Promise<void> {
  // DELETE is state-changing and unauthenticated, so it needs the double-submit pair.
  const res = await fetch(`${BASE}/api/dev/mail`, {
    method: 'DELETE',
    headers: await csrfHeaders(),
  });
  if (!res.ok && res.status !== 404) {
    throw new Error(`clear inbox failed: ${res.status}`);
  }
}

/** Messages addressed to `email`, oldest first. */
export async function messagesFor(email: string): Promise<InboxMessage[]> {
  const want = email.toLowerCase();
  const all = await listInbox();
  return all.filter((m) => (m.To ?? []).some((to) => to.toLowerCase() === want));
}

/**
 * Pull the first absolute link of `kind` out of a message body.
 *
 * Tolerant of the two shapes the backend can produce: a full absolute URL
 * (http://localhost:8081/verify-email?token=…) or a bare path.
 */
export function extractLink(body: string, kind: LinkKind): string | null {
  const match = body.match(new RegExp(`https?://[^\\s"'<>]*${kind}\\?token=[0-9a-zA-Z]+`));
  if (match) return match[0];
  const bare = body.match(new RegExp(`/${kind}\\?token=[0-9a-zA-Z]+`));
  return bare ? bare[0] : null;
}

/**
 * Wait until `email` receives a `kind` link, then return it.
 *
 * Polls rather than reading once: the mail send is fire-and-forget in the
 * handler (`_ = h.mail.Send(...)`), so the message is not guaranteed to be in
 * the buffer the instant the HTTP response returns.
 */
export async function waitForLink(
  email: string,
  kind: LinkKind,
  opts: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<string> {
  const timeoutMs = opts.timeoutMs ?? 15_000;
  const intervalMs = opts.intervalMs ?? 250;
  const deadline = Date.now() + timeoutMs;
  let seen = 0;

  for (; ;) {
    const msgs = await messagesFor(email);
    // Only consider messages newer than the ones already inspected, so a
    // re-verify of the same address cannot resolve to a consumed token.
    for (let i = seen; i < msgs.length; i++) {
      const link = extractLink(msgs[i].Body ?? '', kind);
      if (link) return link;
    }
    seen = msgs.length;
    if (Date.now() > deadline) {
      throw new Error(
        `no ${kind} email for ${email} within ${timeoutMs}ms (saw ${msgs.length} message(s))`,
      );
    }
    await new Promise((r) => setTimeout(r, intervalMs));
  }
}

/** The raw token embedded in a `kind` link. */
export function tokenFromLink(link: string): string {
  const token = new URL(link, BASE).searchParams.get('token');
  if (!token) throw new Error(`no token in link: ${link}`);
  return token;
}

/**
 * Consume a `kind` link straight against the API.
 *
 * This is the non-browser path used by helpers that only need a usable
 * account; the golden path uses the browser instead so the delivered link is
 * exercised through the UI a real user lands on.
 */
export async function consumeLinkViaApi(link: string, kind: LinkKind): Promise<void> {
  const token = tokenFromLink(link);
  const res = await fetch(`${BASE}/api/auth/${kind}/${token}`);
  if (!res.ok) {
    throw new Error(`consume ${kind} link failed: ${res.status} ${await res.text()}`);
  }
}

/** Deliver → read → consume, in one step. */
export async function verifyEmailViaApi(email: string): Promise<string> {
  const link = await waitForLink(email, 'verify-email');
  await consumeLinkViaApi(link, 'verify-email');
  return link;
}
