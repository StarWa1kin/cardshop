import { useAppStore } from '../stores/app'
import { readThemePreview, resolveStorefrontTheme, type StorefrontTheme } from '../../../shared/storefrontTheme'

// A preview belongs to this document only. Reload/exit explicitly resolves again.
export const themePreview = typeof window === 'undefined' ? null : readThemePreview(window.location.search)
let documentTheme: StorefrontTheme | null = null

export function initStorefrontTheme(): void {
  try { localStorage.removeItem('dj-storefront-template') } catch { /* storage may be disabled */ }
}

export function getActiveTheme(): StorefrontTheme {
  if (themePreview) return themePreview.theme
  if (documentTheme) return documentTheme
  const config = useAppStore().config
  if (!config) return (documentTheme = 'theme-1')
  documentTheme = resolveStorefrontTheme(config.storefront_theme, config.storefront_template)
  return documentTheme
}

type ViewLoader = () => Promise<unknown>
const vaultViews = import.meta.glob('./vault/**/*.vue')
// Themes 3+ override only the pages they restyle; everything else falls back to vault.
const ownViews: Partial<Record<StorefrontTheme, { dir: string, views: Record<string, ViewLoader> }>> = {
  'theme-3': { dir: 'tech', views: import.meta.glob('./tech/**/*.vue') },
  'theme-4': { dir: 'cyber', views: import.meta.glob('./cyber/**/*.vue') },
}

// Derived themes reuse the existing purchasing pages and their exact business handlers.
export const templateView = (name: string, classicLoader: ViewLoader): ViewLoader => () => {
  const theme = getActiveTheme()
  const own = ownViews[theme]
  const ownView = own?.views[`./${own.dir}/${name}.vue`]
  if (ownView) return ownView()
  if (theme !== 'theme-1') {
    const sharedView = vaultViews[`./vault/${name}.vue`]
    if (sharedView) return sharedView()
  }
  return classicLoader()
}
