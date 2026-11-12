import { expect, test } from 'vitest'
import {
  FALLBACK_LANGUAGES,
  LANGUAGE_OPTIONS,
  languageName,
  languageOptions,
  languageTemplate,
  toLanguages,
} from './languages'

/**
 * `lang/*.yaml` on the backend is the single source of truth, served live at
 * GET /api/languages. These tests guard the two ways the frontend has drifted
 * from it before:
 *
 *  1. It used to be a hardcoded array in five places. The copies disagreed with
 *     each other and with the judge — stale compilers, a contest picker
 *     missing whole languages.
 *  2. It was then one array in the bundle, which still ignored the panel: an
 *     admin edit or a new language never reached the dropdowns.
 *
 * The fallback list is the only local copy left, and it exists solely so a
 * dropdown is never empty when the request fails. It must therefore match the
 * shipped files exactly — a test pins that.
 *
 * The YAML files are pulled through `import.meta.glob` rather than `node:fs`
 * so this file typechecks under the browser tsconfig, which has no
 * `@types/node` — the same approach as `i18n/i18n-keys.test.ts`.
 */
const yamlSources = import.meta.glob('../../../lang/*.yaml', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

function backendLanguages(): { key: string; name: string; extensions: string[] }[] {
  return Object.entries(yamlSources).map(([path, body]) => {
    const key = body.match(/^key:\s*"([^"]+)"/m)?.[1]
      // Fall back to the file name, which is the convention for every entry.
      ?? path.replace(/^.*\//, '').replace(/\.yaml$/, '')
    const name = body.match(/^name:\s*"([^"]+)"/m)?.[1]
      ?? body.match(/^name:\s*([^"\n]+)$/m)?.[1]?.trim()
      ?? key
    const extLine = body.match(/^extensions:\s*\[(.*)\]$/m)?.[1] ?? ''
    const extensions = extLine.split(',').map(e => e.trim().replace(/^["']|["']$/g, '')).filter(Boolean)
    return { key, name, extensions }
  })
}

function backendLanguageKeys(): string[] {
  return backendLanguages().map(l => l.key).sort()
}

test('the backend language configs were actually found', () => {
  // Guards against the glob silently matching nothing and passing vacuously.
  expect(Object.keys(yamlSources).length, 'expected to find lang/*.yaml').toBeGreaterThan(5)
})

test('the offline fallback matches lang/*.yaml exactly', () => {
  expect(FALLBACK_LANGUAGES.map(l => l.key).sort()).toEqual(backendLanguageKeys())
})

test('the offline fallback uses the display names from the YAML files', () => {
  const byKey = new Map(backendLanguages().map(l => [l.key, l]))
  for (const lang of FALLBACK_LANGUAGES) {
    const backend = byKey.get(lang.key)
    if (!backend) continue // the key-set test above already reports this
    expect(lang.name, `${lang.key} name has drifted from lang/${lang.key}.yaml`).toBe(backend.name)
  }
})

test('every fallback language has a display name and a starter template', () => {
  for (const lang of FALLBACK_LANGUAGES) {
    expect(lang.name, `${lang.key} needs a name`).toBeTruthy()
    expect(languageName(lang.key)).toBe(lang.name)
    expect(languageTemplate(lang.key).length, `${lang.key} template is empty`).toBeGreaterThan(0)
  }
})

test('unknown keys degrade to the raw key instead of blanking the UI', () => {
  expect(languageName('some-new-language')).toBe('some-new-language')
  expect(languageName(undefined)).toBe('—')
  expect(languageTemplate('some-new-language')).toBe('')
})

test('LANGUAGE_OPTIONS is derived from the fallback registry, not a second copy', () => {
  expect(LANGUAGE_OPTIONS).toHaveLength(FALLBACK_LANGUAGES.length)
  expect(LANGUAGE_OPTIONS[0]).toEqual({ value: FALLBACK_LANGUAGES[0].key, label: FALLBACK_LANGUAGES[0].name })
})

test('every judged language is selectable, including the ones contests used to hide', () => {
  const keys = LANGUAGE_OPTIONS.map(o => o.value)
  // ContestProblem carried its own shorter list, which silently hid these.
  for (const required of ['cpp-clang', 'pypy', 'javascript', 'c-gcc-32']) {
    expect(keys, `${required} must be selectable`).toContain(required)
  }
})

// --- Server rows -> UI languages -------------------------------------------

test('server rows win over the fallback, so an admin edit reaches the dropdown', () => {
  const langs = toLanguages([
    { key: 'cpp-gpp-64', name: 'C++ 20 (GCC 13)', extensions: ['.cpp'] },
    { key: 'custom-lang', name: 'My Language', extensions: ['.ml'] },
  ])

  expect(langs).toHaveLength(2)
  expect(langs[0].name).toBe('C++ 20 (GCC 13)')
  // A key the bundle has never heard of still renders, with a name from the
  // server — this is exactly the "add a language in the panel" case.
  expect(langs[1]).toEqual(expect.objectContaining({ key: 'custom-lang', name: 'My Language' }))
  expect(languageOptions(langs).map(o => o.value)).toEqual(['cpp-gpp-64', 'custom-lang'])
})

test('a failed or empty response falls back instead of rendering an empty dropdown', () => {
  expect(toLanguages(undefined).map(l => l.key).sort()).toEqual(backendLanguageKeys())
  expect(toLanguages(null).map(l => l.key).sort()).toEqual(backendLanguageKeys())
  expect(toLanguages([]).map(l => l.key).sort()).toEqual(backendLanguageKeys())
})

test('an unknown language key still gets starter code from its extension', () => {
  // An admin can invent a key; the editor still needs something to open with.
  const [lang] = toLanguages([{ key: 'totally-new', name: 'Totally New', extensions: ['.cpp'] }])
  expect(lang.template).toContain('#include <iostream>')
})

test('a name omitted by the server degrades to the key rather than an empty label', () => {
  const [lang] = toLanguages([{ key: 'mystery', name: '', extensions: [] }])
  expect(lang.name).toBe('mystery')
})

test('languageName accepts an explicit list, so pages that fetched can resolve renamed keys', () => {
  const langs = toLanguages([{ key: 'python', name: 'Python 3.12', extensions: ['.py'] }])
  expect(languageName('python', langs)).toBe('Python 3.12')
})
