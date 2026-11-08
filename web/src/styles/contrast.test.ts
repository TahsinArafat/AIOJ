import { expect, test, describe } from 'vitest'
// tsconfig.app.json deliberately has no `@types/node` (the app targets the
// browser), so the two Node names below need suppression. Both resolve at
// runtime, which is what matters: this file only runs under vitest, on Node.
// @ts-expect-error node builtin, no @types/node in this project
import { readFileSync } from 'node:fs'

/**
 * Computed-contrast guard for the theme tokens.
 *
 * The bug this exists for: inline code in a problem statement rendered as the
 * typography plugin's slate-900 on the dark `--surface-3`, roughly 1.2:1 —
 * invisible. Neither colour is wrong on its own, so nothing failed review; only
 * the *pair* was broken. Asserting the pair in both palettes makes that class of
 * mistake impossible to re-introduce silently.
 *
 * Values come from global.css rather than being duplicated here, so the test
 * fails if either side of a pair is retuned into an unreadable combination.
 *
 * `import css from '../global.css?raw'` does NOT work: vitest stubs CSS modules,
 * so the string arrives empty and every assertion passes vacuously.
 */
// `import.meta.url` is not a file: URL under vitest (modules are served), so the
// path is taken from the process cwd instead — vitest's root is `web/`, one
// level above `src/`. The parse guard below fails loudly if this ever drifts.
// @ts-expect-error `process` is a Node global; see the note on the fs import
const css = readFileSync(`${process.cwd()}/src/global.css`, 'utf8')

/** Extracts the declarations of the first `selector { … }` block. */
function blockColors(selector: string): Record<string, string> {
    const start = css.indexOf(`${selector} {`)
    if (start === -1) throw new Error(`selector not found in global.css: ${selector}`)
    const end = css.indexOf('}', start)
    const body = css.slice(start, end)
    const out: Record<string, string> = {}
    for (const m of body.matchAll(/(--[\w-]+):\s*([^;]+);/g)) {
        out[m[1]] = m[2].trim()
    }
    return out
}

function hexToRgb(hex: string): [number, number, number] {
    const h = hex.replace('#', '').trim()
    const full = h.length === 3 ? h.split('').map(c => c + c).join('') : h
    return [
        parseInt(full.slice(0, 2), 16),
        parseInt(full.slice(2, 4), 16),
        parseInt(full.slice(4, 6), 16),
    ]
}

function relativeLuminance(hex: string): number {
    const [r, g, b] = hexToRgb(hex).map(v => {
        const s = v / 255
        return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4)
    })
    return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/** WCAG 2.1 contrast ratio. */
function contrast(fg: string, bg: string): number {
    const a = relativeLuminance(fg)
    const b = relativeLuminance(bg)
    const [light, dark] = a > b ? [a, b] : [b, a]
    return (light + 0.05) / (dark + 0.05)
}

const light = blockColors(':root')
const dark = blockColors('.dark')

function token(theme: Record<string, string>, name: string): string {
    const v = theme[name]
    if (!v || !v.startsWith('#')) {
        throw new Error(`${name} is not a literal hex colour in global.css (got ${v})`)
    }
    return v
}

test('the palette was actually parsed', () => {
    // Guards against a silent parse failure making the suites below vacuous.
    expect(Object.keys(light).length).toBeGreaterThan(20)
    expect(Object.keys(dark).length).toBeGreaterThan(20)
})

describe.each([
    ['light', light],
    ['dark', dark],
])('%s theme legibility', (_name, theme) => {
    // Inline code: the pair that was broken in dark mode.
    test('inline code text is legible on its own background', () => {
        const ratio = contrast(token(theme, '--ink'), token(theme, '--surface-3'))
        expect(ratio).toBeGreaterThanOrEqual(4.5)
    })

    // Fenced code keeps a dark surface in both themes, so its foreground is the
    // light --code-ink rather than the theme's --ink.
    test('code block text is legible on the code surface', () => {
        const ratio = contrast(token(theme, '--code-ink'), token(theme, '--code-bg'))
        expect(ratio).toBeGreaterThanOrEqual(4.5)
    })

    test('body copy is legible on the canvas', () => {
        for (const ink of ['--ink', '--ink-soft', '--muted'] as const) {
            expect(contrast(token(theme, ink), token(theme, '--canvas'))).toBeGreaterThanOrEqual(4.5)
        }
    })
})

// Proves the guard is not vacuous: the actual pre-fix combination — the
// typography plugin's slate-900 foreground on the dark --surface-3 — fails it.
test('the old slate-on-dark-code combination fails the guard', () => {
    expect(contrast('#0f172a', token(dark, '--surface-3'))).toBeLessThan(4.5)
})

test('the inline code rule pairs a colour with its background', () => {
    // The original defect was a call site that set only `bg-surface-3`, leaving
    // the foreground to the typography plugin. The global rule sets both.
    const rule = css.slice(css.indexOf('.prose :where(code):not(:where(pre *'))
    const body = rule.slice(0, rule.indexOf('}'))
    expect(body).toContain('background-color: var(--surface-3)')
    expect(body).toContain('color: var(--ink)')
})

test('dark prose maps the typography palette to theme tokens', () => {
    // Without this, any `.prose` block that is not also `prose-invert` keeps the
    // light palette on a dark canvas.
    expect(css).toContain('.dark .prose {')
    expect(css).toMatch(/\.dark \.prose \{[^}]*--tw-prose-code:\s*var\(--ink\)/)
    expect(css).toMatch(/\.dark \.prose \{[^}]*--tw-prose-body:\s*var\(--ink-soft\)/)
})
