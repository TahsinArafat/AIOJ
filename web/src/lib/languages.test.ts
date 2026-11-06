import { expect, test } from 'vitest'
import { LANGUAGES, LANGUAGE_OPTIONS, languageName, languageTemplate } from './languages'

/**
 * The judge language list is defined once, in `lang/*.yaml` at the repo root.
 * These tests keep the frontend copy honest: the two must agree on the set of
 * keys, so adding a language on the backend without surfacing it in the UI (or
 * advertising a key the judge does not have) fails here.
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

function backendLanguageKeys(): string[] {
  const keys = Object.entries(yamlSources).map(([path, body]) => {
    const match = body.match(/^key:\s*"([^"]+)"/m)
    // Fall back to the file name, which is the convention for every entry.
    return match ? match[1] : path.replace(/^.*\//, '').replace(/\.yaml$/, '')
  })
  return keys.sort()
}

test('the backend language configs were actually found', () => {
  // Guards against the glob silently matching nothing and passing vacuously.
  expect(Object.keys(yamlSources).length, 'expected to find lang/*.yaml').toBeGreaterThan(5)
})

test('frontend language keys match lang/*.yaml exactly', () => {
  expect(LANGUAGES.map(l => l.key).sort()).toEqual(backendLanguageKeys())
})

test('every language has a display name and a starter template', () => {
  for (const lang of LANGUAGES) {
    expect(lang.name, `${lang.key} needs a name`).toBeTruthy()
    expect(languageName(lang.key)).toBe(lang.name)
    expect(typeof languageTemplate(lang.key), `${lang.key} needs a template`).toBe('string')
    expect(languageTemplate(lang.key).length, `${lang.key} template is empty`).toBeGreaterThan(0)
  }
})

test('unknown keys degrade to the raw key instead of blanking the UI', () => {
  expect(languageName('some-new-language')).toBe('some-new-language')
  expect(languageName(undefined)).toBe('—')
  expect(languageTemplate('some-new-language')).toBe('')
})

test('LANGUAGE_OPTIONS is derived from the registry, not a second copy', () => {
  expect(LANGUAGE_OPTIONS).toHaveLength(LANGUAGES.length)
  expect(LANGUAGE_OPTIONS[0]).toEqual({ value: LANGUAGES[0].key, label: LANGUAGES[0].name })
})

test('every judged language is selectable, including the ones contests used to hide', () => {
  const keys = LANGUAGE_OPTIONS.map(o => o.value)
  // ContestProblem carried its own shorter list, which silently hid these.
  for (const required of ['cpp-clang', 'pypy', 'javascript', 'c-gcc-32']) {
    expect(keys, `${required} must be selectable`).toContain(required)
  }
})
