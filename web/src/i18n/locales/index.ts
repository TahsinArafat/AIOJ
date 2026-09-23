/**
 * Single source of truth for the languages this app ships.
 *
 * Before this file the language list existed twice — once as the `resources`
 * map in `web/src/i18n/index.ts` and once as the `languages` array inside
 * `LanguageSwitcher.tsx`. Adding a language meant editing both, and nothing
 * caught drift between them: a code present in the switcher but absent from
 * `resources` would render without any translation, silently.
 *
 * Both consumers now derive from this registry, so adding a language is one
 * entry plus one catalog file.
 *
 * Phase C: zh / ru / ja / es added as full catalogs (parity with en via
 * locales.test.ts). RTL readiness: `rtl` flag is optional and drives
 * `<html dir>` from i18n/index.ts.
 */

import en from './en.json';
import bn from './bn.json';
import zh from './zh.json';
import ru from './ru.json';
import ja from './ja.json';
import es from './es.json';

export interface LocaleDefinition {
  /** BCP-47 code, also the i18next resource key. */
  code: string;
  /** Name shown in the switcher — always in the language's own script. */
  label: string;
  /** Translation catalog. Typed as the English catalog so a missing key is a build error. */
  catalog: typeof en;
  /** True for right-to-left scripts (ar, he, fa, …). Absent = LTR. */
  rtl?: boolean;
}

/** English is the fallback: `fallbackLng` in the i18n init points at this code. */
export const DEFAULT_LOCALE = 'en';

export const LOCALES: readonly LocaleDefinition[] = [
  { code: 'en', label: 'English', catalog: en },
  { code: 'bn', label: 'বাংলা', catalog: bn },
  { code: 'zh', label: '中文', catalog: zh },
  { code: 'ru', label: 'Русский', catalog: ru },
  { code: 'ja', label: '日本語', catalog: ja },
  { code: 'es', label: 'Español', catalog: es },
] as const;

/** `{ en: { translation }, … }` — the shape i18next `resources` expects. */
export const resources = Object.fromEntries(
  LOCALES.map(({ code, catalog }) => [code, { translation: catalog }]),
) as Record<string, { translation: typeof en }>;

/** The codes i18next is allowed to resolve, e.g. for `supportedLngs`. */
export const supportedCodes = LOCALES.map(({ code }) => code);

export function isSupported(code: string): boolean {
  return supportedCodes.includes(code);
}

/** Whether a locale renders right-to-left (used to set `<html dir>`). */
export function isRtl(code: string): boolean {
  return LOCALES.find(l => l.code === code)?.rtl === true;
}
