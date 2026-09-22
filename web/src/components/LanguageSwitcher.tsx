import { useTranslation } from 'react-i18next';
import { LOCALES } from '../i18n/locales';

export default function LanguageSwitcher() {
  const { i18n } = useTranslation();

  return (
    <select
      value={i18n.language}
      onChange={(e) => i18n.changeLanguage(e.target.value)}
      aria-label="Language"
      className="border rounded px-2 py-1 text-xs bg-white dark:bg-gray-800 cursor-pointer"
    >
      {/* Driven by the locale registry — previously this list was hardcoded
          here *and* in i18n/index.ts, with no check that they agreed. */}
      {LOCALES.map(({ code, label }) => (
        <option key={code} value={code}>
          {label}
        </option>
      ))}
    </select>
  );
}
