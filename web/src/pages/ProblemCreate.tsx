import { useRef, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { api } from '../lib/api'
import { PageHeader } from '../components/ui/PageHeader'
import { buttonClass } from '../components/ui/buttonStyles'

/**
 * The first step of authoring a problem: a name, nothing else.
 *
 * Everything else — statement, limits, test cases, checker — has a home in the
 * workspace this page redirects to, and asking for it twice meant the setter
 * filled in half a statement before seeing the editor. The slug is derived by
 * the server so this form stays one field wide.
 */
export default function ProblemCreate() {
    const nav = useNavigate()
    const inputRef = useRef<HTMLInputElement>(null)
    const [title, setTitle] = useState('')
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!title.trim()) return
        setError('')
        setSubmitting(true)
        try {
            const problem = await api.problems.create({ title: title.trim() })
            nav(`/setter/${problem.slug}`)
        } catch (err) {
            setError((err instanceof Error ? err.message : '') || 'Failed to create problem')
            setSubmitting(false)
            inputRef.current?.focus()
        }
    }

    return (
        <div className="max-w-xl">
            <PageHeader
                eyebrow={<Link to="/setter" className="hover:text-ink">← Setter workspace</Link>}
                title="Create problem"
                description="Name it and you are in the editor. Statement, limits and test cases all live there."
            />
            {error && (
                <div role="alert" className="mb-4 rounded-lg border border-bad/30 bg-bad/10 px-4 py-2 text-sm text-bad">
                    {error}
                </div>
            )}
            <form onSubmit={handleSubmit} className="space-y-5">
                <div>
                    <label htmlFor="problem-title" className="block text-sm font-medium text-ink-soft mb-1">Problem name</label>
                    <input
                        id="problem-title"
                        ref={inputRef}
                        // One field on the page: focusing it saves the only click
                        // the form would otherwise need.
                        autoFocus
                        required
                        value={title}
                        onChange={e => setTitle(e.target.value)}
                        placeholder="e.g. Range Sum Queries"
                        className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                    />
                </div>
                <button type="submit" disabled={submitting || !title.trim()} className={buttonClass({ size: 'lg' })}>
                    {submitting ? 'Creating…' : 'Next →'}
                </button>
            </form>
        </div>
    )
}