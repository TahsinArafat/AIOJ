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
        list: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/problems?offset=${offset}&limit=${limit}`),
        get: (slug: string) => request<any>(`/problems/${slug}`),
        create: (d: any) => request<any>('/problems', { method: 'POST', body: JSON.stringify(d) }),
    },
    submissions: {
        create: (d: any) => request<any>('/submissions', { method: 'POST', body: JSON.stringify(d) }),
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
}
