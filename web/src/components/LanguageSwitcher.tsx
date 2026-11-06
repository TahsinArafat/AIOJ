import { useTranslation } from 'react-i18next';
import { LOCALES } from '../i18n/locales';

export default function LanguageSwitcher() {
  const { i18n } = useTranslation();

  return (
    <select
      value={i18n.language}
      onChange={(e) => i18n.changeLanguage(e.target.value)}
      aria-label="Language"
      className="h-9 cursor-pointer rounded-lg border border-hairline-strong bg-surface px-2 text-xs text-ink-soft transition-colors hover:border-muted-soft focus:border-brand focus:outline-none"
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
