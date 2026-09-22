import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { DEFAULT_LOCALE, resources, supportedCodes } from './locales';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    // Sourced from the locale registry so this and LanguageSwitcher can never
    // disagree about which languages exist.
    resources,
    supportedLngs: supportedCodes,
    fallbackLng: DEFAULT_LOCALE,
    interpolation: { escapeValue: false },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
    },
  });

export default i18n;

/**
 * Mirror the active language onto <html lang>.
 *
 * Previously the attribute was hardcoded to "en" in index.html and never
 * updated, so a Bengali session still advertised lang="en" to screen readers
 * and crawlers. Runs once for the detected language and on every change.
 */
function syncDocumentLang(code: string) {
  if (typeof document !== 'undefined') document.documentElement.lang = code;
}

syncDocumentLang(i18n.language);
i18n.on('languageChanged', syncDocumentLang);
