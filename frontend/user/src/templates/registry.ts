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

const vaultViews = import.meta.glob('./vault/**/*.vue')
const techViews = import.meta.glob('./tech/**/*.vue')
type ViewLoader = () => Promise<unknown>

// Tech reuses the existing purchasing pages and their exact business handlers.
export const templateView = (name: string, classicLoader: ViewLoader): ViewLoader => () => {
  const theme = getActiveTheme()
  if (theme === 'theme-3') {
    const ownView = techViews[`./tech/${name}.vue`]
    if (ownView) return ownView()
  }
  if (theme !== 'theme-1') {
    const sharedView = vaultViews[`./vault/${name}.vue`]
    if (sharedView) return sharedView()
  }
  return classicLoader()
}
