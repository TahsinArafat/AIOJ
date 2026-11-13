import { useState, useEffect } from 'react'
import { useNavigate, Link, useLocation } from 'react-router-dom'
import { api, getAccessToken, contestSlug } from '../lib/api'
import { loginHref } from '../lib/authRedirect'
import { PageHeader } from '../components/ui/PageHeader'

/**
 * Create a round from its name, then hand it to the workspace.
 *
 * This page used to collect six sections — basic info, schedule, format and its
 * per-format tuning, a problem picker, and options — before it would submit. Every
 * one of those is also a field on the workspace Settings tab, and the problem
 * picker duplicated the Challenges tab. Two screens for one job, with the create
 * page's copies already drifting: it offered `educational` as a contest type the
 * settings form did not, and its format list was a hand-written constant while the
 * settings form asks the server.
 *
 * So it asks for the one thing the workspace cannot invent — what the round is
 * called — and creates it. The rest is configured where it can be seen in the
 * context of the round it applies to.
 *
 * The server defaults the schedule (one hour from now, one hour long), so a
 * name-only round is valid and reads as UPCOMING rather than ended. That matters:
 * start_time is NOT NULL, and Go's zero time is year 1, which every status
 * derivation would report as long finished.
 */
function slugify(text: string): string {
    return text
        .toLowerCase()
        .replace(/[^a-z0-9\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '')
        .slice(0, 80)
}

export default function ContestCreate() {
    const nav = useNavigate()
    const location = useLocation()

    const [title, setTitle] = useState('')
    const [slug, setSlug] = useState('')
    const [slugEdited, setSlugEdited] = useState(false)
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)

    // Derived rather than an effect: mirrors slugify(title) until edited by hand.
    const effectiveSlug = slugEdited ? slug : slugify(title)

    const login = loginHref(location)
    useEffect(() => {
        if (!getAccessToken()) nav(login)
    }, [nav, login])

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!title.trim()) {
            setError('Give the round a name.')
            return
        }
        setError('')
        setSubmitting(true)
        try {
            const contest = await api.contests.create({
                title: title.trim(),
                slug: effectiveSlug || undefined,
            })
            // Straight to the workspace, not the public page. The round is a draft at
            // this point, so the public page has nothing to show — and the next thing
            // a setter needs is the Settings tab, where the schedule and the problems
            // live.
            nav(`/setter/contest/${contestSlug(contest)}/manage#settings`)
        } catch (err) {
            setError((err instanceof Error ? err.message : String(err)) || 'Failed to create contest.')
        } finally {
            setSubmitting(false)
        }
    }

    const inputCls = 'w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand transition'
    const labelCls = 'block text-sm font-medium text-ink-soft mb-1'

    return (
        <div className="max-w-xl mx-auto pb-12">
            <PageHeader
                eyebrow={<Link to="/setter" className="hover:text-ink">← Setter workspace</Link>}
                title="Create contest"
                description="Name the round, then set up its schedule, problems and access in the workspace."
            />

            {error && (
                <div role="alert" className="mb-6 rounded-lg border border-bad/30 bg-bad/10 px-4 py-3 text-sm text-bad">
                    {error}
                </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-5">
                <section className="bg-surface border border-hairline rounded-lg p-6 space-y-4">
                    <div>
                        <label className={labelCls}>
                            Name <span className="text-bad">*</span>
                        </label>
                        <input
                            aria-label="Title"
                            required
                            autoFocus
                            value={title}
                            onChange={e => setTitle(e.target.value)}
                            placeholder="e.g. AIOJ Round 1 - Division 2"
                            className={inputCls}
                        />
                    </div>
                    <div>
                        <label className={labelCls}>URL Slug <span className="text-muted-soft">(optional)</span></label>
                        <div className="flex items-center gap-0">
                            <span className="inline-flex items-center px-3 py-2 rounded-l-md border border-r-0 border-hairline-strong bg-surface-2/20 text-muted text-sm">
                                /contests/
                            </span>
                            <input
                                aria-label="URL Slug"
                                value={effectiveSlug}
                                onChange={e => { setSlug(e.target.value); setSlugEdited(true) }}
                                placeholder="auto-generated-from-name"
                                pattern="[a-z0-9\-]*"
                                className="flex-1 border border-hairline-strong rounded-r-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand transition"
                            />
                        </div>
                        <p className="mt-1 text-xs text-muted-soft">Letters, numbers and hyphens. Changing it later breaks existing links.</p>
                    </div>
                </section>

                {/* What the workspace will ask for next. Not decoration: the create
                    page used to collect all of this, and a setter who does not know the
                    rest moved rather than vanished will go looking for it. */}
                <p className="text-xs text-muted-soft">
                    Next, in the workspace: <span className="text-ink-soft">schedule</span> ·{' '}
                    <span className="text-ink-soft">scoring format</span> ·{' '}
                    <span className="text-ink-soft">problems</span> ·{' '}
                    <span className="text-ink-soft">access</span>. The round starts as a draft and
                    is scheduled an hour from now until you set a real time.
                </p>

                <div className="flex items-center justify-end gap-3">
                    <button
                        type="button"
                        onClick={() => nav(-1)}
                        className="px-5 py-2.5 text-sm font-medium text-ink-soft bg-surface border border-hairline-strong rounded-lg hover:bg-surface-2 transition"
                    >
                        Cancel
                    </button>
                    <button
                        type="submit"
                        disabled={submitting}
                        className="px-6 py-2.5 text-sm font-medium text-on-brand bg-brand rounded-lg hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed transition flex items-center gap-2"
                    >
                        {submitting && <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />}
                        {submitting ? 'Creating...' : 'Create contest and continue to workspace'}
                    </button>
                </div>
            </form>
        </div>
    )
}