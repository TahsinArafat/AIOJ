export interface TestCaseResult {
    case_name?: string
    status: string
    time?: number
    memory?: number
    score?: number
    subtask_id?: number
    detail?: string
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

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        ...(opts.headers as Record<string, string> || {}),
    }
    if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`

    let res = await fetch(BASE + path, { ...opts, headers })

    if (res.status === 401 && refreshToken) {
        const ref = await fetch(BASE + '/auth/refresh', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ refresh_token: refreshToken }),
        })
        if (ref.ok) {
            const d = await ref.json()
            setTokens(d.access_token, d.refresh_token)
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
    } catch (e) {
        return text as unknown as T
    }
}

export const api = {
    auth: {
        register: (d: { username: string; email: string; password: string }) =>
            request<{ access_token: string; refresh_token: string; user: any }>('/auth/register', {
                method: 'POST',
                body: JSON.stringify(d),
            }),
        login: (d: { username: string; password: string }) =>
            request<{ access_token: string; refresh_token: string; user: any }>('/auth/login', {
                method: 'POST',
                body: JSON.stringify(d),
            }),
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
    },
    problems: {
        list: (offset = 0, limit = 20, filters?: { difficulty?: string; tags?: string[]; search?: string; source?: string }) => {
            let url = `/problems?offset=${offset}&limit=${limit}`;
            if (filters?.difficulty) url += `&difficulty=${filters.difficulty}`;
            if (filters?.tags?.length) url += `&tags=${filters.tags.join(',')}`;
            if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
            if (filters?.source) url += `&source=${encodeURIComponent(filters.source)}`;
            return request<{ data: any[]; total: number }>(url);
        },
        listTags: () => request<{ data: string[] }>('/problems/tags'),
        get: (slug: string) => request<any>(`/problems/${slug}`),
        create: (d: any) => request<any>('/problems', { method: 'POST', body: JSON.stringify(d) }),
        update: (slug: string, d: any) => request<any>(`/problems/${slug}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (slug: string) => request<any>(`/problems/${slug}`, { method: 'DELETE' }),
        getPermissions: (slug: string) => request<{ data: any[] }>(`/problems/${slug}/permissions`),
        addPermission: (slug: string, username: string, accessLevel: string) => 
            request<any>(`/problems/${slug}/permissions`, { 
                method: 'POST', 
                body: JSON.stringify({ username, access_level: accessLevel }) 
            }),
        removePermission: (slug: string, userId: string) => 
            request<any>(`/problems/${slug}/permissions/${userId}`, { method: 'DELETE' }),
        uploadTestcases: async (slug: string, file: File) => {
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
                if (!text) return null as any
                try {
                    return JSON.parse(text)
                } catch {
                    return text as any
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
    },
    submissions: {
        create: (d: any) => request<any>('/submissions', { method: 'POST', body: JSON.stringify(d) }),
        createUpsolving: (d: any) => request<any>('/submissions/upsolving', { method: 'POST', body: JSON.stringify(d) }),
        run: (d: { source_code: string; language: string; input: string; expected?: string }) => 
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
        get: (id: string) => request<any>(`/submissions/${id}`),
        list: (offset = 0, limit = 20, problemId?: string, contestId?: string) => {
            let url = `/submissions?offset=${offset}&limit=${limit}`;
            if (problemId) url += `&problem_id=${problemId}`;
            if (contestId) url += `&contest_id=${contestId}`;
            return request<{ data: any[]; total: number }>(url);
        },
    },
    admin: {
        listUsers: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/admin/users?offset=${offset}&limit=${limit}`),
        updateRole: (userId: string, role: string) =>
            request(`/admin/users/${userId}/role`, { method: 'PUT', body: JSON.stringify({ role }) }),
        listApps: () => request<{ data: any[] }>('/admin/setter-applications'),
        reviewApp: (userId: string, status: string) =>
            request(`/admin/setter-applications/${userId}/review`, { method: 'POST', body: JSON.stringify({ status }) }),
        botAccounts: {
            list: (offset = 0, limit = 20) =>
                request<{ data: any[]; total: number }>(`/admin/bot-accounts?offset=${offset}&limit=${limit}`),
            create: (d: { user_id?: string; platform: string; platform_user: string; platform_pass: string; api_key?: string; api_secret?: string; rate_limit_rps?: number; session_data?: Record<string, string> }) =>
                request<any>('/admin/bot-accounts', { method: 'POST', body: JSON.stringify(d) }),
            update: (id: string, d: { platform_user?: string; platform_pass?: string; api_key?: string; api_secret?: string; status?: string; rate_limit_rps?: number; session_data?: Record<string, string> }) =>
                request<any>(`/admin/bot-accounts/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
            delete: (id: string) =>
                request(`/admin/bot-accounts/${id}`, { method: 'DELETE' }),
            testLogin: (d: { platform: string; platform_user?: string; platform_pass?: string; session_data?: Record<string, string> }) =>
                request<{ status: string; message: string; cookies?: number }>('/admin/bot-accounts/test-login', { method: 'POST', body: JSON.stringify(d) }),
        },
        remoteLanguages: {
            list: (platform: string) =>
                request<{ data: any[] }>(`/admin/remote-languages/${platform}`),
            create: (d: { platform: string; local_id: string; remote_id: string; display_name: string; enabled?: boolean; sort_order?: number }) =>
                request<any>('/admin/remote-languages', { method: 'POST', body: JSON.stringify(d) }),
            update: (id: string, d: { local_id?: string; remote_id?: string; display_name?: string; enabled?: boolean; sort_order?: number }) =>
                request<any>(`/admin/remote-languages/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
            delete: (id: string) =>
                request(`/admin/remote-languages/${id}`, { method: 'DELETE' }),
        },
        submissions: {
            pendingRemote: () =>
                request<{ data: any[]; total: number }>('/admin/submissions/pending-remote'),
            rejudge: (id: string) =>
                request<{ status: string }>(`/admin/submissions/${id}/rejudge`, { method: 'POST' }),
            refresh: (id: string) =>
                request<{ status: string }>(`/admin/submissions/${id}/refresh`, { method: 'POST' }),
        },
        settings: {
            list: () =>
                request<{ data: { key: string; value: any; description: string; updated_at: string; updated_by: string | null }[] }>('/admin/settings'),
            update: (key: string, value: any) =>
                request(`/admin/settings/${encodeURIComponent(key)}`, { method: 'PUT', body: JSON.stringify({ value }) }),
        },
        languages: {
            list: () =>
                request<{ data: any[] }>('/admin/languages'),
            get: (key: string) =>
                request<any>(`/admin/languages/${key}`),
            getRaw: (key: string) =>
                request<string>(`/admin/languages/${key}/raw`),
            create: (d: any) =>
                request<any>('/admin/languages', { method: 'POST', body: JSON.stringify(d) }),
            update: (key: string, d: any) =>
                request<any>(`/admin/languages/${key}`, { method: 'PUT', body: JSON.stringify(d) }),
            updateRaw: (key: string, content: string) =>
                request<any>(`/admin/languages/${key}/raw`, { method: 'PUT', body: JSON.stringify({ content }) }),
            delete: (key: string) =>
                request(`/admin/languages/${key}`, { method: 'DELETE' }),
            test: (key: string) =>
                request<any>(`/admin/languages/${key}/test`, { method: 'POST' }),
            detect: () =>
                request<{ compilers: any[]; interpreters: any[] }>('/admin/languages/detect'),
            templates: () =>
                request<{ data: any[] }>('/admin/languages/templates'),
        },
    },
    setter: {
        apply: (reason: string) => request('/auth/setter-apply', { method: 'POST', body: JSON.stringify({ reason }) }),
        status: () => request<any>('/auth/setter-status'),
    },
    contests: {
        list: (offset = 0, limit = 20, division?: number) => {
            let url = `/contests?offset=${offset}&limit=${limit}`;
            if (division !== undefined) url += `&division=${division}`;
            return request<{ data: any[]; total: number }>(url);
        },
        get: (id: string) => request<any>(`/contests/${id}`),
        create: (d: any) => request<any>('/contests', { method: 'POST', body: JSON.stringify(d) }),
        getFormats: () => request<{ formats: string[] }>('/contests/formats'),
        scoreboard: (id: string) => request<any>(`/contests/${id}/scoreboard`),
        register: (id: string) => request(`/contests/${id}/register`, { method: 'POST' }),
        unregister: (id: string) => request(`/contests/${id}/register`, { method: 'DELETE' }),
        checkRegistration: (id: string) => request<{ registered: boolean }>(`/contests/${id}/register`),
        listRegistrations: (id: string) => request<{ data: any[]; count: number }>(`/contests/${id}/registrations`),
    },
    virtual: {
        start: (contestId: string, durationMinutes?: number) =>
            request<any>('/virtual/start', { method: 'POST', body: JSON.stringify({ contest_id: contestId, duration_minutes: durationMinutes }) }),
        status: () => request<any>('/virtual/status'),
        complete: (id: string) => request(`/virtual/${id}/complete`, { method: 'POST' }),
    },
    gym: {
        list: (offset = 0, limit = 20, filters?: { category?: string; search?: string }) => {
            let url = `/gym?offset=${offset}&limit=${limit}`;
            if (filters?.category) url += `&category=${filters.category}`;
            if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
            return request<{ data: any[]; total: number }>(url);
        },
        get: (id: string) => request<any>(`/gym/${id}`),
        create: (d: any) => request<any>('/gym', { method: 'POST', body: JSON.stringify(d) }),
        markSolved: (id: string) => request(`/gym/${id}/solve`, { method: 'POST' }),
    },
    hacks: {
        submit: (d: { contest_id: string; problem_id: string; submission_id: string; test_input: string }) =>
            request<any>('/hacks', { method: 'POST', body: JSON.stringify(d) }),
        get: (id: string) => request<any>(`/hacks/${id}`),
        listByContest: (contestId: string) => request<any>(`/hacks/contest/${contestId}`),
        listHackable: (contestId: string, problemId: string) => request<any>(`/hacks/hackable/${contestId}/${problemId}`),
    },
    stats: {
        getPlatform: () => request<{ problems: number; users: number; submissions: number }>('/stats/platform'),
        getProblemStats: (problemId: string) => request<any>(`/stats/problems/${problemId}`),
        getMyStats: () => request<any>('/stats/me'),
        getUserStats: (userId: string) => request<any>(`/stats/user/${userId}`),
    },
    notifications: {
        list: (unreadOnly = false, limit = 50) =>
            request<{ data: any[] }>(`/notifications?unread=${unreadOnly}&limit=${limit}`),
        unreadCount: () => request<{ count: number }>('/notifications/unread-count'),
        markAsRead: (id: string) => request(`/notifications/${id}/read`, { method: 'POST' }),
        markAllAsRead: () => request('/notifications/read-all', { method: 'POST' }),
        getPreferences: () => request<any>('/notifications/preferences'),
        updatePreferences: (prefs: any) => request('/notifications/preferences', { method: 'PUT', body: JSON.stringify(prefs) }),
    },
    groups: {
        list: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/groups?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<any>(`/groups/${id}`),
        create: (d: any) => request<any>('/groups', { method: 'POST', body: JSON.stringify(d) }),
        join: (id: string) => request(`/groups/${id}/join`, { method: 'POST' }),
        leave: (id: string) => request(`/groups/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<any>(`/groups/${id}/members`),
        addContest: (id: string, contestId: string) =>
            request(`/groups/${id}/contests`, { method: 'POST', body: JSON.stringify({ contest_id: contestId }) }),
    },
    teams: {
        list: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/teams?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<any>(`/teams/${id}`),
        create: (d: any) => request<any>('/teams', { method: 'POST', body: JSON.stringify(d) }),
        join: (id: string) => request(`/teams/${id}/join`, { method: 'POST' }),
        leave: (id: string) => request(`/teams/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<any>(`/teams/${id}/members`),
    },
    blog: {
        list: (offset = 0, limit = 20, tag?: string) => {
            let url = `/blog?offset=${offset}&limit=${limit}`;
            if (tag) url += `&tag=${encodeURIComponent(tag)}`;
            return request<{ data: any[]; total: number }>(url);
        },
        get: (id: string) => request<any>(`/blog/${id}`),
        create: (d: any) => request<any>('/blog', { method: 'POST', body: JSON.stringify(d) }),
        getComments: (type: string, id: string) => request<{ data: any[] }>(`/blog/${type}/${id}/comments`),
        createComment: (d: any) => request<any>('/blog/comments', { method: 'POST', body: JSON.stringify(d) }),
        vote: (d: { target_type: string; target_id: string; value: number }) =>
            request('/blog/vote', { method: 'POST', body: JSON.stringify(d) }),
    },
    editorials: {
        list: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/editorials?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<any>(`/editorials/${id}`),
        getByProblem: (problemId: string) => request<{ data: any[] }>(`/editorials/problem/${problemId}`),
        create: (d: any) => request<any>('/editorials', { method: 'POST', body: JSON.stringify(d) }),
    },
    apiKeys: {
        list: () => request<{ data: any[] }>('/keys'),
        create: (d: { name: string; description?: string }) =>
            request<{ api_key: any; secret: string }>('/keys', { method: 'POST', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/keys/${id}`, { method: 'DELETE' }),
    },
    webhooks: {
        list: () => request<{ data: any[] }>('/webhooks'),
        create: (d: { url: string; secret?: string; events: string[] }) =>
            request<any>('/webhooks', { method: 'POST', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/webhooks/${id}`, { method: 'DELETE' }),
    },
    recommendations: {
        get: (rating?: number) =>
            request<{
                progression: any[];
                weak_tags: { tags: string[]; problems: any[] };
                hybrid: any[];
            }>(rating !== undefined ? `/recommendations?rating=${rating}` : '/recommendations'),
    },
    organizations: {
        list: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/organizations?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<any>(`/organizations/${id}`),
        create: (d: { name: string; description: string }) =>
            request<any>('/organizations', { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: { name: string; description: string }) =>
            request<any>(`/organizations/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/organizations/${id}`, { method: 'DELETE' }),
        join: (id: string) => request(`/organizations/${id}/join`, { method: 'POST' }),
        leave: (id: string) => request(`/organizations/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<{ data: any[] }>(`/organizations/${id}/members`),
        addMember: (id: string, userId: string, role: string) =>
            request(`/organizations/${id}/members`, { method: 'POST', body: JSON.stringify({ user_id: userId, role }) }),
        removeMember: (id: string, userId: string) =>
            request(`/organizations/${id}/members/${userId}`, { method: 'DELETE' }),
        my: () => request<{ data: any[] }>('/organizations/my'),
    },
    classes: {
        list: (orgId: string, offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/organizations/${orgId}/classes?offset=${offset}&limit=${limit}`),
        get: (id: string) => request<any>(`/classes/${id}`),
        create: (orgId: string, d: { name: string; description: string }) =>
            request<any>(`/organizations/${orgId}/classes`, { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: { name: string; description: string }) =>
            request<any>(`/classes/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/classes/${id}`, { method: 'DELETE' }),
        joinByCode: (inviteCode: string) =>
            request<any>('/classes/join', { method: 'POST', body: JSON.stringify({ invite_code: inviteCode }) }),
        leave: (id: string) => request(`/classes/${id}/leave`, { method: 'POST' }),
        members: (id: string) => request<{ data: any[] }>(`/classes/${id}/members`),
    },
    training: {
        list: (offset = 0, limit = 20, opts?: { orgId?: string; public?: boolean }) => {
            let url = `/training?offset=${offset}&limit=${limit}`;
            if (opts?.orgId) url += `&org_id=${opts.orgId}`;
            if (opts?.public) url += `&public=true`;
            return request<{ data: any[]; total: number }>(url);
        },
        get: (id: string) => request<any>(`/training/${id}`),
        create: (d: any) => request<any>('/training', { method: 'POST', body: JSON.stringify(d) }),
        update: (id: string, d: any) => request<any>(`/training/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
        delete: (id: string) => request(`/training/${id}`, { method: 'DELETE' }),
        enroll: (id: string) => request(`/training/${id}/enroll`, { method: 'POST' }),
        unenroll: (id: string) => request(`/training/${id}/enroll`, { method: 'DELETE' }),
        enrollments: (id: string) => request<{ data: any[] }>(`/training/${id}/enrollments`),
        progress: (id: string) => request<any>(`/training/${id}/progress`),
        addSection: (planId: string, d: { title: string; description: string }) =>
            request<any>(`/training/${planId}/sections`, { method: 'POST', body: JSON.stringify(d) }),
        deleteSection: (sectionId: string) =>
            request(`/training/sections/${sectionId}`, { method: 'DELETE' }),
        addProblem: (sectionId: string, d: { problem_id: string; points: number }) =>
            request<any>(`/training/sections/${sectionId}/problems`, { method: 'POST', body: JSON.stringify(d) }),
        removeProblem: (problemId: string) =>
            request(`/training/problems/${problemId}`, { method: 'DELETE' }),
    },
    ratings: {
        getByUser: (userId: string, limit = 50) =>
            request<{ data: any[] }>(`/rating/user/${userId}?limit=${limit}`),
        getByContest: (contestId: string) =>
            request<{ data: any[] }>(`/rating/contest/${contestId}`),
    },
    plagiarism: {
        runCheck: (contestId: string, threshold?: number) =>
            request<any>(`/contests/${contestId}/plagiarism/check`, {
                method: 'POST',
                body: JSON.stringify({ contest_id: contestId, threshold }),
            }),
        getReport: (contestId: string) =>
            request<any>(`/contests/${contestId}/plagiarism/report`),
        listPairs: (contestId: string, reportId: string, offset = 0, limit = 50) =>
            request<{ data: any[]; total: number }>(
                `/contests/${contestId}/plagiarism/report/${reportId}/pairs?offset=${offset}&limit=${limit}`
            ),
        updatePair: (contestId: string, pairId: string, status: string) =>
            request(`/contests/${contestId}/plagiarism/pairs/${pairId}`, {
                method: 'PUT',
                body: JSON.stringify({ status }),
            }),
    },
    rankings: {
        list: (offset = 0, limit = 50) =>
            request<{ data: any[]; total: number }>(`/rankings?offset=${offset}&limit=${limit}`),
    },
    users: {
        getByUsername: (username: string) =>
            request<any>(`/users/${encodeURIComponent(username)}`),
    },
    search: {
        global: (q: string, limit = 10) =>
            request<{ problems: any[]; users: any[]; contests: any[] }>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),
    },
}
