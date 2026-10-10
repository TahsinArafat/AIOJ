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

export function setTokens(a: string, r: string) {
    accessToken = a
    refreshToken = r
    localStorage.setItem('access_token', a)
    localStorage.setItem('refresh_token', r)
}

export function clearTokens() {
    accessToken = null
    refreshToken = null
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
}

export function getAccessToken(): string | null { return accessToken }

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
let refreshInFlight: Promise<boolean> | null = null

async function refreshAccessToken(): Promise<boolean> {
    if (!refreshToken) return false
    if (refreshInFlight) return refreshInFlight

    refreshInFlight = (async () => {
        try {
            const res = await fetch(BASE + '/auth/refresh', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': await ensureCsrfToken() },
                body: JSON.stringify({ refresh_token: refreshToken }),
            })
            if (!res.ok) return false
            const data = await res.json()
            setTokens(data.access_token, data.refresh_token)
            return true
        } catch {
            return false
        } finally {
            // Cleared in a microtask so callers awaiting this promise all see
            // the same result before a new refresh can start.
            queueMicrotask(() => { refreshInFlight = null })
        }
    })()

    return refreshInFlight
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
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

    if (res.status === 401 && refreshToken) {
        if (await refreshAccessToken()) {
            headers['Authorization'] = `Bearer ${accessToken}`
            res = await fetch(BASE + path, { ...opts, headers })
        } else {
            clearTokens()
            throw new Error('session expired')
        }
    }

    if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `HTTP ${res.status}`)
    }
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

import type { AIModel, APIKey, ActivityItem, AuthResponse, AuthSessionResponse, BackupFile, BlogListItem, BlogPost, BotAccount, Clarification, Class, ClassListItem, ClassMember, Comment, Contest, ContestDetailResponse, ContestNotice, ContestPermission, ContestProblem, ContestRegistration, ContestSubmissionsResponse, DetectedTool, Editorial, Group, GroupListItem, GroupMember, GroupPendingInvite, GymContest, GymData, Hack, HackResult, HackSubmission, LanguageConfig, Notification, NotificationPreferences, OnsiteBatchUser, Organization, OrganizationListItem, OrganizationMember, PendingInvite, PendingRemoteSubmission, PlagiarismPair, PlagiarismReport, Problem, ProblemI18n, ProblemListItem, ProblemPermission, ProblemStats, PublicProfile, RankingEntry, RatingHistoryEntry, RemoteLanguage, ScoreboardData, SetterApplication, Submission, Team, TeamListItem, TeamMember, TestResult, TrainingPlan, TrainingPlanDetail, TrainingPlanEnrollment, TrainingPlanProblem, TrainingPlanProgress, TrainingPlanSection, User, UserAchievement, UserProblemStats, UserProfile, VirtualContest, VirtualStatusResponse, WeakTagsRecommendations, Webhook } from '../types/api'

export const api = {
    auth: {
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
        list: (offset = 0, limit = 20, filters?: { difficulty?: string; tags?: string[]; search?: string; source?: string }) => {
            let url = `/problems?offset=${offset}&limit=${limit}`;
            if (filters?.difficulty) url += `&difficulty=${filters.difficulty}`;
            if (filters?.tags?.length) url += `&tags=${filters.tags.join(',')}`;
            if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
            if (filters?.source) url += `&source=${encodeURIComponent(filters.source)}`;
            return request<{ data: ProblemListItem[]; total: number }>(url);
        },
        listMy: (offset = 0, limit = 20) => request<{ data: ProblemListItem[]; total: number }>(`/problems/my?offset=${offset}&limit=${limit}`),
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
            const headers: Record<string, string> = {}
            const token = getAccessToken()
            if (token) headers['Authorization'] = `Bearer ${token}`
            const res = await fetch(BASE + `/problems/${slug}/testcases`, {
                method: 'POST',
                headers,
                body: formData
            })
            if (!res.ok) {
                const text = await res.text()
                throw new Error(text || `HTTP ${res.status}`)
            }
            if (res.status === 204 || res.status === 200) {
                const text = await res.text()
                if (!text) return null
                try {
                    return JSON.parse(text)
                } catch {
                    return text
                }
            }
            return res.json()
        },
        importProblem: async (file: File) => {
            const formData = new FormData()
            formData.append('file', file)
            const headers: Record<string, string> = {}
            const token = getAccessToken()
            if (token) headers['Authorization'] = `Bearer ${token}`
            const res = await fetch(BASE + '/problems/import', {
                method: 'POST',
                headers,
                body: formData
            })
            if (!res.ok) {
                const text = await res.text()
                throw new Error(text || `HTTP ${res.status}`)
            }
            return res.json() as Promise<{ status: string; problem_id: string; slug: string }>
        },
        exportProblemUrl: (slug: string) => BASE + `/problems/${slug}/export`,
        importCodeforces: async (contestId: string, problemIndex: string) => {
            const headers: Record<string, string> = { 'Content-Type': 'application/json' }
            const token = getAccessToken()
            if (token) headers['Authorization'] = `Bearer ${token}`
            const res = await fetch(BASE + '/problems/import/codeforces', {
                method: 'POST',
                headers,
                body: JSON.stringify({ contest_id: contestId, problem_index: problemIndex })
            })
            if (!res.ok) {
                const text = await res.text()
                throw new Error(text || `HTTP ${res.status}`)
            }
            return res.json() as Promise<{ status: string; problem_id: string; slug: string }>
        },
        importCSES: async (problemId: string) => {
            const headers: Record<string, string> = { 'Content-Type': 'application/json' }
            const token = getAccessToken()
            if (token) headers['Authorization'] = `Bearer ${token}`
            const res = await fetch(BASE + '/problems/import/cses', {
                method: 'POST',
                headers,
                body: JSON.stringify({ problem_id: problemId })
            })
            if (!res.ok) {
                const text = await res.text()
                throw new Error(text || `HTTP ${res.status}`)
            }
            return res.json() as Promise<{ status: string; problem_id: string; slug: string }>
        },
        importAtCoder: async (contestId: string, problemId: string) => {
            const headers: Record<string, string> = { 'Content-Type': 'application/json' }
            const token = getAccessToken()
            if (token) headers['Authorization'] = `Bearer ${token}`
            const res = await fetch(BASE + '/problems/import/atcoder', {
                method: 'POST',
                headers,
                body: JSON.stringify({ contest_id: contestId, problem_id: problemId })
            })
            if (!res.ok) {
                const text = await res.text()
                throw new Error(text || `HTTP ${res.status}`)
            }
            return res.json() as Promise<{ status: string; problem_id: string; slug: string }>
        },
        importToph: async (problemId: string) => {
            const headers: Record<string, string> = { 'Content-Type': 'application/json' }
            const token = getAccessToken()
            if (token) headers['Authorization'] = `Bearer ${token}`
            const res = await fetch(BASE + '/problems/import/toph', {
                method: 'POST',
                headers,
                body: JSON.stringify({ problem_id: problemId })
            })
            if (!res.ok) {
                const text = await res.text()
                throw new Error(text || `HTTP ${res.status}`)
            }
            return res.json() as Promise<{ status: string; problem_id: string; slug: string }>
        },
        importQOJ: async (problemId: string) => {
            const headers: Record<string, string> = { 'Content-Type': 'application/json' }
            const token = getAccessToken()
            if (token) headers['Authorization'] = `Bearer ${token}`
            const res = await fetch(BASE + '/problems/import/qoj', {
                method: 'POST',
                headers,
                body: JSON.stringify({ problem_id: problemId })
            })
            if (!res.ok) {
                const text = await res.text()
                throw new Error(text || `HTTP ${res.status}`)
            }
            return res.json() as Promise<{ status: string; problem_id: string; slug: string }>
        },
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
        get: (id: string) => request<Submission>(`/submissions/${id}`),
        retryRemote: (id: string) => request<{ status: string }>(`/submissions/${id}/retry-remote`, { method: 'POST' }),
        recheckRemote: (id: string) => request<{ status: string }>(`/submissions/${id}/recheck-remote`, { method: 'POST' }),
        list: (offset = 0, limit = 20, problemId?: string, contestId?: string) => {
            let url = `/submissions?offset=${offset}&limit=${limit}`;
            if (problemId) url += `&problem_id=${problemId}`;
            if (contestId) url += `&contest_id=${contestId}`;
            return request<{ data: Submission[]; total: number }>(url);
        },
    },
    admin: {
        listUsers: (offset = 0, limit = 20) =>
            request<{ data: User[]; total: number }>(`/admin/users?offset=${offset}&limit=${limit}`),
        updateRole: (userId: string, role: string) =>
            request(`/admin/users/${userId}/role`, { method: 'PUT', body: JSON.stringify({ role }) }),
        listApps: () => request<{ data: SetterApplication[] }>('/admin/setter-applications'),
        reviewApp: (userId: string, status: string) =>
            request(`/admin/setter-applications/${userId}/review`, { method: 'POST', body: JSON.stringify({ status }) }),
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
            create: (d: { name: string; endpoint: string; api_key: string; model_name: string; enabled: boolean; priority: number; description: string }) =>
                request<AIModel>('/admin/ai-models', { method: 'POST', body: JSON.stringify(d) }),
            update: (id: string, d: { name: string; endpoint: string; api_key: string; model_name: string; enabled: boolean; priority: number; description: string }) =>
                request<AIModel>(`/admin/ai-models/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
            toggle: (id: string) =>
                request<{ id: string; enabled: boolean }>(`/admin/ai-models/${id}/toggle`, { method: 'PUT' }),
            delete: (id: string) =>
                request(`/admin/ai-models/${id}`, { method: 'DELETE' }),
            testConnection: (d: { endpoint: string; api_key: string; model_name: string }) =>
                request<{ ok: boolean; error?: string }>('/admin/ai-models/test', { method: 'POST', body: JSON.stringify(d) }),
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
        get: (id: string) => request<ContestDetailResponse>(`/contests/${id}`),
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
            request<{ problem: Problem; contest: Contest; can_submit?: boolean; upsolving_disabled?: boolean; statement_hidden?: boolean; is_judge?: boolean }>(`/contests/${contestId}/problems/${index}`),
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
        announcements: (id: string) => request<ContestNotice[]>(`/contests/${id}/notices`),
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
    },
    clarifications: {
        list: (contestId: string) =>
            request<{ data: Clarification[] }>(`/contests/${contestId}/clarifications`),
        create: (contestId: string, d: { problem_id?: string; question: string }) =>
            request<Clarification>(`/contests/${contestId}/clarifications`, { method: 'POST', body: JSON.stringify(d) }),
        answer: (contestId: string, id: string, d: { answer: string; is_public: boolean }) =>
            request<Clarification>(`/contests/${contestId}/clarifications/${id}/answer`, { method: 'POST', body: JSON.stringify(d) }),
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
        loginAsTeam: (contestId: string, d: unknown) =>
            request<unknown>(`/contests/${contestId}/onsite/login`, { method: 'POST', body: JSON.stringify(d) }),
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
        listPairs: (contestId: string, reportId: string, offset = 0, limit = 50) =>
            request<{ data: PlagiarismPair[]; total: number }>(
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
    generate: {
        problem: (d: {
            tags: string[]
            rating: number
            problem_type: string
            testcase_count: number
            has_subtasks: boolean
            additional_prompt?: string
        }) =>
            request<{ problem_id: string; problem_slug: string; editorial_id: string }>(
                '/generate/problem',
                { method: 'POST', body: JSON.stringify(d) }
            ),
    },
}
