/** Presentation-only policy shared by the two frontends. No business state. */
export const STOREFRONT_THEMES = ['theme-1', 'theme-2', 'theme-3'] as const
export type StorefrontTheme = typeof STOREFRONT_THEMES[number]
export type ColorMode = 'light' | 'dark'

export const isStorefrontTheme = (value: unknown): value is StorefrontTheme =>
  typeof value === 'string' && (STOREFRONT_THEMES as readonly string[]).includes(value)

export function resolveStorefrontTheme(value: unknown, legacy?: unknown): StorefrontTheme {
  return isStorefrontTheme(value) ? value : legacy === 'vault' ? 'theme-2' : 'theme-1'
}

export function readThemePreview(search: string) {
  const params = new URLSearchParams(search)
  const theme = params.get('theme_preview')
  const mode = params.get('theme_preview_mode')
  return isStorefrontTheme(theme)
    ? { theme, mode: mode === 'light' || mode === 'dark' ? mode as ColorMode : null }
    : null
}

export function exitThemePreviewUrl(href: string) {
  const url = new URL(href)
  url.searchParams.delete('theme_preview')
  url.searchParams.delete('theme_preview_mode')
  return url.href
}

export function themePreviewUrl(base: string, theme: StorefrontTheme, mode: ColorMode): string | null {
  try {
    const url = new URL(base)
    if (!['http:', 'https:'].includes(url.protocol)) return null
    url.searchParams.set('theme_preview', theme)
    url.searchParams.set('theme_preview_mode', mode)
    return url.href
  } catch { return null }
}
