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
    return res.json()
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
    },
    problems: {
        list: (offset = 0, limit = 20, filters?: { difficulty?: string; tags?: string[]; search?: string }) => {
            let url = `/problems?offset=${offset}&limit=${limit}`;
            if (filters?.difficulty) url += `&difficulty=${filters.difficulty}`;
            if (filters?.tags?.length) url += `&tags=${filters.tags.join(',')}`;
            if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
            return request<{ data: any[]; total: number }>(url);
        },
        listTags: () => request<{ data: string[] }>('/problems/tags'),
        get: (slug: string) => request<any>(`/problems/${slug}`),
        create: (d: any) => request<any>('/problems', { method: 'POST', body: JSON.stringify(d) }),
    },
    submissions: {
        create: (d: any) => request<any>('/submissions', { method: 'POST', body: JSON.stringify(d) }),
        createUpsolving: (d: any) => request<any>('/submissions/upsolving', { method: 'POST', body: JSON.stringify(d) }),
        get: (id: string) => request<any>(`/submissions/${id}`),
        list: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/submissions?offset=${offset}&limit=${limit}`),
    },
    admin: {
        listUsers: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/admin/users?offset=${offset}&limit=${limit}`),
        updateRole: (userId: string, role: string) =>
            request(`/admin/users/${userId}/role`, { method: 'PUT', body: JSON.stringify({ role }) }),
        listApps: () => request<{ data: any[] }>('/admin/setter-applications'),
        reviewApp: (userId: string, status: string) =>
            request(`/admin/setter-applications/${userId}/review`, { method: 'POST', body: JSON.stringify({ status }) }),
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
        getProblemStats: (problemId: string) => request<any>(`/stats/problems/${problemId}`),
        getMyStats: () => request<any>('/stats/me'),
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
}
