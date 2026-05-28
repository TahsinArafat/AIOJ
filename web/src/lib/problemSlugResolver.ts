import { api } from './api'

// Cache: problem UUID → { slug, title }
let cache: Map<string, { slug: string; title: string }> | null = null
let loading: Promise<void> | null = null

async function ensureLoaded() {
    if (cache) return
    if (loading) return loading
    loading = (async () => {
        const map = new Map<string, { slug: string; title: string }>()
        try {
            const result = await api.problems.list(0, 1000)
            for (const p of result.data) {
                map.set(p.id, { slug: p.slug, title: p.title })
            }
        } catch {
            // fallback: map stays empty
        }
        cache = map
        loading = null
    })()
    return loading
}

export async function resolveProblemSlug(problemId: string): Promise<string | null> {
    await ensureLoaded()
    return cache?.get(problemId)?.slug ?? null
}

export async function resolveProblemTitle(problemId: string): Promise<string | null> {
    await ensureLoaded()
    return cache?.get(problemId)?.title ?? null
}
