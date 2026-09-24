import { expect, test } from 'vitest'
import i18n from './index'

/**
 * Regression net for raw-key rendering.
 *
 * App.tsx once called t('common.login') while the key actually lived at
 * nav.login, so the welcome sidebar rendered the literal string
 * "common.login" to every signed-out visitor. Every literal t('…') /
 * i18n.t('…') call in web/src must resolve against the catalog; dynamic
 * keys (template literals, variables) can't be checked statically and are
 * skipped.
 *
 * Sources are pulled through import.meta.glob rather than node:fs so this
 * file typechecks under the browser tsconfig (no @types/node).
 */

const sources = import.meta.glob('../**/*.{ts,tsx}', {
    query: '?raw',
    import: 'default',
    eager: true,
}) as Record<string, string>

const CALL = /(?<![\w$])t\(\s*(['"])([^'"]+)\1/g

test('every literal translation key used in web/src exists in the catalog', () => {
    const missing: string[] = []
    let scanned = 0
    for (const [path, code] of Object.entries(sources)) {
        if (path.includes('/i18n/') || /\.test\./.test(path)) continue
        for (const match of code.matchAll(CALL)) {
            scanned++
            const key = match[2]
            if (!i18n.exists(key)) missing.push(`${path}: ${key}`)
        }
    }
    // Guard against the glob silently matching nothing and passing vacuously.
    expect(scanned, 'expected to find literal t() call sites in web/src').toBeGreaterThan(20)
    expect(missing, `Missing translation keys:\n${missing.join('\n')}`).toEqual([])
})
