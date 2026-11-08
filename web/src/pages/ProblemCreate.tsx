import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { api } from '../lib/api'
import { PageHeader } from '../components/ui/PageHeader'

export default function ProblemCreate() {
    const nav = useNavigate()
    const [form, setForm] = useState({
        slug: '', title: '', description: '', difficulty: 'easy', rating: 0,
        time_limit: 1000, memory_limit: 262144, tags: '', input_format: '', output_format: '',
        constraints: '',
    })
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [touchedSlug, setTouchedSlug] = useState(false)

    const handleTitleChange = (val: string) => {
        setForm(p => {
            const next = { ...p, title: val }
            if (!touchedSlug) {
                next.slug = val
                    .toLowerCase()
                    .replace(/[^a-z0-9\s-]/g, '')
                    .replace(/\s+/g, '-')
                    .slice(0, 50)
            }
            return next
        })
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')
        setSubmitting(true)
        try {
            await api.problems.create({
                slug: form.slug, title: form.title, description: form.description,
                difficulty: form.difficulty, rating: form.rating,
                time_limit: form.time_limit, memory_limit: form.memory_limit,
                tags: form.tags.split(',').map((t: string) => t.trim()).filter(Boolean),
                input_format: form.input_format, output_format: form.output_format,
                constraints: form.constraints,
            })
            nav('/setter')
        } catch (err) {
            setError((err instanceof Error ? err.message : '') || 'Failed to create problem')
        } finally {
            setSubmitting(false)
        }
    }

    return (
        // Left-aligned like every other page: `mx-auto` on a narrow column put
        // this heading at x=74 while the rest of the app sits at x=24.
        <div className="max-w-2xl">
            <PageHeader
                eyebrow={<Link to="/setter" className="hover:text-ink">← Setter workspace</Link>}
                title="Create problem"
                description="Set up the statement and limits. Test cases and the checker come next."
            />
            {error && (
                <div role="alert" className="mb-4 rounded-lg border border-bad/30 bg-bad/10 px-4 py-2 text-sm text-bad">
                    {error}
                </div>
            )}
            <form onSubmit={handleSubmit} className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                    <div>
                        <label htmlFor="problem-slug" className="block text-sm font-medium text-ink-soft mb-1">Slug</label>
                        <input id="problem-slug" required value={form.slug} onChange={e => {
                            setForm(p => ({...p, slug: e.target.value}))
                            setTouchedSlug(true)
                        }}
                            className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                    </div>
                    <div>
                        <label htmlFor="problem-title" className="block text-sm font-medium text-ink-soft mb-1">Title</label>
                        <input id="problem-title" required value={form.title} onChange={e => handleTitleChange(e.target.value)}
                            className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                    </div>
                </div>
                <div>
                    <label htmlFor="problem-description" className="block text-sm font-medium text-ink-soft mb-1">Description</label>
                    <textarea id="problem-description" required rows={6} value={form.description} onChange={e => setForm(p => ({...p, description: e.target.value}))}
                        className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                </div>
                <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
                    <div>
                        <label htmlFor="problem-difficulty" className="block text-sm font-medium text-ink-soft mb-1">Difficulty</label>
                        <select id="problem-difficulty" value={form.difficulty} onChange={e => setForm(p => ({...p, difficulty: e.target.value}))}
                            className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]">
                            <option value="easy">Easy</option><option value="medium">Medium</option><option value="hard">Hard</option>
                        </select>
                    </div>
                    <div>
                        <label htmlFor="problem-rating" className="block text-sm font-medium text-ink-soft mb-1">Rating</label>
                        <input id="problem-rating" type="number" min={0} max={3500} step={100} placeholder="0 = unrated"
                            value={form.rating} onChange={e => setForm(p => ({...p, rating: +e.target.value}))}
                            className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                    </div>
                    <div>
                        <label htmlFor="problem-time-limit-ms" className="block text-sm font-medium text-ink-soft mb-1">Time Limit (ms)</label>
                        <input id="problem-time-limit-ms" type="number" value={form.time_limit} onChange={e => setForm(p => ({...p, time_limit: +e.target.value}))}
                            className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                    </div>
                    <div>
                        <label htmlFor="problem-memory-limit-kb" className="block text-sm font-medium text-ink-soft mb-1">Memory Limit (KB)</label>
                        <input id="problem-memory-limit-kb" type="number" value={form.memory_limit} onChange={e => setForm(p => ({...p, memory_limit: +e.target.value}))}
                            className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                    </div>
                </div>
                <div>
                    <label htmlFor="problem-tags-comma-separated" className="block text-sm font-medium text-ink-soft mb-1">Tags (comma-separated)</label>
                    <input id="problem-tags-comma-separated" value={form.tags} onChange={e => setForm(p => ({...p, tags: e.target.value}))}
                        placeholder="e.g. dp, graph, math" className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                </div>
                <div>
                    <label htmlFor="problem-input-format" className="block text-sm font-medium text-ink-soft mb-1">Input Format</label>
                    <textarea id="problem-input-format" rows={2} value={form.input_format} onChange={e => setForm(p => ({...p, input_format: e.target.value}))}
                        className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                </div>
                <div>
                    <label htmlFor="problem-output-format" className="block text-sm font-medium text-ink-soft mb-1">Output Format</label>
                    <textarea id="problem-output-format" rows={2} value={form.output_format} onChange={e => setForm(p => ({...p, output_format: e.target.value}))}
                        className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                </div>
                <div>
                    <label htmlFor="problem-constraints" className="block text-sm font-medium text-ink-soft mb-1">Constraints</label>
                    <textarea id="problem-constraints" rows={2} value={form.constraints} onChange={e => setForm(p => ({...p, constraints: e.target.value}))}
                        placeholder="e.g. $1 \le n \le 2\cdot10^5$"
                        className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                    <p className="mt-1 text-xs text-muted">LaTeX bounds, one per line. Rendered as its own section on the problem page.</p>
                </div>
                <button type="submit" disabled={submitting}
                    className="bg-brand text-on-brand px-6 py-2 rounded hover:bg-brand-hover disabled:opacity-50 transition-colors">
                    {submitting ? 'Creating...' : 'Create Problem'}
                </button>
            </form>
        </div>
    )
}
