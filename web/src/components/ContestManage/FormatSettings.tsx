import { formatSpec } from '../../lib/contestFormatConfig'

/**
 * The selected scoring format's own settings.
 *
 * This block is why the create page's copy could be deleted: it was the only UI
 * that could reach these numbers, so without it a round could change format and
 * never tune it.
 *
 * The fields come from the format registry rather than being hand-written per
 * format, so a format added to the registry gets its settings UI for free. A
 * format this build has no spec for says so and leaves the stored values alone,
 * rather than rendering an empty fieldset that would save as "cleared".
 *
 * Values are strings because the inputs are text: a number input bound to a
 * number cannot represent an empty box, and an empty box is how a setter clears
 * a field to retype it.
 */
export function FormatSettings({
    format,
    config,
    onChange,
}: {
    format: string
    config: Record<string, string>
    /** Applied to one key of the format config. */
    onChange: (key: string, value: string) => void
}) {
    const spec = formatSpec(format)
    if (!spec) {
        return (
            <p className="mt-2 text-xs text-warn">
                This round uses a format this build has no settings for
                ({format}). Its stored settings are left untouched.
            </p>
        )
    }

    return (
        <fieldset className="mt-3 rounded-lg border border-hairline p-3">
            <legend className="px-1 text-xs font-semibold text-ink-soft">
                {spec.label} settings
            </legend>
            <div className="mt-2 space-y-3">
                {spec.fields.map(f => (
                    <div key={f.key}>
                        {f.type === 'checkbox' ? (
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    aria-label={f.label}
                                    checked={config[f.key] === 'true'}
                                    onChange={e => onChange(f.key, e.target.checked ? 'true' : 'false')}
                                    className="rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]"
                                />
                                <span className="text-sm text-ink-soft">{f.label}</span>
                            </label>
                        ) : (
                            <>
                                <label className="block text-sm font-medium text-ink-soft mb-1">
                                    {f.label}
                                </label>
                                <input
                                    type="number"
                                    aria-label={f.label}
                                    min={f.min}
                                    max={f.max}
                                    step={f.step}
                                    value={config[f.key] ?? ''}
                                    onChange={e => onChange(f.key, e.target.value)}
                                    className="w-full max-w-xs border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand bg-surface"
                                />
                            </>
                        )}
                        {f.hint && <p className="mt-1 text-xs text-muted-soft">{f.hint}</p>}
                    </div>
                ))}
            </div>
        </fieldset>
    )
}