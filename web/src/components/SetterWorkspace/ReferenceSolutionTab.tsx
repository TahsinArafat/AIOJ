import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { FALLBACK_LANGUAGES, DEFAULT_LANGUAGE, languageOptions } from '../../lib/languages'
import CodeEditor from '../CodeEditor'

interface Props {
    problemId: string
}

/**
 * Manage the problem's reference solution.
 *
 * The solution lives on the official editorial row (the same one AI Studio's
 * apply writes), so this tab and an editorial apply update one another rather
 * than keeping two copies. Writing only the solution fields leaves any editorial
 * prose the setter wrote intact.
 */
export default function ReferenceSolutionTab({ problemId }: Props) {
    const [code, setCode] = useState('')
    const [language, setLanguage] = useState(DEFAULT_LANGUAGE)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState('')
    const [success, setSuccess] = useState('')

    useEffect(() => {
        let cancelled = false
        queueMicrotask(() => {
            setLoading(true)
            api.editorials.getSolution(problemId)
                .then(e => {
                    if (cancelled) return
                    setCode(e.solution_code || '')
                    setLanguage(e.solution_language || DEFAULT_LANGUAGE)
                })
                .catch(err => { if (!cancelled) setError(errorMessage(err)) })
                .finally(() => { if (!cancelled) setLoading(false) })
        })
        return () => { cancelled = true }
    }, [problemId])

    const save = async () => {
        setSaving(true)
        setError('')
        setSuccess('')
        try {
            await api.editorials.upsertSolution(problemId, { solution_code: code, solution_language: language })
            setSuccess('Reference solution saved.')
        } catch (err) {
            setError(errorMessage(err) || 'Failed to save reference solution')
        } finally {
            setSaving(false)
        }
    }

    if (loading) {
        return <div className="text-center py-10 text-muted">Loading reference solution…</div>
    }

    return (
        <div className="space-y-4 text-ink">
            <div>
                <h2 className="text-lg font-semibold">Reference Solution</h2>
                <p className="text-xs text-muted">
                    The model solution contestants are judged against. Stored on the official editorial, so
                    applying a solution from AI Studio updates this same code.
                </p>
            </div>

            {error && <div className="rounded border border-bad/40 bg-bad/10 p-3 text-sm text-bad">{error}</div>}
            {success && <div className="rounded border border-ok/40 bg-ok/10 p-3 text-sm text-ok">{success}</div>}

            <div>
                <label className="block text-[10px] font-bold text-muted-soft uppercase tracking-wider mb-1">Language</label>
                <select
                    value={language}
                    onChange={e => setLanguage(e.target.value)}
                    className="border border-hairline-strong rounded px-3 py-1.5 text-sm bg-surface text-ink"
                >
                    {languageOptions(FALLBACK_LANGUAGES).map(o => (
                        <option key={o.value} value={o.value}>{o.label}</option>
                    ))}
                </select>
            </div>

            <div>
                <label className="block text-[10px] font-bold text-muted-soft uppercase tracking-wider mb-1">Solution Code</label>
                <div className="border border-hairline rounded overflow-hidden shadow-inner bg-surface-2">
                    <CodeEditor language={language} value={code} onChange={setCode} height="420px" />
                </div>
            </div>

            <button
                type="button"
                onClick={save}
                disabled={saving}
                className="min-h-11 cursor-pointer rounded bg-brand px-5 text-on-brand disabled:opacity-50"
            >
                {saving ? 'Saving…' : 'Save reference solution'}
            </button>
        </div>
    )
}