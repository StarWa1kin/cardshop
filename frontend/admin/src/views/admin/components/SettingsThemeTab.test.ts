import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, nextTick, type App } from 'vue'
import SettingsThemeTab from './SettingsThemeTab.vue'

const api = vi.hoisted(() => ({ getSettings: vi.fn(), updateSettings: vi.fn() }))
const notify = vi.hoisted(() => ({ notifyError: vi.fn(), notifySuccess: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: api }))
vi.mock('@/utils/notify', () => notify)
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
let app: App
let host: HTMLDivElement
const flush = async () => { for (let i = 0; i < 5; i++) { await Promise.resolve(); await nextTick() } }
const button = (suffix: string) => Array.from(host.querySelectorAll('button')).find(el => el.textContent?.includes(`theme.${suffix}`))!
const select = async (value: string) => {
  const radio = host.querySelector<HTMLInputElement>(`input[value="${value}"]`)!
  radio.checked = true
  radio.dispatchEvent(new Event('change', { bubbles: true }))
  await nextTick()
}
beforeEach(() => {
  vi.clearAllMocks()
  api.getSettings.mockImplementation(({ key }) => Promise.resolve({ data: { data: key === 'site_config' ? { storefront_template: 'vault' } : {} } }))
  api.updateSettings.mockImplementation((request) => Promise.resolve({ data: { data: request.value } }))
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(SettingsThemeTab, { siteUrl: 'https://shop.test/' })
  app.mount(host)
})
afterEach(() => { app.unmount(); host.remove() })
describe('storefront theme independent controls', () => {
  it('maps legacy vault and does not save merely by selecting or previewing', async () => {
    await flush()
    expect(host.querySelector<HTMLInputElement>('input[value="theme-2"]')?.checked).toBe(true)
    await select('theme-3')
    expect(api.updateSettings).not.toHaveBeenCalled()
    const previews = Array.from(host.querySelectorAll('a'))
    expect(previews.map(a => new URL(a.href).searchParams.get('theme_preview'))).toEqual(['theme-1', 'theme-2', 'theme-3'])
  })
  it('applies only the dedicated setting and confirms success', async () => {
    await flush(); await select('theme-3'); button('apply').click(); await flush()
    expect(api.updateSettings).toHaveBeenCalledExactlyOnceWith({ key: 'storefront_theme_config', value: { theme_id: 'theme-3' } })
    expect(notify.notifySuccess).toHaveBeenCalledOnce()
    expect(button('apply').disabled).toBe(true)
  })
  it('keeps the previous active theme and pending selection on save failure', async () => {
    await flush(); api.updateSettings.mockRejectedValueOnce(new Error('offline'))
    await select('theme-3'); button('apply').click(); await flush()
    expect(notify.notifySuccess).not.toHaveBeenCalled()
    expect(notify.notifyError).toHaveBeenCalledOnce()
    expect(host.querySelector<HTMLInputElement>('input[value="theme-3"]')?.checked).toBe(true)
    expect(button('apply').disabled).toBe(false)
    expect(host.textContent).toContain('theme.pending')
  })
  it('failed configuration reads disable saving and offer retry', async () => {
    await flush()
    app.unmount()
    api.getSettings.mockRejectedValue(new Error('offline'))
    app = createApp(SettingsThemeTab, { siteUrl: 'https://shop.test/' })
    app.mount(host)
    await flush()
    expect(host.querySelector('[role="alert"]')).not.toBeNull()
    expect(host.querySelector('input[type="radio"]')).toBeNull()
    expect(api.updateSettings).not.toHaveBeenCalled()
  })
  it('preview color is local and never saves configuration', async () => {
    await flush(); button('dark').click(); await nextTick()
    expect(Array.from(host.querySelectorAll('a')).every(a => new URL(a.href).searchParams.get('theme_preview_mode') === 'dark')).toBe(true)
    expect(api.updateSettings).not.toHaveBeenCalled()
  })
})
