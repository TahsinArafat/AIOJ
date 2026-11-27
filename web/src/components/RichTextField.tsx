import { lazy, Suspense } from 'react'
import MarkdownContent from './MarkdownContent'
import 'katex/dist/katex.min.css'

// The tiptap/lowlight/turndown stack is by far the heaviest part of the app.
// Lazy-loading it defers the editor bundle until a field actually mounts.
const VisualEditor = lazy(() => import('./VisualEditor'))

export interface RichTextFieldProps {
    /** Markdown source. The editor reads and writes this same value. */
    value: string
    onChange: (markdown: string) => void
    /** Accessible name, and the label when `label` is not given. */
    ariaLabel: string
    label?: string
    placeholder?: string
    rows?: number
    /** Minimum height of the editing surface. */
    minHeight?: string
    required?: boolean
    className?: string
}

/**
 * Every rich-text box in the app. Statements, translations, editorials and blog
 * posts all render this, so a change to the editing or rendering behaviour lands
 * everywhere at once instead of in whichever surface was updated first.
 *
 * `rows` is honoured by the plain textarea the editor falls back to while its
 * bundle loads; the WYSIWYG editor sizes itself.
 */
export default function RichTextField({
    value,
    onChange,
    ariaLabel,
    label,
    placeholder,
    rows = 6,
    minHeight,
    required,
    className = '',
}: RichTextFieldProps) {
    return (
        <div className={className}>
            {(label || required) && (
                <label className="block text-sm font-medium text-ink-soft mb-1">
                    {label || ariaLabel}
                    {required ? ' *' : ''}
                </label>
            )}
            <Suspense
                fallback={
                    <textarea
                        aria-label={ariaLabel}
                        value={value}
                        onChange={e => onChange(e.target.value)}
                        placeholder={placeholder}
                        rows={rows}
                        required={required}
                        className="w-full border rounded px-3 py-2 bg-surface border-hairline-strong font-mono text-sm"
                    />
                }
            >
                <VisualEditor
                    content={value}
                    onChange={onChange}
                    placeholder={placeholder}
                    minHeight={minHeight}
                />
            </Suspense>
        </div>
    )
}

interface RichTextPreviewProps {
    value: string
    className?: string
    /** Shown when there is nothing to render yet. */
    empty?: string
    label?: string
}

/**
 * The read side of the same source of truth: renders the markdown the field
 * above writes, with the same plugins, so a preview never disagrees with the
 * published page.
 */
export function RichTextPreview({ value, className = '', empty, label }: RichTextPreviewProps) {
    return (
        <div className={className}>
            {label && (
                <h4 className="text-xs font-bold text-muted-soft uppercase tracking-wider mb-2">{label}</h4>
            )}
            {value.trim() ? (
                <MarkdownContent content={value} />
            ) : (
                empty && <p className="text-muted-soft text-sm italic">{empty}</p>
            )}
        </div>
    )
}