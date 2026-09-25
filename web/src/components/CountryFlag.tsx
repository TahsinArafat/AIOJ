/**
 * Country flag from a country name (or ISO code).
 * Uses regional-indicator emoji — no image assets required.
 */
const NAME_TO_ISO: Record<string, string> = {
  Bangladesh: 'BD', India: 'IN', Pakistan: 'PK', Nepal: 'NP', 'Sri Lanka': 'LK',
  'United States': 'US', USA: 'US', 'United Kingdom': 'GB', UK: 'GB',
  Germany: 'DE', France: 'FR', Canada: 'CA', Australia: 'AU', Japan: 'JP',
  China: 'CN', Russia: 'RU', Brazil: 'BR', 'South Korea': 'KR', Korea: 'KR',
  Ukraine: 'UA', Poland: 'PL', Vietnam: 'VN', Thailand: 'TH', Indonesia: 'ID',
  Malaysia: 'MY', Singapore: 'SG', Philippines: 'PH', Egypt: 'EG', Turkey: 'TR',
  Spain: 'ES', Italy: 'IT', Netherlands: 'NL', Sweden: 'SE', Norway: 'NO',
  Denmark: 'DK', Finland: 'FI', Mexico: 'MX', Argentina: 'AR', Chile: 'CL',
  Nigeria: 'NG', Kenya: 'KE', 'South Africa': 'ZA', Israel: 'IL', 'Saudi Arabia': 'SA',
  'United Arab Emirates': 'AE', Iran: 'IR', Iraq: 'IQ', Morocco: 'MA',
}

function isoFrom(country: string): string | null {
  const c = (country || '').trim()
  if (!c) return null
  if (/^[A-Za-z]{2}$/.test(c)) return c.toUpperCase()
  if (NAME_TO_ISO[c]) return NAME_TO_ISO[c]
  // case-insensitive name match
  const lower = c.toLowerCase()
  for (const [name, iso] of Object.entries(NAME_TO_ISO)) {
    if (name.toLowerCase() === lower) return iso
  }
  return null
}

/** Flag emoji for a country name/code, or null when unknown. */
export function flagEmoji(country: string): string | null { // eslint-disable-line react-refresh/only-export-components -- pure helper stays co-located with its component
  const iso = isoFrom(country)
  if (!iso || iso.length !== 2) return null
  const A = 0x1f1e6
  const base = 'A'.charCodeAt(0)
  const c1 = iso.charCodeAt(0) - base + A
  const c2 = iso.charCodeAt(1) - base + A
  return String.fromCodePoint(c1, c2)
}

interface CountryFlagProps {
  country?: string | null
  /** Show the country name next to the flag. */
  withName?: boolean
  className?: string
}

export default function CountryFlag({ country, withName = false, className = '' }: CountryFlagProps) {
  if (!country?.trim()) return null
  const emoji = flagEmoji(country)
  return (
    <span className={`inline-flex items-center gap-1 ${className}`} title={country}>
      {emoji && <span aria-hidden="true">{emoji}</span>}
      {withName && <span>{country}</span>}
      {!emoji && !withName && <span className="text-xs text-gray-500">{country}</span>}
    </span>
  )
}
