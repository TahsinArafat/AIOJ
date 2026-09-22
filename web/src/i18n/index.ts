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
