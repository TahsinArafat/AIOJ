const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

/** Returns true if the string looks like a UUID — auto-generated slugs that aren't "real". */
export function isUuid(v: string): boolean {
    return UUID_RE.test(v)
}

/** Get the best human-readable contest identifier for URLs: real slug > display_id > UUID id */
export function contestSlug(c: { slug?: string; display_id?: number; id: string }): string {
    if (c.slug && !isUuid(c.slug)) return c.slug
    if (c.display_id) return String(c.display_id)
    return c.id
}

export interface ProblemDetail {
    id: string
    title: string
    slug: string
    description: string
    input_format?: string
    output_format?: string
    hint?: string
    difficulty: string
    time_limit: number
    memory_limit: number
    tags?: string[]
    source?: string
    interactive?: boolean
    scoring_mode?: string
    problem_type?: string
    sample_cases?: { input: string; output: string; explanation?: string }[]
}

const BASE = '/api'

let accessToken: string | null = localStorage.getItem('access_token')
let refreshToken: string | null = localStorage.getItem('refresh_token')

/**
 * Fired whenever the session starts or ends.
 *
 * Feature verdicts depend on who is signed in (role and rating), so the
 * features context has to refetch when the session changes. A DOM event keeps
 * this module free of any React dependency.
 */
export const AUTH_CHANGE_EVENT = 'aioj:auth-change'

function notifyAuthChange() {
    window.dispatchEvent(new Event(AUTH_CHANGE_EVENT))
}

export function setTokens(a: string, r: string) {
    accessToken = a
    refreshToken = r
    localStorage.setItem('access_token', a)
    localStorage.setItem('refresh_token', r)
    notifyAuthChange()
}

export function clearTokens() {
    const wasSignedIn = accessToken !== null || refreshToken !== null
    accessToken = null
    refreshToken = null
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
    if (wasSignedIn) notifyAuthChange()
}

function readStoredRefreshToken(): string | null {
    return localStorage.getItem('refresh_token')
}

/**
 * Keep this tab's copy of the tokens in step with every other tab.
 *
 * Refresh tokens rotate: the tab that refreshes replaces the stored token, and
 * every other tab is still holding the copy that was just spent. Without this
 * listener those tabs sit on a dead token and present it at the next 401 —
 * which the server rejects, and which used to sign the whole browser out.
 */
if (typeof window !== 'undefined') {
    window.addEventListener('storage', event => {
        // key is null when another tab calls localStorage.clear().
        if (event.key === null || event.key === 'access_token' || event.key === 'refresh_token') {
            const wasSignedIn = accessToken !== null
            accessToken = localStorage.getItem('access_token')
            refreshToken = localStorage.getItem('refresh_token')
            if ((accessToken !== null) !== wasSignedIn) notifyAuthChange()
        }
    })
}

/**
 * End the session on both sides.
 *
 * Clearing localStorage alone left the refresh token valid on the server for
 * its full lifetime (168h), so a copy of it kept working after "logout". The
 * local state is dropped first so sign-out is instant and works offline; the
 * server call is best effort.
 */
export async function signOut(): Promise<void> {
    const token = refreshToken
    clearTokens()
    if (!token) return
    try {
        await fetch(BASE + '/auth/logout', {
            method: 'POST',
            credentials: 'same-origin',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': await ensureCsrfToken(),
            },
            body: JSON.stringify({ refresh_token: token }),
        })
    } catch {
        // Offline or server down: the token still expires on its own.
    }
}

export function getAccessToken(): string | null { return accessToken }

/**
 * Claims of the session token, or null when signed out.
 *
 * Reads the stored token as a fallback so a fresh page load (before this
 * module's state is populated) still knows who is signed in. Never throws: a
 * truncated or corrupted token must degrade to "signed out", not blank a page.
 */
export function currentUser(): { id: string; username: string; role: string } | null {
    const token = accessToken ?? localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        if (typeof payload?.uid !== 'string') return null
        return {
            id: payload.uid,
            username: typeof payload.uname === 'string' ? payload.uname : '',
            role: typeof payload.role === 'string' ? payload.role : '',
        }
    } catch {
        return null
    }
}

function getCsrfToken(): string {
    const m = document.cookie.match(/(?:^|;\s*)csrf=([^;]+)/)
    return m ? decodeURIComponent(m[1]) : ''
}

let csrfPrimed: Promise<void> | null = null

/**
 * Ensure the server has minted a csrf cookie, and return its value.
 *
 * The cookie is only issued in response to a safe request, and the SPA is
 * served by nginx — so a cold visitor who lands directly on /register has made
 * no /api/* call and holds no cookie. Every unauthenticated write (register,
 * login, forgot-password, 2FA verify) was therefore rejected with a raw
 * "csrf cookie missing" before this existed. Prime explicitly instead.
 *
 * Concurrent callers share one in-flight request, so a page that fires several
 * mutations at once does not stampede the endpoint.
 */
async function ensureCsrfToken(): Promise<string> {
    const existing = getCsrfToken()
    if (existing) return existing
    if (!csrfPrimed) {
        csrfPrimed = fetch(BASE + '/auth/csrf', { credentials: 'same-origin' })
            .then(() => undefined)
            .finally(() => { csrfPrimed = null })
    }
    await csrfPrimed
    return getCsrfToken()
}

// ── Token refresh (single-flight) ────────────────────────────────────────────
//
// Several in-flight requests can 401 at the same time (the notification bell
// polls, plus any page load firing 3-4 reads). Each one used to POST
// /auth/refresh independently, and because the server rotates refresh tokens
// the later call presented an already-consumed token and logged the user out.
// One shared promise means concurrent 401s all await the same refresh.
//
// The outcome is three-valued because "could not ask" and "the token is dead"
// demand opposite reactions: only the second may end the session. Treating a
// failed request as a rejection is what used to sign users out on a deploy, a
// restarted database or a dropped connection.
type RefreshOutcome = 'ok' | 'rejected' | 'unavailable'

// A spent token is not instantly proof the session is over. Refresh tokens
// rotate and localStorage is shared by every tab, so another tab may have
// spent this one moments ago and simply not have stored the replacement yet.
// Re-reading storage across a short backoff separates that race from a dead
// session. The wait stays under a second so a genuine expiry still ends
// promptly instead of hanging the request.
const REFRESH_REREAD_DELAYS_MS = [150, 350]

function delay(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms))
}

let refreshInFlight: Promise<RefreshOutcome> | null = null

async function refreshAccessToken(): Promise<RefreshOutcome> {
    if (refreshInFlight) return refreshInFlight

    refreshInFlight = (async () => {
        try {
            const csrf = await ensureCsrfToken()
            for (let attempt = 0; attempt <= REFRESH_REREAD_DELAYS_MS.length; attempt++) {
                // Two passes over the candidates. The first offers the token
                // stored right now (freshest across tabs), then the copy this
                // tab holds in memory. The second re-reads storage: another tab
                // may have rotated the token while the first pass was in
                // flight, and adopting its result is the difference between
                // continuing and a bogus sign-out.
                const attempted = new Set<string>()
                let tried = 0
                let sawSpentToken = false
                for (let pass = 0; pass < 2; pass++) {
                    for (const token of [readStoredRefreshToken(), refreshToken]) {
                        if (!token || attempted.has(token)) continue
                        attempted.add(token)
                        tried++
                        let res: Response
                        try {
                            res = await fetch(BASE + '/auth/refresh', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
                                body: JSON.stringify({ refresh_token: token }),
                            })
                        } catch {
                            // The server never answered, so nothing at all is
                            // known about the token.
                            return 'unavailable'
                        }
                        if (res.ok) {
                            const data = await res.json()
                            setTokens(data.access_token, data.refresh_token)
                            return 'ok'
                        }
                        if (res.status === 401) {
                            sawSpentToken = true
                            continue
                        }
                        // 403 is a policy answer (a suspended account). Every
                        // other status is the server failing, not the session.
                        return res.status === 403 ? 'rejected' : 'unavailable'
                    }
                }
                if (tried === 0) return 'rejected'
                if (!sawSpentToken) return 'unavailable'
                if (attempt === REFRESH_REREAD_DELAYS_MS.length) return 'rejected'
                // Every candidate was spent. Another tab may be mid-rotation and
                // about to store the replacement, so re-read before giving up.
                await delay(REFRESH_REREAD_DELAYS_MS[attempt])
            }
            return 'rejected'
        } catch {
            return 'unavailable'
        } finally {
            // Cleared in a microtask so callers awaiting this promise all see
            // the same result before a new refresh can start.
            queueMicrotask(() => { refreshInFlight = null })
        }
    })()

    return refreshInFlight
}

/**
 * Send one request with the bearer token and CSRF cookie attached, refreshing
 * once on 401.
 *
 * Exported to the module (not the public `api` object) so every caller shares
 * one auth path. The uploads and remote-import endpoints used to open-code
 * `fetch` with `if (token) headers['Authorization'] = ...`, which meant an
 * expired access token got no refresh retry — and because the CSRF middleware
 * only skips its check for a *present* Authorization header, the request came
 * back 403 "csrf cookie missing" instead of a 401 the client could recover
 * from. Every caller must go through here.
 */
async function fetchAuthed(path: string, opts: RequestInit = {}): Promise<Response> {
    const method = (opts.method || 'GET').toUpperCase()
    const headers: Record<string, string> = {
        ...(opts.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
        ...(opts.headers as Record<string, string> || {}),
    }
    if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`
    // Echo double-submit cookie for non-Bearer mutations (login/register/refresh).
    if (method !== 'GET' && method !== 'HEAD' && !headers['Authorization']) {
        headers['X-CSRF-Token'] = await ensureCsrfToken()
    }

    let res = await fetch(BASE + path, { ...opts, headers })

    if (res.status === 401 && (refreshToken || readStoredRefreshToken())) {
        const outcome = await refreshAccessToken()
        if (outcome === 'ok') {
            headers['Authorization'] = `Bearer ${accessToken}`
            res = await fetch(BASE + path, { ...opts, headers })
        } else if (outcome === 'rejected') {
            clearTokens()
            throw new Error('session expired')
        } else {
            // The session store could not be reached. Keep the tokens: the next
            // request retries, and clearing here would turn a server restart or
            // a database blip into a sign-out.
            throw new Error('connection lost, please try again')
        }
    }
    return res
}

/**
 * Turn a failed response into the Error the UI shows.
 *
 * Error bodies are usually JSON ({"error": "...", "code": "..."}); surface the
 * message rather than the raw JSON blob, and keep the code so callers can react
 * to password_change_required.
 */
async function throwForStatus(res: Response): Promise<never> {
    const text = await res.text()
    let message = text
    let code = ''
    let scoreboardPublic = false
    try {
        const parsed = JSON.parse(text)
        if (parsed && typeof parsed === 'object') {
            message = parsed.error || parsed.message || text
            code = parsed.code || ''
            // Carried for one case: a password-protected round that publishes
            // its board. Such a round is unlisted, so the locked-contest screen
            // is the only place a spectator can be handed the way in.
            scoreboardPublic = parsed.scoreboard_public === 'true'
        }
    } catch {
        // Plain-text error: keep it as-is.
    }
    const err = new Error(message || `HTTP ${res.status}`)
    if (code) (err as Error & { code?: string }).code = code
    if (scoreboardPublic) (err as Error & { scoreboardPublic?: boolean }).scoreboardPublic = true
    throw err
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
    const res = await fetchAuthed(path, opts)
    if (!res.ok) await throwForStatus(res)

    const contentType = res.headers.get('content-type') || ''
    if (res.status === 204 || contentType.includes('text/plain')) {
        return null as unknown as T
    }
    const text = await res.text()
    if (!text) return null as unknown as T
    try {
        return JSON.parse(text)
    } catch {
        return text as unknown as T
    }
}

/** Same auth and refresh path as request(), for endpoints that return bytes. */
async function requestBlob(path: string, opts: RequestInit = {}): Promise<Blob> {
    const res = await fetchAuthed(path, opts)
    if (!res.ok) await throwForStatus(res)
    return res.blob()
}

import type { AIBrief, ContestAdjustment, AIBriefPreset, AIGenerationJob, AIModel, APIKey, ActivityItem, AdminBulkAction, AdminBulkResponse, AdminFeature, AdminUserDetail, AdminUserQuery, AdminAuthMethods, AdminUserRow, AuthMethods, AuthResponse, AuthSessionResponse, BackupFile, BlogListItem, BlogPost, BotAccount, Clarification, Class, ClassListItem, ClassMember, Comment, Contest, ContestDetailResponse, ContestNotice, ContestPermission, ContestProblem, ContestRegistration, ContestSubmissionsResponse, DetectedTool, Editorial, FeaturePolicy, Group, GroupListItem, GroupMember, GroupPendingInvite, GymContest, GymData, Hack, HackResult, HackSubmission, LanguageConfig, Notification, NotificationPreferences, OAuthUsernameCheck, OnsiteBatchUser, Organization, OrganizationListItem, OrganizationMember, PendingInvite, PendingRemoteSubmission, PlagiarismPairDetail, PlagiarismReport, Problem, ProblemI18n, ProblemListItem, ProblemPermission, ProblemStats, PublicLanguage, PublicProfile, RankingEntry, RatingHistoryEntry, RemoteLanguage, ResolvedFeature, ScoreboardData, SetterApplication, Submission, Team, TeamListItem, TeamMember, TestcaseFile, TestcaseFileContent, TestResult, TrainingPlan, TrainingPlanDetail, TrainingPlanEnrollment, TrainingPlanProblem, TrainingPlanProgress, TrainingPlanSection, User, UserAchievement, UserProblemStats, UserProfile, VirtualContest, VirtualStatusResponse, WeakTagsRecommendations, Webhook } from '../types/api'

export const api = {
    auth: {
        /**
         * Which sign-in methods this instance offers. Public: the login and
         * register forms need it before there is a session.
         */
        methods: () => request<{ data: AuthMethods }>('/auth/methods'),
        register: (d: { username: string; email: string; password: string }) =>
            request<AuthSessionResponse>('/auth/register', {
                method: 'POST',
                body: JSON.stringify(d),
            }),
        login: (d: { username: string; password: string }) =>
            request<AuthResponse>('/auth/login', {
                method: 'POST',
                body: JSON.stringify(d),
            }),
        verify2FA: (challenge_id: string, code: string) =>
            request<AuthSessionResponse>(
                '/auth/2fa/verify',
                { method: 'POST', body: JSON.stringify({ challenge_id, code }) },
            ),
        forgotPassword: (d: { email: string }) =>
            request<{ message: string; token?: string }>('/auth/forgot-password', {
                method: 'POST',
                body: JSON.stringify(d),
            }),
        resetPassword: (d: { token: string; new_password: string }) =>
            request<{ message: string }>('/auth/reset-password', {
                method: 'POST',
                body: JSON.stringify(d),
            }),
        verifyEmail: (token: string) =>
            request<{ status: string }>(`/auth/verify-email/${encodeURIComponent(token)}`),
        resendVerification: () =>
            request<{ message: string }>('/auth/verify-email/resend', { method: 'POST' }),
        logout: (refresh_token: string) =>
            request<null>('/auth/logout', { method: 'POST', body: JSON.stringify({ refresh_token }) }),
        /**
         * Finish a first-time OAuth sign-up.
         *
         * The callback deliberately issues a signed ticket instead of an
         * account, so the person can pick the handle everyone else will see.
         * This trades that ticket for a real session.
         */
        completeOAuthSignup: (d: { signup_token: string; username: string }) =>
            request<AuthSessionResponse>('/auth/oauth/complete', {
                method: 'POST',
                body: JSON.stringify(d),
            }),
        /** Live availability hint for the sign-up form (needs its ticket). */
        oauthUsernameAvailable: (signup_token: string, username: string) =>
            request<OAuthUsernameCheck>(
                `/auth/oauth/username-available?signup_token=${encodeURIComponent(signup_token)}` +
                    `&username=${encodeURIComponent(username)}`,
            ),
    },
    twoFactor: {
        begin: () =>
            request<{ secret: string; uri: string }>('/auth/2fa/begin', { method: 'POST' }),
        enable: (code: string) =>
            request<{ backup_codes: string[] }>('/auth/2fa/enable', {
                method: 'POST',
                body: JSON.stringify({ code }),
            }),
        disable: (password: string) =>
            request<{ status: string }>('/auth/2fa/disable', {
                method: 'POST',
                body: JSON.stringify({ password }),
            }),
        verify: (body: { challenge_id: string; code: string }) =>
            request<AuthSessionResponse>('/auth/2fa/verify', {
                method: 'POST',
                body: JSON.stringify(body),
            }),
    },
    problems: {
        list: (offset = 0, limit = 20, filters?: { difficulty?: string; tags?: string[]; search?: string; source?: string; rating?: string; sort?: string }) => {
            let url = `/problems?offset=${offset}&limit=${limit}`;
            if (filters?.difficulty) url += `&difficulty=${filters.difficulty}`;
            if (filters?.tags?.length) url += `&tags=${filters.tags.join(',')}`;
            if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
            if (filters?.source) url += `&source=${encodeURIComponent(filters.source)}`;
            // The handler reads `rating`/`sort`; neither used to be sent, so the
            // rating filter and every sort option were silently ignored.
            if (filters?.rating) url += `&rating=${encodeURIComponent(filters.rating)}`;
            if (filters?.sort) url += `&sort=${encodeURIComponent(filters.sort)}`;
            return request<{ data: ProblemListItem[]; total: number }>(url);
        },
        listMy: (offset = 0, limit = 20) => request<{ data: ProblemListItem[]; total: number }>(`/problems/my?offset=${offset}&limit=${limit}`),
        // Publishing is an admin decision, so an author's last step is to submit
        // for review rather than to flip a visible flag.
        submitForReview: (slug: string) =>
            request(`/problems/${slug}/submit`, { method: 'POST' }),
        // Run the problem's validator over its inputs. A validator existed only
        // inside the AI harness before this; a setter can now run it by hand.
        validateCases: (slug: string) =>
            request<{ checked: number; failed: number; cases: { name: string; ok: boolean; message?: string }[] }>(
                `/problems/${slug}/validate-cases`,
                { method: 'POST' },
            ),
        // The contest problem picker: the archive plus the caller's own
        // unpublished drafts, which the archive listing can never return.
        listSelectable: (search: string, offset = 0, limit = 20) => {
            let url = `/problems/selectable?offset=${offset}&limit=${limit}`;
            if (search) url += `&search=${encodeURIComponent(search)}`;
            return request<{ data: ProblemListItem[]; total: number }>(url);
        },
        listTags: () => request<{ data: string[] }>('/problems/tags'),
        get: (slug: string) => request<Problem>(`/problems/${slug}`),
        // Per-problem statement translations. `listI18n` is public; the write
        // methods are used by the setter panel.
        listI18n: (id: string) => request<ProblemI18n[]>(`/problems/${id}/i18n/`),
        upsertI18n: (id: string, lang: string, d: { title?: string; description?: string }) =>
            request<ProblemI18n>(`/problems/${id}/i18n/${lang}`, { method: 'PUT', body: JSON.stringify(d) }),
        deleteI18n: (id: string, lang: string) =>
            request<{ status: string }>(`/problems/${id}/i18n/${lang}`, { method: 'DELETE' }),
        create: (d: unknown) => request<Problem>('/problems', { method: 'POST', body: JSON.stringify(d) }),
        update: (slug: string, d: unknown) => request<Problem>(`/problems/${slug}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (slug: string) => request<{ status: string }>(`/problems/${slug}`, { method: 'DELETE' }),
        getPermissions: (slug: string) => request<{ data: ProblemPermission[] }>(`/problems/${slug}/permissions`),
        /** Setter view of a problem's submissions (names the submitter). */
        submissions: (slug: string, offset = 0, limit = 50, filters?: { handle?: string; status?: string; language?: string; contestId?: string }) => {
            const qs = new URLSearchParams({ offset: String(offset), limit: String(limit) });
            if (filters?.handle) qs.set('handle', filters.handle);
            if (filters?.status) qs.set('status', filters.status);
            if (filters?.language) qs.set('language', filters.language);
            if (filters?.contestId) qs.set('contest_id', filters.contestId);
            return request<{ data: Submission[]; total: number }>(`/problems/${slug}/submissions/manage?${qs.toString()}`);
        },
        /** Rejudge one submission, a whole contest's, or every judged row. */
        rejudgeSubmissions: (slug: string, d: { submission_id?: string; contest_id?: string } = {}) =>
            request<{ enqueued: number; total: number }>(`/problems/${slug}/submissions/rejudge`, {
                method: 'POST',
                body: JSON.stringify(d),
            }),
        /**
         * Void a submission: it stops counting as an attempt and stops scoring.
         * A reason is required — a void nobody can explain later is worse than
         * the submission it removed. Which roles may do this is an admin
         * setting (Admin → Feature Gates), and the caller must also own the
         * problem or run the round.
         */
        skipSubmission: (slug: string, submissionId: string, reason: string) =>
            request<{ id: string; status: string; was: string }>(`/problems/${slug}/submissions/${submissionId}/skip`, {
                method: 'POST',
                body: JSON.stringify({ reason }),
            }),
        addPermission: (slug: string, username: string, accessLevel: string) =>
            request<{ status: string }>(`/problems/${slug}/permissions`, {
                method: 'POST',
                body: JSON.stringify({ username, access_level: accessLevel })
            }),
        removePermission: (slug: string, userId: string) =>
            request<{ status: string }>(`/problems/${slug}/permissions/${userId}`, { method: 'DELETE' }),
        uploadTestcases: async (slug: string, file: File): Promise<unknown> => {
            const formData = new FormData()
            formData.append('file', file)
            return request<unknown>(`/problems/${slug}/testcases`, {
                method: 'POST',
                body: formData,
            })
        },
        /** Every file in the problem's testdata directory, with role and pair. */
        listTestcases: (slug: string) =>
            request<{ files: TestcaseFile[] }>(`/problems/${slug}/testcases`),
        getTestcase: (slug: string, name: string) =>
            request<TestcaseFileContent>(`/problems/${slug}/testcases/${encodeURIComponent(name)}`),
        saveTestcase: (slug: string, name: string, content: string) =>
            request<{ status: string; name: string }>(`/problems/${slug}/testcases/${encodeURIComponent(name)}`, {
                method: 'PUT',
                body: JSON.stringify({ content }),
            }),
        deleteTestcase: (slug: string, name: string) =>
            request<null>(`/problems/${slug}/testcases/${encodeURIComponent(name)}`, { method: 'DELETE' }),
        /** Fetches with the bearer token, then hands the bytes to the browser. */
        downloadTestcase: async (slug: string, name: string) => {
            const blob = await requestBlob(
                `/problems/${slug}/testcases/${encodeURIComponent(name)}?download=1`,
            )
            const url = URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = name
            a.click()
            URL.revokeObjectURL(url)
        },
        importProblem: async (file: File) => {
            const formData = new FormData()
            formData.append('file', file)
            return request<{ status: string; problem_id: string; slug: string }>('/problems/import', {
                method: 'POST',
                body: formData,
            })
        },
        exportProblemUrl: (slug: string) => BASE + `/problems/${slug}/export`,
        importCodeforces: async (contestId: string, problemIndex: string) =>
            request<{ status: string; problem_id: string; slug: string }>('/problems/import/codeforces', {
                method: 'POST',
                body: JSON.stringify({ contest_id: contestId, problem_index: problemIndex }),
            }),
        importCSES: async (problemId: string) =>
            request<{ status: string; problem_id: string; slug: string }>('/problems/import/cses', {
                method: 'POST',
                body: JSON.stringify({ problem_id: problemId }),
            }),
        importAtCoder: async (contestId: string, problemId: string) =>
            request<{ status: string; problem_id: string; slug: string }>('/problems/import/atcoder', {
                method: 'POST',
                body: JSON.stringify({ contest_id: contestId, problem_id: problemId }),
            }),
        importToph: async (problemId: string) =>
            request<{ status: string; problem_id: string; slug: string }>('/problems/import/toph', {
                method: 'POST',
                body: JSON.stringify({ problem_id: problemId }),
            }),
        importQOJ: async (problemId: string) =>
            request<{ status: string; problem_id: string; slug: string }>('/problems/import/qoj', {
                method: 'POST',
                body: JSON.stringify({ problem_id: problemId }),
            }),
    },
    submissions: {
        create: (d: unknown) => request<Submission>('/submissions', { method: 'POST', body: JSON.stringify(d) }),
        createUpsolving: (d: unknown) => request<Submission>('/submissions/upsolving', { method: 'POST', body: JSON.stringify(d) }),
        run: (d: { source_code: string; language: string; input: string; expected?: string; time_limit_ms?: number; memory_limit_kb?: number }) =>
            request<{
                status: string;
                stdout: string;
                stderr: string;
                time_used: number;
                memory_used: number;
                compile_output: string;
                passed: boolean | null;
                expected: string;
            }>('/submissions/run', { method: 'POST', body: JSON.stringify(d) }),
        /**
         * Run every sample case in one request. One quota token covers the
         * sweep, so a problem with more samples than /run's burst (3) no longer
         * fails partway through. Results come back in the order sent.
         */
        runBatch: (d: {
            source_code: string;
            language: string;
            cases: { input: string; expected?: string }[];
            time_limit_ms?: number;
            memory_limit_kb?: number;
        }) =>
            request<{
                results: {
                    status: string;
                    stdout?: string;
                    stderr?: string;
                    time_used: number;
                    memory_used: number;
                    compile_output?: string;
                    passed?: boolean | null;
                    expected?: string;
                }[];
            }>('/submissions/run-batch', { method: 'POST', body: JSON.stringify(d) }),
        get: (id: string) => request<Submission>(`/submissions/${id}`),
        retryRemote: (id: string) => request<{ status: string }>(`/submissions/${id}/retry-remote`, { method: 'POST' }),
        recheckRemote: (id: string) => request<{ status: string }>(`/submissions/${id}/recheck-remote`, { method: 'POST' }),
        /** Site-wide status page. Filters mirror the query parameters. */
        listAll: (params: {
            offset?: number;
            limit?: number;
            handle?: string;
            problem?: string;
            problemId?: string;
            contestId?: string;
            userId?: string;
            language?: string;
            status?: string;
        } = {}) => {
            const qs = new URLSearchParams();
            qs.set('offset', String(params.offset ?? 0));
            qs.set('limit', String(params.limit ?? 50));
            if (params.handle) qs.set('handle', params.handle);
            if (params.problem) qs.set('problem', params.problem);
            if (params.problemId) qs.set('problem_id', params.problemId);
            if (params.contestId) qs.set('contest_id', params.contestId);
            if (params.userId) qs.set('user_id', params.userId);
            if (params.language) qs.set('language', params.language);
            if (params.status) qs.set('status', params.status);
            return request<{ data: Submission[]; total: number }>(`/submissions/all?${qs.toString()}`);
        },
        list: (offset = 0, limit = 20, problemId?: string, contestId?: string) => {
            let url = `/submissions?offset=${offset}&limit=${limit}`;
            if (problemId) url += `&problem_id=${problemId}`;
            if (contestId) url += `&contest_id=${contestId}`;
            return request<{ data: Submission[]; total: number }>(url);
        },
    },
    features: {
        /**
         * Every gateable feature with the verdict for the current caller.
         * Public: the SPA needs this before it can render navigation, and the
         * verdict differs per role/rating, so it is resolved server-side.
         */
        list: () => request<{ data: ResolvedFeature[] }>('/features'),
    },
    languages: {
        /**
         * The judge's own language list, read from lang/*.yaml on the server.
         * Public and unauthenticated. This is the single source of truth for
         * every language dropdown — the SPA must not keep its own copy, or an
         * admin edit in the panel silently fails to reach the submit forms.
         */
        list: () => request<{ data: PublicLanguage[] }>('/languages'),
    },

    admin: {
        /**
         * Which sign-in methods this instance offers. `configured` is the
         * deployment's own capability (an OAuth provider with no client id
         * cannot work), so the panel can explain a switch that does nothing.
         */
        authMethods: {
            get: () => request<{ data: AdminAuthMethods }>('/admin/auth-methods'),
            set: (settings: Partial<AdminAuthMethods['settings']>) =>
                request<{ data: AdminAuthMethods }>('/admin/auth-methods', {
                    method: 'PUT',
                    body: JSON.stringify(settings),
                }),
        },
        features: {
            list: () => request<{ data: AdminFeature[] }>('/admin/features'),
            set: (key: string, policy: FeaturePolicy) =>
                request(`/admin/features/${encodeURIComponent(key)}`, { method: 'PUT', body: JSON.stringify(policy) }),
            reset: (key: string) =>
                request(`/admin/features/${encodeURIComponent(key)}`, { method: 'DELETE' }),
        },
        listUsers: (offset = 0, limit = 20) =>
            request<{ data: User[]; total: number }>(`/admin/users?offset=${offset}&limit=${limit}`),
        updateRole: (userId: string, role: string) =>
            request(`/admin/users/${userId}/role`, { method: 'PUT', body: JSON.stringify({ role }) }),
        // Advanced user management. `listUsers` above is kept for the bot-account
        // panel, which only needs a flat dropdown of accounts.
        users: {
            list: (q: AdminUserQuery = {}) => {
                // Only append keys that were actually set: `?status=` would be
                // rejected by the API's filter validation.
                const p = new URLSearchParams()
                if (q.search) p.set('search', q.search)
                if (q.role) p.set('role', q.role)
                if (q.status) p.set('status', q.status)
                if (q.sort) p.set('sort', q.sort)
                if (q.order) p.set('order', q.order)
                if (q.offset) p.set('offset', String(q.offset))
                if (q.limit) p.set('limit', String(q.limit))
                const qs = p.toString()
                return request<{ data: AdminUserRow[]; total: number; offset: number; limit: number }>(
                    `/admin/users${qs ? `?${qs}` : ''}`,
                )
            },
            get: (userId: string) => request<AdminUserDetail>(`/admin/users/${userId}`),
            setRole: (userId: string, role: string) =>
                request(`/admin/users/${userId}/role`, { method: 'PUT', body: JSON.stringify({ role }) }),
            resetPassword: (userId: string, password: string, force_change = true) =>
                request(`/admin/users/${userId}/password`, {
                    method: 'PUT',
                    body: JSON.stringify({ password, force_change }),
                }),
            ban: (userId: string, reason: string) =>
                request(`/admin/users/${userId}/ban`, { method: 'POST', body: JSON.stringify({ reason }) }),
            unban: (userId: string) =>
                request(`/admin/users/${userId}/ban`, { method: 'DELETE' }),
            verifyEmail: (userId: string) =>
                request(`/admin/users/${userId}/verify-email`, { method: 'POST' }),
            unverifyEmail: (userId: string) =>
                request(`/admin/users/${userId}/verify-email`, { method: 'DELETE' }),
            bulk: (d: { action: AdminBulkAction; ids: string[]; role?: string; reason?: string }) =>
                request<AdminBulkResponse>('/admin/users/bulk', { method: 'POST', body: JSON.stringify(d) }),
        },
        listApps: () => request<{ data: SetterApplication[] }>('/admin/setter-applications'),
        reviewApp: (userId: string, status: string) =>
            request(`/admin/setter-applications/${userId}/review`, { method: 'POST', body: JSON.stringify({ status }) }),
        // Authoring review: setters propose, admins publish. These are the only
        // routes that can put a problem or a contest into the public set.
        problemReview: {
            queue: (status = 'pending', offset = 0, limit = 20) =>
                request<{ data: ProblemListItem[]; total: number }>(
                    `/admin/problem-review?status=${encodeURIComponent(status)}&offset=${offset}&limit=${limit}`,
                ),
            review: (slug: string, status: string, note = '') =>
                request(`/admin/problem-review/${slug}`, {
                    method: 'POST',
                    body: JSON.stringify({ status, note }),
                }),
        },
        contestReview: {
            queue: (status = 'pending', offset = 0, limit = 20) =>
                request<{ data: Contest[]; total: number }>(
                    `/admin/contest-review?status=${encodeURIComponent(status)}&offset=${offset}&limit=${limit}`,
                ),
            review: (id: string, status: string, note = '') =>
                request(`/admin/contest-review/${id}`, {
                    method: 'POST',
                    body: JSON.stringify({ status, note }),
                }),
        },
        botAccounts: {
            list: (offset = 0, limit = 20) =>
                request<{ data: BotAccount[]; total: number }>(`/admin/bot-accounts?offset=${offset}&limit=${limit}`),
            create: (d: { user_id?: string; platform: string; platform_user: string; platform_pass: string; api_key?: string; api_secret?: string; rate_limit_rps?: number; session_data?: Record<string, string>; proxy_url?: string; proxy_enabled?: boolean }) =>
                request<BotAccount>('/admin/bot-accounts', { method: 'POST', body: JSON.stringify(d) }),
            update: (id: string, d: { platform_user?: string; platform_pass?: string; api_key?: string; api_secret?: string; status?: string; rate_limit_rps?: number; session_data?: Record<string, string>; proxy_url?: string; proxy_enabled?: boolean }) =>
                request<BotAccount>(`/admin/bot-accounts/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
            delete: (id: string) =>
                request(`/admin/bot-accounts/${id}`, { method: 'DELETE' }),
            testLogin: (d: { platform: string; platform_user?: string; platform_pass?: string; session_data?: Record<string, string> }) =>
                request<{ status: string; message: string; cookies?: number }>('/admin/bot-accounts/test-login', { method: 'POST', body: JSON.stringify(d) }),
        },
        remoteLanguages: {
            list: (platform: string) =>
                request<{ data: RemoteLanguage[] }>(`/admin/remote-languages/${platform}`),
            create: (d: { platform: string; local_id: string; remote_id: string; display_name: string; enabled?: boolean; sort_order?: number }) =>
                request<RemoteLanguage>('/admin/remote-languages', { method: 'POST', body: JSON.stringify(d) }),
            update: (id: string, d: { local_id?: string; remote_id?: string; display_name?: string; enabled?: boolean; sort_order?: number }) =>
                request<RemoteLanguage>(`/admin/remote-languages/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
            delete: (id: string) =>
                request(`/admin/remote-languages/${id}`, { method: 'DELETE' }),
            detect: (platform: string) =>
                request<{ matched: { remote_id: string; display_name: string; local_id: string }[]; unmatched: { remote_id: string; display_name: string }[]; existing: RemoteLanguage[] }>(`/admin/remote-languages/detect/${platform}`, { method: 'POST' }),
            bulkUpsert: (items: { platform: string; local_id: string; remote_id: string; display_name: string; enabled: boolean; sort_order: number }[]) =>
                request<unknown>('/admin/remote-languages/bulk', { method: 'POST', body: JSON.stringify(items) }),
        },
        submissions: {
            pendingRemote: () =>
                request<{ data: PendingRemoteSubmission[]; total: number }>('/admin/submissions/pending-remote'),
            /** Recent System Error submissions with the judge's failure text. */
            failed: (limit = 50) =>
                request<{ data: Submission[]; total: number }>(`/admin/submissions/failed?limit=${limit}`),
            rejudge: (id: string) =>
                request<{ status: string }>(`/admin/submissions/${id}/rejudge`, { method: 'POST' }),
            refresh: (id: string) =>
                request<{ status: string }>(`/admin/submissions/${id}/refresh`, { method: 'POST' }),
        },
        settings: {
            list: () =>
                request<{ data: { key: string; value: string | number | boolean; description: string; updated_at: string; updated_by?: string | null }[] }>('/admin/settings'),
            update: (key: string, value: unknown) =>
                request(`/admin/settings/${encodeURIComponent(key)}`, { method: 'PUT', body: JSON.stringify({ value }) }),
        },
        languages: {
            list: () =>
                request<{ data: LanguageConfig[] }>('/admin/languages'),
            get: (key: string) =>
                request<LanguageConfig>(`/admin/languages/${key}`),
            getRaw: (key: string) =>
                request<string>(`/admin/languages/${key}/raw`),
            create: (d: unknown) =>
                request<LanguageConfig>('/admin/languages', { method: 'POST', body: JSON.stringify(d) }),
            update: (key: string, d: unknown) =>
                request<LanguageConfig>(`/admin/languages/${key}`, { method: 'PUT', body: JSON.stringify(d) }),
            updateRaw: (key: string, content: string) =>
                request<unknown>(`/admin/languages/${key}/raw`, { method: 'PUT', body: JSON.stringify({ content }) }),
            delete: (key: string) =>
                request(`/admin/languages/${key}`, { method: 'DELETE' }),
            /** Restore the config shipped in lang/<key>.yaml. */
            reset: (key: string) =>
                request<LanguageConfig>(`/admin/languages/${key}/reset`, { method: 'POST' }),
            test: (key: string) =>
                request<TestResult>(`/admin/languages/${key}/test`, { method: 'POST' }),
            detect: () =>
                request<{ compilers: DetectedTool[]; interpreters: DetectedTool[] }>('/admin/languages/detect'),
            templates: () =>
                request<{ data: LanguageConfig[] }>('/admin/languages/templates'),
        },
        backups: {
            list: () =>
                request<{ data: BackupFile[] }>('/admin/backups'),
            create: (type: string) =>
                request<unknown>('/admin/backups', { method: 'POST', body: JSON.stringify({ type }) }),
            delete: (filename: string) =>
                request<unknown>(`/admin/backups/${encodeURIComponent(filename)}`, { method: 'DELETE' }),
            restore: (filename: string, password: string, type: string) =>
                request<unknown>(`/admin/backups/${encodeURIComponent(filename)}/restore`, { method: 'POST', body: JSON.stringify({ password, type }) }),
            upload: (file: File) => {
                const formData = new FormData()
                formData.append('file', file)
                return request<unknown>('/admin/backups/upload', {
                    method: 'POST',
                    body: formData,
                })
            },
        },
        aiModels: {
            list: () =>
                request<{ data: AIModel[] }>('/admin/ai-models'),
            getById: (id: string) =>
                request<AIModel>(`/admin/ai-models/${id}`),
            create: (d: { name: string; endpoint: string; api_key?: string; model_name: string; enabled: boolean; priority: number; description: string; allow_private_network?: boolean; structured_output_mode?: string; request_timeout_seconds?: number; max_output_tokens?: number }) =>
                request<AIModel>('/admin/ai-models', { method: 'POST', body: JSON.stringify(d) }),
            update: (id: string, d: { name: string; endpoint: string; api_key?: string; model_name: string; enabled: boolean; priority: number; description: string; allow_private_network?: boolean; structured_output_mode?: string; request_timeout_seconds?: number; max_output_tokens?: number }) =>
                request<AIModel>(`/admin/ai-models/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
            toggle: (id: string) =>
                request<{ id: string; enabled: boolean }>(`/admin/ai-models/${id}/toggle`, { method: 'PUT' }),
            delete: (id: string) =>
                request(`/admin/ai-models/${id}`, { method: 'DELETE' }),
            testConnection: (d: { id?: string; endpoint: string; api_key: string; model_name: string; allow_private_network?: boolean; structured_output_mode?: string }) =>
                request<{ ok: boolean; error?: string; mode?: string; latency_ms?: number }>('/admin/ai-models/test', { method: 'POST', body: JSON.stringify(d) }),
        },
    },
    setter: {
        apply: (reason: string) => request('/auth/setter-apply', { method: 'POST', body: JSON.stringify({ reason }) }),
        status: () => request<SetterApplication>('/auth/setter-status'),
    },
    contests: {
        list: (offset = 0, limit = 20, division?: number) => {
            let url = `/contests?offset=${offset}&limit=${limit}`;
            if (division !== undefined) url += `&division=${division}`;
            return request<{ data: Contest[]; total: number }>(url);
        },
        listMy: (offset = 0, limit = 20) => request<{ data: Contest[]; total: number }>(`/contests/my?offset=${offset}&limit=${limit}`),
        // A setter proposes a round; only an admin can publish it.
        submitForReview: (id: string) =>
            request(`/contests/${id}/submit`, { method: 'POST' }),

        get: (id: string) => request<ContestDetailResponse>(`/contests/${id}`),
        /**
         * Redeem a private round's password for access. 204 on success; the grant is
         * what the server checks on every later read and on submit, so this only has
         * to be called once.
         */
        unlock: (id: string, password: string) =>
            request(`/contests/${id}/unlock`, { method: 'POST', body: JSON.stringify({ password }) }),
        create: (d: unknown) => request<Contest>('/contests', { method: 'POST', body: JSON.stringify(d) }),
        getFormats: () => request<{ formats: string[] }>('/contests/formats'),
        scoreboard: (id: string, view?: string, page?: number) => {
            const params = new URLSearchParams();
            if (view) params.set('view', view);
            if (page && page > 1) params.set('page', String(page));
            const qs = params.toString();
            return request<ScoreboardData>(`/contests/${id}/scoreboard${qs ? '?' + qs : ''}`);
        },
        register: (id: string) => request(`/contests/${id}/register`, { method: 'POST' }),
        unregister: (id: string) => request(`/contests/${id}/register`, { method: 'DELETE' }),
        checkRegistration: (id: string) => request<{ registered: boolean }>(`/contests/${id}/register`),
        listRegistrations: (id: string) => request<{ data: ContestRegistration[]; count: number }>(`/contests/${id}/registrations`),
        getProblemByIndex: (contestId: string, index: string) =>
            request<{ problem: Problem; contest: Contest; can_submit?: boolean; upsolving_disabled?: boolean; statement_hidden?: boolean; is_judge?: boolean; spoilers_hidden?: boolean }>(`/contests/${contestId}/problems/${index}`),
        addProblem: (contestId: string, d: { problem_id: string; index: string; score?: number }) =>
            request<{ status: string }>(`/contests/${contestId}/problems`, { method: 'POST', body: JSON.stringify(d) }),
        updateProblem: (contestId: string, problemId: string, d: { index: string; score: number; sort_order: number }) =>
            request<{ status: string }>(`/contests/${contestId}/problems/${problemId}`, { method: 'PUT', body: JSON.stringify(d) }),
        removeProblem: (contestId: string, problemId: string) =>
            request(`/contests/${contestId}/problems/${problemId}`, { method: 'DELETE' }),
        update: (id: string, d: unknown) =>
            request<Contest>(`/contests/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        stats: (id: string) => request<unknown>(`/contests/${id}/stats`),
        standings: (id: string, page?: number) => {
            const params = new URLSearchParams();
            if (page && page > 1) params.set('page', String(page));
            const qs = params.toString();
            return request<ScoreboardData>(`/contests/${id}/scoreboard${qs ? '?' + qs : ''}`);
        },
        problems: (id: string) => request<ContestProblem[]>(`/contests/${id}/problems`),
        /**
         * Every notice for a contest, wrapped as { data } by the server.
         * Typed correctly here because the poll that drives the popup needs the
         * real shape: treating this as a bare array made the .data branch
         * unreachable and "new" detection fall back to comparing lengths.
         */
        announcements: (id: string) => request<{ data: ContestNotice[] }>(`/contests/${id}/notices`),
        postAnnouncement: (id: string, message: string) =>
            request<ContestNotice>(`/contests/${id}/notices`, { method: 'POST', body: JSON.stringify({ content: message }) }),
        deleteAnnouncement: (id: string, announcementId: string) =>
            request(`/contests/${id}/notices/${announcementId}`, { method: 'DELETE' }),
        submissions: (id: string, offset?: number, limit?: number, filters?: { mine?: boolean; problem_id?: string; language?: string; status?: string }) => {
            const params = new URLSearchParams();
            if (offset) params.set('offset', String(offset));
            if (limit) params.set('limit', String(limit));
            if (filters?.mine !== undefined) params.set('mine', String(filters.mine));
            if (filters?.problem_id) params.set('problem_id', filters.problem_id);
            if (filters?.language) params.set('language', filters.language);
            if (filters?.status) params.set('status', filters.status);
            return request<ContestSubmissionsResponse>(`/contests/${id}/submissions${params.toString() ? '?' + params : ''}`);
        },
        /**
         * Void a submission from the round's own submissions tab. Same action
         * and same admin-configured role policy as the problem-scoped route;
         * the contest in the path is what the accountability check runs against.
         */
        skipSubmission: (id: string, submissionId: string, reason: string) =>
            request<{ id: string; status: string; was: string }>(`/contests/${id}/submissions/${submissionId}/skip`, {
                method: 'POST',
                body: JSON.stringify({ reason }),
            }),
    },
    /**
     * Manual standings corrections. A delta layered over the computed result, so
     * the submissions keep the verdicts they earned.
     */
    adjustments: {
        list: (contestId: string) =>
            request<{ data: ContestAdjustment[] }>(`/contests/${contestId}/adjustments`),
        create: (contestId: string, d: {
            user_id: string
            scope: 'contest' | 'problem' | 'submission'
            problem_id?: string
            submission_id?: string
            point_delta?: number
            solved_delta?: number
            penalty_delta?: number
            reason: string
        }) => request<ContestAdjustment>(`/contests/${contestId}/adjustments`, {
            method: 'POST', body: JSON.stringify(d),
        }),
        remove: (contestId: string, id: string) =>
            request(`/contests/${contestId}/adjustments/${id}`, { method: 'DELETE' }),
    },
    clarifications: {
        /**
         * A round's clarification queue.
         *
         * `problem_id` and `answered` narrow it (the jury's "unanswered, about
         * problem C"); omit both for the full list. `is_judge` tells the UI
         * whether the caller is staff and so may answer.
         */
        list: (contestId: string, filters?: { problem_id?: string; answered?: boolean }) => {
            const params = new URLSearchParams();
            if (filters?.problem_id) params.set('problem_id', filters.problem_id);
            if (filters?.answered !== undefined) params.set('answered', String(filters.answered));
            const qs = params.toString();
            return request<{ data: Clarification[]; is_judge: boolean }>(
                `/contests/${contestId}/clarifications${qs ? '?' + qs : ''}`);
        },
        create: (contestId: string, d: { problem_id?: string; question: string }) =>
            request<Clarification>(`/contests/${contestId}/clarifications`, { method: 'POST', body: JSON.stringify(d) }),
        /** Returns the updated row, so the queue can replace it in place. */
        answer: (contestId: string, id: string, d: { answer: string; is_public: boolean }) =>
            request<Clarification>(`/contests/${contestId}/clarifications/${id}/answer`, { method: 'POST', body: JSON.stringify(d) }),
        /** One answer applied to many questions; returns how many changed. */
        answerMany: (contestId: string, d: { ids: string[]; answer: string; is_public: boolean }) =>
            request<{ answered: number }>(`/contests/${contestId}/clarifications/bulk-answer`, { method: 'POST', body: JSON.stringify(d) }),
    },
    notices: {
        list: (contestId: string) =>
            request<{ data: ContestNotice[] }>(`/contests/${contestId}/notices`),
        create: (contestId: string, d: { content: string }) =>
            request<ContestNotice>(`/contests/${contestId}/notices`, { method: 'POST', body: JSON.stringify(d) }),
        delete: (contestId: string, id: string) =>
            request(`/contests/${contestId}/notices/${id}`, { method: 'DELETE' }),
    },
    virtual: {
        start: (contestId: string, durationMinutes?: number) =>
            request<VirtualContest>('/virtual/start', { method: 'POST', body: JSON.stringify({ contest_id: contestId, duration_minutes: durationMinutes }) }),
        status: () => request<VirtualStatusResponse>('/virtual/status'),
        complete: (id: string) => request(`/virtual/${id}/complete`, { method: 'POST' }),
    },
    onsite: {
        listBalloons: (contestId: string) =>
            request<{ data: unknown[] }>(`/contests/${contestId}/onsite/balloons`),
        dispatchBalloon: (contestId: string, balloonId: string) =>
            request(`/contests/${contestId}/onsite/balloons/${balloonId}/dispatch`, { method: 'POST' }),
        requestPrint: (contestId: string, d: { filename: string; content: string }) =>
            request(`/contests/${contestId}/onsite/print`, { method: 'POST', body: JSON.stringify(d) }),
        listPrints: (contestId: string) =>
            request<{ data: unknown[] }>(`/contests/${contestId}/onsite/prints`),
        updatePrintStatus: (contestId: string, printId: string, status: string) =>
            request(`/contests/${contestId}/onsite/prints/${printId}/status`, { method: 'POST', body: JSON.stringify({ status }) }),
        // Returns AuthResponse: a linked account with TOTP enabled gets
        // { requires_2fa, challenge_id } instead of tokens.
        loginAsTeam: (contestId: string, d: unknown) =>
            request<AuthResponse>(`/contests/${contestId}/onsite/login`, { method: 'POST', body: JSON.stringify(d) }),
        generateBatch: (contestId: string, d: unknown) =>
            request<{ users: OnsiteBatchUser[] }>(`/contests/${contestId}/onsite/generate`, { method: 'POST', body: JSON.stringify(d) }),
        listBatch: (contestId: string) =>
            request<{ data: OnsiteBatchUser[] }>(`/contests/${contestId}/onsite/users`),
        deleteUser: (contestId: string, userId: string) =>
            request(`/contests/${contestId}/onsite/users/${userId}`, { method: 'DELETE' }),
    },
    permissions: {
        list: (contestId: string) =>
            request<{ data: ContestPermission[] }>(`/contests/${contestId}/permissions`),
        add: (contestId: string, d: { user_id: string; access_level: string }) =>
            request<{ status: string }>(`/contests/${contestId}/permissions`, { method: 'POST', body: JSON.stringify(d) }),
        remove: (contestId: string, userId: string) =>
            request(`/contests/${contestId}/permissions/${userId}`, { method: 'DELETE' }),
    },
    gym: {
        list: (offset = 0, limit = 20, filters?: { category?: string; search?: string }) => {
            let url = `/gym?offset=${offset}&limit=${limit}`;
            if (filters?.category) url += `&category=${filters.category}`;
            if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
            return request<{ data: GymContest[]; total: number }>(url);
        },
        get: (id: string) => request<GymData>(`/gym/${id}`),
        create: (d: unknown) => request<unknown>('/gym', { method: 'POST', body: JSON.stringify(d) }),
        markSolved: (id: string) => request(`/gym/${id}/solve`, { method: 'POST' }),
        isSolved: (id: string) => request<{ solved: boolean }>(`/gym/${id}/solved`),
    },
    hacks: {
        submit: (d: { contest_id: string; problem_id: string; submission_id: string; test_input: string }) =>
            request<HackResult>('/hacks', { method: 'POST', body: JSON.stringify(d) }),
        get: (id: string) => request<Hack>(`/hacks/${id}`),
        listByContest: (contestId: string) => request<unknown>(`/hacks/contest/${contestId}`),
        listHackable: (contestId: string, problemId: string) => request<{ data: HackSubmission[] }>(`/hacks/hackable/${contestId}/${problemId}`),
    },
    stats: {
        getPlatform: () => request<{ problems: number; users: number; submissions: number }>('/stats/platform'),
        getProblemStats: (problemId: string) => request<ProblemStats>(`/stats/problems/${problemId}`),
        getMyStats: () => request<UserProblemStats>('/stats/me'),
        getUserStats: (userId: string) => request<UserProblemStats>(`/stats/user/${userId}`),
    },
    notifications: {
        list: (unreadOnly = false, limit = 50) =>
            request<{ data: Notification[] }>(`/notifications?unread=${unreadOnly}&limit=${limit}`),
        unreadCount: () => request<{ count: number }>('/notifications/unread-count'),
        markAsRead: (id: string) => request(`/notifications/${id}/read`, { method: 'POST' }),
        markAllAsRead: () => request('/notifications/read-all', { method: 'POST' }),
        getPreferences: () => request<NotificationPreferences>('/notifications/preferences'),
        updatePreferences: (prefs: unknown) => request('/notifications/preferences', { method: 'PUT', body: JSON.stringify(prefs) }),
    },
    groups: {
        list: (offset = 0, limit = 20) =>
            request<{ data: GroupListItem[]; total: number }>(`/groups?offset=${offset}&limit=${limit}`),
        my: () => request<{ data: Group[] }>('/groups/my'),
        get: (id: string) => request<Group>(`/groups/${id}`),
        create: (d: unknown) => request<Group>('/groups', { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: unknown) => request<Group>(`/groups/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request<unknown>(`/groups/${id}`, { method: 'DELETE' }),
        join: (id: string) => request(`/groups/${id}/join`, { method: 'POST' }),
        leave: (id: string) => request(`/groups/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<{ data: GroupMember[] }>(`/groups/${id}/members`),
        getContests: (id: string) => request<{ data: Contest[] }>(`/groups/${id}/contests`),
        addContest: (id: string, contestId: string) =>
            request(`/groups/${id}/contests`, { method: 'POST', body: JSON.stringify({ contest_id: contestId }) }),
        removeContest: (id: string, contestId: string) =>
            request(`/groups/${id}/contests/${contestId}`, { method: 'DELETE' }),
        invite: (id: string, d: { username: string }) =>
            request<unknown>(`/groups/${id}/invite`, { method: 'POST', body: JSON.stringify(d) }),
        respond: (id: string, d: { user_id: string; action: string }) =>
            request<unknown>(`/groups/${id}/respond`, { method: 'POST', body: JSON.stringify(d) }),
        pending: (id: string) =>
            request<{ data: GroupPendingInvite[] }>(`/groups/${id}/pending`),
        joinByCode: (code: string) =>
            request<{ group_name?: string }>('/groups/join-code', { method: 'POST', body: JSON.stringify({ invite_code: code }) }),
    },
    teams: {
        list: (offset = 0, limit = 20) =>
            request<{ data: TeamListItem[]; total: number }>(`/teams?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<Team>(`/teams/${id}`),
        create: (d: unknown) => request<Team>('/teams', { method: 'POST', body: JSON.stringify(d) }),
        join: (id: string) => request(`/teams/${id}/join`, { method: 'POST' }),
        leave: (id: string) => request(`/teams/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<{ data: TeamMember[] }>(`/teams/${id}/members`),
        update: (id: string, d: unknown) => request<Team>(`/teams/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request<unknown>(`/teams/${id}`, { method: 'DELETE' }),
        invite: (id: string, d: { username: string }) =>
            request<unknown>(`/teams/${id}/invite`, { method: 'POST', body: JSON.stringify(d) }),
        requestJoin: (id: string) =>
            request<unknown>(`/teams/${id}/request`, { method: 'POST' }),
        respond: (id: string, d: { user_id: string; action: string }) =>
            request<unknown>(`/teams/${id}/respond`, { method: 'POST', body: JSON.stringify(d) }),
        pending: (id: string) =>
            request<{ data: TeamMember[] }>(`/teams/${id}/pending`),
    },
    blog: {
        list: (offset = 0, limit = 20, tag?: string) => {
            let url = `/blog?offset=${offset}&limit=${limit}`;
            if (tag) url += `&tag=${encodeURIComponent(tag)}`;
            return request<{ data: BlogListItem[]; total: number }>(url);
        },
        get: (id: string) => request<BlogPost>(`/blog/${id}`),
        create: (d: unknown) => request<BlogPost>('/blog', { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: unknown) => request<BlogPost>(`/blog/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request<unknown>(`/blog/${id}`, { method: 'DELETE' }),
        getComments: (type: string, id: string) => request<{ data: Comment[] }>(`/blog/${type}/${id}/comments`),
        createComment: (d: unknown) => request<Comment>('/blog/comments', { method: 'POST', body: JSON.stringify(d) }),
        vote: (d: { target_type: string; target_id: string; value: number }) =>
            request('/blog/vote', { method: 'POST', body: JSON.stringify(d) }),
        getVote: (id: string, type = 'blog') => request<{ value: number }>(`/blog/${id}/vote?type=${type}`),
    },
    editorials: {
        list: (offset = 0, limit = 20) =>
            request<{ data: Editorial[]; total: number }>(`/editorials?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<Editorial>(`/editorials/${id}`),
        getByProblem: (problemId: string) => request<{ data: Editorial[] }>(`/editorials/problem/${problemId}`),
        create: (d: unknown) => request<Editorial>('/editorials', { method: 'POST', body: JSON.stringify(d) }),
    },
    apiKeys: {
        list: () => request<{ data: APIKey[] }>('/keys'),
        create: (d: { name: string; description?: string }) =>
            request<{ api_key: string; secret: string }>('/keys', { method: 'POST', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/keys/${id}`, { method: 'DELETE' }),
    },
    webhooks: {
        list: () => request<{ data: Webhook[] }>('/webhooks'),
        create: (d: { url: string; secret?: string; events: string[] }) =>
            request<Webhook>('/webhooks', { method: 'POST', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/webhooks/${id}`, { method: 'DELETE' }),
    },
    recommendations: {
        get: (rating?: number) =>
            request<{
                progression: ProblemListItem[];
                weak_tags: WeakTagsRecommendations;
                hybrid: ProblemListItem[];
            }>(rating !== undefined ? `/recommendations?rating=${rating}` : '/recommendations'),
    },
    organizations: {
        list: (offset = 0, limit = 20) =>
            request<{ data: OrganizationListItem[]; total: number }>(`/organizations?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<Organization>(`/organizations/${id}`),
        create: (d: { name: string; description: string }) =>
            request<Organization>('/organizations', { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: { name: string; description: string }) =>
            request<Organization>(`/organizations/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/organizations/${id}`, { method: 'DELETE' }),
        join: (id: string) => request(`/organizations/${id}/join`, { method: 'POST' }),
        leave: (id: string) => request(`/organizations/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<{ data: OrganizationMember[] }>(`/organizations/${id}/members`),
        addMember: (id: string, userId: string, role: string) =>
            request(`/organizations/${id}/members`, { method: 'POST', body: JSON.stringify({ user_id: userId, role }) }),
        removeMember: (id: string, userId: string) =>
            request(`/organizations/${id}/members/${userId}`, { method: 'DELETE' }),
        my: () => request<{ data: OrganizationListItem[] }>('/organizations/my'),
    },
    classes: {
        list: (orgId: string, offset = 0, limit = 20) =>
            request<{ data: ClassListItem[]; total: number }>(`/organizations/${orgId}/classes?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<Class>(`/classes/${id}`),
        create: (orgId: string, d: { name: string; description: string }) =>
            request<Class>(`/organizations/${orgId}/classes`, { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: { name: string; description: string }) =>
            request<Class>(`/classes/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/classes/${id}`, { method: 'DELETE' }),
        joinByCode: (inviteCode: string) =>
            request<{ status: string; class_id: string }>('/classes/join', { method: 'POST', body: JSON.stringify({ invite_code: inviteCode }) }),
        leave: (id: string) => request(`/classes/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<{ data: ClassMember[] }>(`/classes/${id}/members`),
    },
    training: {
        list: (offset = 0, limit = 20, opts?: { orgId?: string; public?: boolean }) => {
            let url = `/training?offset=${offset}&limit=${limit}`;
            if (opts?.orgId) url += `&org_id=${opts.orgId}`;
            if (opts?.public) url += `&public=true`;
            return request<{ data: TrainingPlan[]; total: number }>(url);
        },
        get: (id: string) => request<TrainingPlanDetail>(`/training/${id}`),
        create: (d: unknown) => request<TrainingPlan>('/training', { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: unknown) => request<TrainingPlan>(`/training/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/training/${id}`, { method: 'DELETE' }),
        enroll: (id: string) => request(`/training/${id}/enroll`, { method: 'POST' }),
        unenroll: (id: string) => request(`/training/${id}/enroll`, { method: 'DELETE' }),
        enrollments: (id: string) => request<{ data: TrainingPlanEnrollment[] }>(`/training/${id}/enrollments`),
        progress: (id: string) => request<TrainingPlanProgress>(`/training/${id}/progress`),
        addSection: (planId: string, d: { title: string; description: string }) =>
            request<TrainingPlanSection>(`/training/${planId}/sections`, { method: 'POST', body: JSON.stringify(d) }),
        deleteSection: (sectionId: string) =>
            request(`/training/sections/${sectionId}`, { method: 'DELETE' }),
        addProblem: (sectionId: string, d: { problem_id: string; points: number }) =>
            request<TrainingPlanProblem>(`/training/sections/${sectionId}/problems`, { method: 'POST', body: JSON.stringify(d) }),
        removeProblem: (problemId: string) =>
            request(`/training/problems/${problemId}`, { method: 'DELETE' }),
    },
    ratings: {
        getByUser: (userId: string, limit = 50) =>
            request<{ data: RatingHistoryEntry[] }>(`/rating/user/${userId}?limit=${limit}`),
        getByContest: (contestId: string) =>
            request<{ data: RatingHistoryEntry[] }>(`/rating/contest/${contestId}`),
    },
    plagiarism: {
        runCheck: (contestId: string, threshold?: number) =>
            request<unknown>(`/contests/${contestId}/plagiarism/check`, {
                method: 'POST',
                body: JSON.stringify({ contest_id: contestId, threshold }),
            }),
        getReport: (contestId: string) =>
            request<PlagiarismReport>(`/contests/${contestId}/plagiarism/report`),
        // The endpoint returns PlagiarismPairDetail rows (user names, problem
        // title, languages), not the bare PlagiarismPair model. Typing it as the
        // latter made every render of those fields a type error waiting to be
        // papered over with an `as never`.
        listPairs: (contestId: string, reportId: string, offset = 0, limit = 50) =>
            request<{ data: PlagiarismPairDetail[]; total: number }>(
                `/contests/${contestId}/plagiarism/report/${reportId}/pairs?offset=${offset}&limit=${limit}`
            ),
        updatePair: (contestId: string, pairId: string, status: string) =>
            request(`/contests/${contestId}/plagiarism/pairs/${pairId}`, {
                method: 'PUT',
                body: JSON.stringify({ status }),
            }),
    },
    rankings: {
        list: (offset = 0, limit = 50, country?: string, organization?: string) => {
            let url = `/rankings?offset=${offset}&limit=${limit}`;
            if (country) url += `&country=${encodeURIComponent(country)}`;
            if (organization) url += `&organization=${encodeURIComponent(organization)}`;
            return request<{ data: RankingEntry[]; total: number }>(url);
        },
    },
    feed: {
        activity: (limit = 30) => request<{ data: ActivityItem[] }>(`/activity?limit=${limit}`),
    },
    users: {
        exportMyData: () => request<Record<string, unknown>>('/users/me/export'),
        deleteAccount: (d: { password: string; confirm: boolean }) =>
            request<{ status: string }>('/users/me', { method: 'DELETE', body: JSON.stringify(d) }),
        getByUsername: (username: string) =>
            request<PublicProfile>(`/users/${encodeURIComponent(username)}`),
        getSubmissions: (username: string, offset = 0, limit = 20) =>
            request<{ data: Submission[]; total: number }>(`/users/${encodeURIComponent(username)}/submissions?offset=${offset}&limit=${limit}`),
        getBlogs: (username: string, offset = 0, limit = 20) =>
            request<{ data: BlogListItem[]; total: number }>(`/users/${encodeURIComponent(username)}/blogs?offset=${offset}&limit=${limit}`),
        getComments: (username: string, offset = 0, limit = 20) =>
            request<{ data: Comment[]; total: number }>(`/users/${encodeURIComponent(username)}/comments?offset=${offset}&limit=${limit}`),
        getProfile: () =>
            request<UserProfile>('/users/profile/edit'),
        updateProfile: (d: { first_name?: string; last_name?: string; country?: string; city?: string; organization?: string; github_url?: string; bio?: string; avatar_url?: string; show_email?: boolean; show_tags?: boolean }) =>
            request<UserProfile>('/users/profile/edit', { method: 'PUT', body: JSON.stringify(d) }),
        updatePassword: (d: { current_password: string; new_password: string }) =>
            request<{ message: string }>('/users/profile/password', { method: 'PUT', body: JSON.stringify(d) }),
        follow: (username: string) =>
            request<{ following: boolean }>(`/users/${encodeURIComponent(username)}/follow`, { method: 'POST' }),
        unfollow: (username: string) =>
            request<{ following: boolean }>(`/users/${encodeURIComponent(username)}/follow`, { method: 'DELETE' }),
        followStatus: (username: string) =>
            request<{ following: boolean; following_count: number; follower_count: number }>(
                `/users/${encodeURIComponent(username)}/follow/status`,
            ),
        followers: (username: string) =>
            request<{ data: unknown[] }>(`/users/${encodeURIComponent(username)}/followers`),
        following: (username: string) =>
            request<{ data: unknown[] }>(`/users/${encodeURIComponent(username)}/following`),
        achievements: (userId: string) =>
            request<{ data: UserAchievement[] }>(`/users/${encodeURIComponent(userId)}/achievements`),
        getPendingInvites: () =>
            request<{ teams: PendingInvite[]; groups: GroupPendingInvite[] }>('/users/profile/invites'),
    },
    search: {
        global: (q: string, limit = 10) =>
            request<{ problems: ProblemListItem[]; users: User[]; contests: Contest[] }>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),
    },
    remoteLanguages: {
        list: (platform: string) =>
            request<{ data: RemoteLanguage[] }>(`/remote-languages/${platform}`),
    },
    ai: {
        connections: () => request<{ data: AIModel[] }>('/ai/connections'),
        presets: () => request<{ version: string; data: AIBriefPreset[] }>('/ai/brief-presets'),
        listJobs: (offset = 0, limit = 50) =>
            request<{ data: AIGenerationJob[]; total: number }>(`/ai/jobs?offset=${offset}&limit=${limit}`),
        createJob: (d: { problem_id?: string; ai_model_id?: string; artifact_type: string; brief: AIBrief }) =>
            request<AIGenerationJob>('/ai/jobs', { method: 'POST', body: JSON.stringify(d) }),
        getJob: (id: string) => request<AIGenerationJob>(`/ai/jobs/${id}`),
        cancelJob: (id: string) => request<{ cancelled: boolean }>(`/ai/jobs/${id}/cancel`, { method: 'POST' }),
        discardJob: (id: string) => request<{ discarded: boolean }>(`/ai/jobs/${id}/discard`, { method: 'POST' }),
        retryJob: (id: string) => request<AIGenerationJob>(`/ai/jobs/${id}/retry`, { method: 'POST' }),
        applyJob: (id: string, sections: string[], expected_updated_at?: string) =>
            request<{ problem_id: string; problem_slug: string }>(`/ai/jobs/${id}/apply`, { method: 'POST', body: JSON.stringify({ sections, expected_updated_at }) }),
        problemJobs: (problemId: string) => request<{ data: AIGenerationJob[] }>(`/ai/problems/${problemId}/jobs`),
        adminListJobs: (params: { offset?: number; limit?: number; actor_id?: string; status?: string } = {}) => {
            const q = new URLSearchParams()
            if (params.offset !== undefined) q.set('offset', String(params.offset))
            if (params.limit !== undefined) q.set('limit', String(params.limit))
            if (params.actor_id) q.set('actor_id', params.actor_id)
            if (params.status) q.set('status', params.status)
            return request<{ data: AIGenerationJob[]; total: number }>(`/admin/ai/jobs?${q}`)
        },
        adminCancelJob: (id: string) => request<{ cancelled: boolean }>(`/admin/ai/jobs/${id}/cancel`, { method: 'POST' }),
    },
}
