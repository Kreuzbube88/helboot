import { describe, expect, it } from 'vitest'
import en from './locales/en/translation.json'
import de from './locales/de/translation.json'

/** Recursively collects dot-separated key paths of a translation tree. */
function keyPaths(obj: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === 'object') {
      return keyPaths(value as Record<string, unknown>, path)
    }
    return [path]
  })
}

describe('translation catalogs', () => {
  // Every visible string goes through i18n (§23); a key existing in only
  // one language would silently fall back and break the other locale.
  it('en and de declare exactly the same keys', () => {
    expect(keyPaths(de).sort()).toEqual(keyPaths(en).sort())
  })

  it('no translation value is empty', () => {
    for (const catalog of [en, de]) {
      for (const path of keyPaths(catalog)) {
        const value = path
          .split('.')
          .reduce<unknown>((node, key) => (node as Record<string, unknown>)[key], catalog)
        expect(value, `empty translation for ${path}`).not.toBe('')
      }
    }
  })
})
