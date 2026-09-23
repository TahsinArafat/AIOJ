import { describe, it, expect } from 'vitest';
import { LOCALES, DEFAULT_LOCALE, resources, supportedCodes, isSupported, isRtl } from './index';
import en from './en.json';

/**
 * Guards the multilingual system's extensibility contract.
 *
 * Catalogs must stay in key-parity with English (including Phase C zh/ru/ja/es)
 * or a key added to en alone renders English inside another locale because
 * `fallbackLng` makes the missing key invisible.
 */

type Tree = { [k: string]: string | Tree };

/** Flatten to dotted leaf paths so nested keys compare exactly. */
function flat(obj: Tree, prefix = ''): Record<string, string> {
  return Object.entries(obj).reduce<Record<string, string>>((acc, [k, v]) => {
    const path = prefix ? `${prefix}.${k}` : k;
    if (typeof v === 'string') acc[path] = v;
    else Object.assign(acc, flat(v as Tree, path));
    return acc;
  }, {});
}

const enKeys = Object.keys(flat(en as Tree)).sort();

describe('locale registry', () => {
  it('exposes the default locale', () => {
    expect(isSupported(DEFAULT_LOCALE)).toBe(true);
  });

  it('builds i18next resources for every registered locale', () => {
    expect(Object.keys(resources).sort()).toEqual([...supportedCodes].sort());
    for (const code of supportedCodes) {
      expect(resources[code].translation).toBeDefined();
    }
  });

  it('has a unique code per locale', () => {
    const codes = LOCALES.map((l) => l.code);
    expect(new Set(codes).size).toBe(codes.length);
  });

  it('labels every locale non-trivially', () => {
    for (const { code, label } of LOCALES) {
      expect(label.trim(), `locale ${code} needs a label`).not.toBe('');
    }
  });

  it('registers Phase C expansion locales', () => {
    for (const code of ['zh', 'ru', 'ja', 'es']) {
      expect(isSupported(code), `missing locale ${code}`).toBe(true);
    }
  });

  it('marks only RTL locales as rtl', () => {
    expect(isRtl('en')).toBe(false);
    expect(isRtl('bn')).toBe(false);
    // When ar/he are added with rtl: true, isRtl must return true.
  });
});

describe('catalog parity (all registered locales)', () => {
  it('en catalog is non-empty', () => {
    expect(enKeys.length).toBeGreaterThan(0);
  });

  for (const { code, catalog } of LOCALES) {
    it(`${code} has exact key parity with en and no blank values`, () => {
      const keys = Object.keys(flat(catalog as unknown as Tree)).sort();
      expect(keys).toEqual(enKeys);
      const values = flat(catalog as unknown as Tree);
      for (const [key, value] of Object.entries(values)) {
        expect(value.trim(), `${code} key "${key}" is blank`).not.toBe('');
      }
    });
  }
});
