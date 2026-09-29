import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { resolveStorefrontTheme, readThemePreview, exitThemePreviewUrl, themePreviewUrl, STOREFRONT_THEMES } from '../../shared/storefrontTheme.ts'

test('theme resolves new IDs before legacy template, with safe fallback', () => {
  assert.equal(resolveStorefrontTheme(undefined, 'vault'), 'theme-2')
  assert.equal(resolveStorefrontTheme(undefined, 'classic'), 'theme-1')
  assert.equal(resolveStorefrontTheme('broken', 'vault'), 'theme-2')
  assert.equal(resolveStorefrontTheme({}, null), 'theme-1')
  for (const id of STOREFRONT_THEMES) assert.equal(resolveStorefrontTheme(id, 'vault'), id)
})
test('only explicit valid URL previews are honored', () => {
  assert.equal(readThemePreview('?template=vault'), null)
  assert.equal(readThemePreview('?theme_preview=unknown&theme_preview_mode=dark'), null)
  assert.deepEqual(readThemePreview('?theme_preview=theme-3&theme_preview_mode=dark'), { theme: 'theme-3', mode: 'dark' })
  assert.deepEqual(readThemePreview('?theme_preview=theme-2&theme_preview_mode=unknown'), { theme: 'theme-2', mode: null })
})
test('preview exit preserves all business query and hash values', () => {
  const result = new URL(exitThemePreviewUrl('https://shop.test/products/item?sku_id=42&redirect=%2Fme&theme_preview=theme-3&theme_preview_mode=dark#options'))
  assert.equal(result.searchParams.get('sku_id'), '42')
  assert.equal(result.searchParams.get('redirect'), '/me')
  assert.equal(result.hash, '#options')
  assert.equal(result.searchParams.has('theme_preview'), false)
  assert.equal(result.searchParams.has('theme_preview_mode'), false)
})
test('preview links accept only web origins and retain existing URL data', () => {
  assert.equal(themePreviewUrl('javascript:alert(1)', 'theme-3', 'dark'), null)
  assert.equal(themePreviewUrl('not a URL', 'theme-3', 'dark'), null)
  const url = new URL(themePreviewUrl('https://shop.test/?ref=keep', 'theme-3', 'light')!)
  assert.equal(url.searchParams.get('ref'), 'keep')
  assert.equal(url.searchParams.get('theme_preview'), 'theme-3')
})

test('shared storefront pages keep business scripts unchanged from pre-theme baseline', () => {
  // Presentation hooks may change; the original shared business scripts stay identical.
  const hashes: Record<string, string> = {
    "Home.vue": "f46eac783f1c2bbf194f92dfa189d9245bab78c3c6c771086459715f47410b66",
    "Products.vue": "d67f3179f66d954c42330ee341c77d83999778c27212cd591e233a4641b8dcb3",
    "components/VaultBannerHero.vue": "3ca4cb560584d2eecd4cb68a466ebf5086fefaea6d0b56d90d06a538239fd952",
    "components/VaultProductCard.vue": "1eca96f74f640927a31ddf4a949755135b87160e658f3864db9dc42f929859c9",
    "components/VaultProductListItem.vue": "0c7aeaeb0213b86ff9d78eff8544c6e1ee6e900edafcd563f0dc4b1cbe151479"
}
  for (const [file, hash] of Object.entries(hashes)) {
    const source = readFileSync(new URL(`../src/templates/vault/${file}`, import.meta.url), 'utf8')
    const script = source.match(/<script[\s\S]*?<\/script>/)?.[0] || ''
    assert.equal(createHash('sha256').update(script).digest('hex'), hash, `${file}: business script changed`)
  }
})

test('quick-buy change is limited to an accessibility directive import', () => {
  const source = readFileSync(new URL('../src/components/ProductQuickBuy.vue', import.meta.url), 'utf8')
  const script = (source.match(/<script[\s\S]*?<\/script>/)?.[0] || '').replace("import { vModalFocus } from '../directives/modalFocus'\n", '')
  assert.equal(createHash('sha256').update(script).digest('hex'), '8655d14a9eb0499c31eba658b7b1098cfd276457655d291f5ec64358c2c9b6d9')
})
