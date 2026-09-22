import { describe, it, expect } from 'vitest';
import { LOCALES, DEFAULT_LOCALE, resources, supportedCodes, isSupported } from './index';
import en from './en.json';
import bn from './bn.json';

/**
 * Guards the multilingual system's extensibility contract.
 *
 * The catalogs were already in perfect 64/64 parity when this was written, but
 * nothing enforced it — a key added to en.json alone would silently render
 * English inside a Bengali session because `fallbackLng` makes it invisible.
 * These tests turn that silent drift into a failure.
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
});

describe('catalog parity', () => {
  const enKeys = Object.keys(flat(en as Tree)).sort();
  const bnKeys = Object.keys(flat(bn as Tree)).sort();

  it('en catalog is non-empty', () => {
    expect(enKeys.length).toBeGreaterThan(0);
  });

  it('every English key exists in Bengali', () => {
    expect(bnKeys).toEqual(enKeys);
  });

  it('no Bengali catalog is empty and no value is untranslated', () => {
    const bnValues = Object.values(flat(bn as Tree));
    expect(bnValues.length).toBe(enKeys.length);
    for (const [key, value] of Object.entries(flat(bn as Tree))) {
      expect(value.trim(), `bn key "${key}" is blank`).not.toBe('');
    }
  });
});
