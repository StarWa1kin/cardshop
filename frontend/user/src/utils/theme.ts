import { ref, watch } from 'vue'
import { readThemePreview, type ColorMode } from '../../../shared/storefrontTheme'

const TEA_THEME_KEY = 'dujiao_theme'
const preview = readThemePreview(window.location.search)
const getSystemTheme = (): ColorMode => window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
const getSavedTheme = (): ColorMode | null => {
    try {
        const value = localStorage.getItem(TEA_THEME_KEY)
        return value === 'dark' || value === 'light' ? value : null
    } catch { return null }
}
const theme = ref<ColorMode>(preview?.mode || getSavedTheme() || getSystemTheme())
watch(theme, (value) => {
    document.documentElement.classList.toggle('dark', value === 'dark')
    document.documentElement.style.colorScheme = value
    if (!preview) {
        try { localStorage.setItem(TEA_THEME_KEY, value) } catch { /* optional persistence */ }
    }
}, { immediate: true })

export const useTheme = () => ({
    theme,
    toggleTheme: () => { theme.value = theme.value === 'dark' ? 'light' : 'dark' },
})
