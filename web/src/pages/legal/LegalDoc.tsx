import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

/** Shared shell for ToS / Privacy / DMCA pages. */
export default function LegalDoc({
    slug,
    title,
}: {
    slug: 'terms_of_service' | 'privacy_policy' | 'dmca'
    title: string
}) {
    const [md, setMd] = useState<string | null>(null)
    const [err, setErr] = useState('')

    useEffect(() => {
        let cancelled = false
        fetch(`/api/legal/${slug}`)
            .then((r) => {
                if (!r.ok) throw new Error(`HTTP ${r.status}`)
                return r.text()
            })
            .then((t) => {
                if (!cancelled) setMd(t)
            })
            .catch((e: unknown) => {
                if (!cancelled) setErr(e instanceof Error ? e.message : 'failed to load')
            })
        return () => {
            cancelled = true
        }
    }, [slug])

    if (err) {
        return (
            <div className="max-w-3xl mx-auto mt-12 p-6 border border-red-200 dark:border-red-800 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300">
                Failed to load {title}: {err}
            </div>
        )
    }
    if (md === null) {
        return (
            <div className="max-w-3xl mx-auto mt-12 text-gray-600 dark:text-gray-400">Loading…</div>
        )
    }
    return (
        <article className="max-w-3xl mx-auto mt-8 mb-16">
            <h1 className="text-3xl font-bold mb-6 text-gray-900 dark:text-gray-100">{title}</h1>
            <div className="prose prose-sm dark:prose-invert max-w-none">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{md}</ReactMarkdown>
            </div>
        </article>
    )
}
