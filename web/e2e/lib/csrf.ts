/**
 * CSRF priming for the e2e suite.
 *
 * The API rejects unauthenticated writes without a double-submit cookie:
 * internal/api/middleware/csrf.go mints the `csrf` cookie on any safe request
 * and requires a matching `X-CSRF-Token` otherwise. Bearer-authenticated calls
 * bypass the check.
 *
 * Node's fetch has no cookie jar, so the suite keeps the value itself: one
 * priming GET against /api/auth/csrf, then echo the token in both `Cookie` and
 * `X-CSRF-Token` on every non-GET.
 *
 * Lives in its own module because both lib/api.ts and lib/mail.ts need it, and
 * api.ts already imports mail.ts — sharing it from either would be circular.
 */

const BASE = (process.env.E2E_BASE_URL ?? 'http://localhost:8081').replace(/\/$/, '');

let csrfToken: string | null = null;
let csrfPrimed: Promise<string> | null = null;

export async function ensureCsrf(): Promise<string> {
  if (csrfToken) return csrfToken;
  if (!csrfPrimed) {
    csrfPrimed = fetch(`${BASE}/api/auth/csrf`)
      .then((res) => {
        const cookies = res.headers.getSetCookie?.() ?? [];
        for (const c of cookies) {
          const m = c.match(/(?:^|;\s*)csrf=([^;]+)/);
          if (m) csrfToken = decodeURIComponent(m[1]);
        }
        if (!csrfToken) {
          throw new Error(
            'GET /api/auth/csrf did not set a csrf cookie — is the backend running the build with the CSRF middleware?',
          );
        }
        return csrfToken;
      })
      .finally(() => {
        csrfPrimed = null;
      });
  }
  return csrfPrimed;
}

/** Headers that satisfy the double-submit check on a state-changing request. */
export async function csrfHeaders(): Promise<Record<string, string>> {
  const tok = await ensureCsrf();
  return { Cookie: `csrf=${tok}`, 'X-CSRF-Token': tok };
}
