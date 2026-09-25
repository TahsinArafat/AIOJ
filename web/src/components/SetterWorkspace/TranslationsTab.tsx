import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { LOCALES, DEFAULT_LOCALE } from '../../i18n/locales'

/**
 * Lets a setter supply per-language titles/descriptions for a problem.
 *
 * The backend (migration 000056) stores these in `problem_i18n`; without this
 * tab the only way to create a translation was the raw API. Language options
 * come from the shared locale registry so this stays in step with the switcher.
 */

interface Props {
  problemId: string
  /** The problem's default text, shown as placeholder so the setter can see what's being translated. */
  defaultTitle?: string
  defaultDescription?: string
  onSaved?: () => void
}

interface TranslationRow {
  language: string
  title?: string | null
  description?: string | null
}

export default function TranslationsTab({ problemId, defaultTitle, defaultDescription, onSaved }: Props) {
  // The default language is intentionally excluded: its text lives on the
  // problem itself, so a row here would be ambiguous about which wins.
  const translatable = LOCALES.filter((l) => l.code !== DEFAULT_LOCALE)

  const [active, setActive] = useState(translatable[0]?.code ?? '')
  const [rows, setRows] = useState<Record<string, TranslationRow>>({})
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (!problemId) return
    let cancelled = false
    // deferred a microtask: spinner lands pre-paint (react-hooks/set-state-in-effect)
    queueMicrotask(() => setLoading(true))
    api.problems
      .listI18n(problemId)
      .then((list) => {
        if (cancelled) return
        const map: Record<string, TranslationRow> = {}
        for (const r of list || []) map[r.language] = r
        setRows(map)
        setError('')
      })
      .catch(() => { if (!cancelled) setError('Failed to load translations') })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [problemId])

  // Load the selected language's existing text into the form.
  useEffect(() => {
    const row = rows[active]
    // deferred a microtask: fields sync pre-paint (react-hooks/set-state-in-effect)
    queueMicrotask(() => {
      setTitle(row?.title ?? '')
      setDescription(row?.description ?? '')
      setSaved(false)
    })
  }, [active, rows])

  const existing = rows[active]

  const handleSave = async () => {
    if (!title.trim()) {
      setError('Title is required for a translation')
      return
    }
    setSaving(true)
    setError('')
    try {
      await api.problems.upsertI18n(problemId, active, {
        title: title.trim(),
        description,
      })
      setRows((prev) => ({ ...prev, [active]: { language: active, title, description } }))
      setSaved(true)
      onSaved?.()
    } catch {
      setError('Failed to save translation')
    } finally {
      setSaving(false)
    }
  }

  const handleRemove = async () => {
    if (!existing) return
    setSaving(true)
    setError('')
    try {
      await api.problems.deleteI18n(problemId, active)
      setRows((prev) => {
        const next = { ...prev }
        delete next[active]
        return next
      })
      setSaved(false)
      onSaved?.()
    } catch {
      setError('Failed to remove translation')
    } finally {
      setSaving(false)
    }
  }

  if (translatable.length === 0) {
    return (
      <div className="p-6 text-sm text-gray-500 dark:text-gray-400">
        No translatable languages are configured.
      </div>
    )
  }

  return (
    <div className="p-6 space-y-5">
      <div>
        <h2 className="text-lg font-semibold">Translations</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
          Provide a title and statement for each language. Solvers see the translation
          for their selected language, and fall back to the default text when none exists.
        </p>
      </div>

      {/* Language picker, with a dot marking languages that already have a translation. */}
      <div className="flex flex-wrap gap-2">
        {translatable.map(({ code, label }) => (
          <button
            key={code}
            type="button"
            onClick={() => setActive(code)}
            className={`px-3 py-1.5 rounded text-sm border ${
              active === code
                ? 'bg-blue-600 text-white border-blue-600'
                : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700'
            }`}
          >
            {label}
            {rows[code] && <span className="ml-1.5 text-emerald-500">•</span>}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="text-sm text-gray-500">Loading…</div>
      ) : (
        <div className="space-y-4 max-w-2xl">
          <div>
            <label className="block text-sm font-medium mb-1">Title</label>
            <input
              type="text"
              value={title}
              onChange={(e) => { setTitle(e.target.value); setSaved(false) }}
              placeholder={defaultTitle || 'Translated title'}
              className="w-full border rounded px-3 py-2 text-sm bg-white dark:bg-gray-800 border-gray-300 dark:border-gray-600"
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Statement</label>
            <textarea
              value={description}
              onChange={(e) => { setDescription(e.target.value); setSaved(false) }}
              placeholder={defaultDescription || 'Translated statement (Markdown supported)'}
              rows={10}
              className="w-full border rounded px-3 py-2 text-sm font-mono bg-white dark:bg-gray-800 border-gray-300 dark:border-gray-600"
            />
          </div>

          {error && (
            <div className="text-sm text-red-600 dark:text-red-400">{error}</div>
          )}
          {saved && !error && (
            <div className="text-sm text-emerald-600 dark:text-emerald-400">Saved.</div>
          )}

          <div className="flex gap-2">
            <button
              type="button"
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? 'Saving…' : 'Save translation'}
            </button>
            {existing && (
              <button
                type="button"
                onClick={handleRemove}
                disabled={saving}
                className="px-4 py-2 border border-red-300 text-red-600 rounded text-sm font-medium hover:bg-red-50 disabled:opacity-50"
              >
                Remove translation
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
