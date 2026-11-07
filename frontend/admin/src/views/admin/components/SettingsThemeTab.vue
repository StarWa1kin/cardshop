<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, ExternalLink, Moon, Sun, LoaderCircle } from 'lucide-vue-next'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { notifyError, notifySuccess } from '@/utils/notify'
import { STOREFRONT_THEMES, isStorefrontTheme, resolveStorefrontTheme, themePreviewUrl, type StorefrontTheme, type ColorMode } from '../../../../../shared/storefrontTheme'
const props = defineProps<{ siteUrl?: string }>()
const { t } = useI18n()
const selected = ref<StorefrontTheme>('theme-1')
const current = ref<StorefrontTheme>('theme-1')
const mode = ref<ColorMode>('light')
const loading = ref(true)
const loadError = ref(false)
const submitting = ref(false)
const dirty = computed(() => current.value !== selected.value)
const load = async () => {
  loading.value = true
  loadError.value = false
  try {
    const [theme, site] = await Promise.all([
      adminAPI.getSettings({ key: 'storefront_theme_config' }),
      adminAPI.getSettings({ key: 'site_config' }),
    ])
    current.value = resolveStorefrontTheme(theme.data.data?.theme_id, site.data.data?.storefront_template)
    selected.value = current.value
  } catch {
    loadError.value = true
  } finally { loading.value = false }
}
const previewUrl = (theme: StorefrontTheme) => {
  const fallback = new URL('/', window.location.origin)
  if (import.meta.env.DEV && fallback.port === '5174') fallback.port = '5173'
  return themePreviewUrl(props.siteUrl?.trim() || fallback.href, theme, mode.value)
}
const apply = async () => {
  if (!dirty.value || submitting.value || loading.value || loadError.value) return
  submitting.value = true
  const submitted = selected.value
  try {
    const response = await adminAPI.updateSettings({ key: 'storefront_theme_config', value: { theme_id: submitted } })
    const saved = response.data.data?.theme_id
    if (!isStorefrontTheme(saved)) throw new Error('Invalid saved theme')
    current.value = saved
    notifySuccess(t('admin.settings.theme.applied'))
  } catch { notifyError(t('admin.settings.alerts.saveFailed')) }
  finally { submitting.value = false }
}
onMounted(load)
</script>
<template>
  <section class="rounded-xl border border-border bg-card" :aria-busy="loading || submitting">
    <div class="flex flex-wrap items-center justify-between gap-4 border-b px-6 py-5">
      <div><h2 class="text-lg font-semibold">{{ t('admin.settings.theme.title') }}</h2><p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.theme.subtitle') }}</p></div>
      <div class="flex gap-1 rounded-lg border p-1" :aria-label="t('admin.settings.theme.previewMode')">
        <Button v-for="color in (['light', 'dark'] as const)" :key="color" size="sm" :variant="mode === color ? 'secondary' : 'ghost'" :aria-pressed="mode === color" @click="mode = color">
          <component :is="color === 'dark' ? Moon : Sun" :size="14" />{{ t(`admin.settings.theme.${color}`) }}
        </Button>
      </div>
    </div>
    <div v-if="loading" class="flex items-center gap-2 p-8 text-sm text-muted-foreground"><LoaderCircle class="h-4 w-4 animate-spin" />{{ t('admin.settings.theme.loading') }}</div>
    <div v-else-if="loadError" class="flex items-center gap-4 p-6" role="alert"><p>{{ t('admin.settings.theme.loadError') }}</p><Button variant="outline" @click="load">{{ t('admin.settings.theme.retry') }}</Button></div>
    <template v-else>
      <fieldset class="grid gap-4 p-6 sm:grid-cols-2 2xl:grid-cols-4" :disabled="submitting">
        <legend class="sr-only">{{ t('admin.settings.theme.title') }}</legend>
        <div v-for="(id, index) in STOREFRONT_THEMES" :key="id" class="theme-choice" :class="{ selected: selected === id }">
          <label class="theme-choice-label">
            <input v-model="selected" type="radio" name="storefront-theme" :value="id" class="sr-only" />
            <div class="theme-miniature" :class="[id, mode]" aria-hidden="true">
              <div class="mini-nav"><b /><span /><span /><i /></div>
              <div class="mini-hero"><div><small>DIGITAL / STORE</small><strong>{{ t(`admin.settings.theme.name${index + 1}`) }}</strong><em /><span class="mini-cta" /></div><div class="mini-art"><i /><i /><i /></div></div>
              <div class="mini-tabs"><i /><i /><i /><i /></div>
              <div class="mini-products"><div v-for="n in 3" :key="n"><i /><span /><small /></div></div>
            </div>
            <div class="flex items-center justify-between gap-2"><h3 class="font-semibold">{{ t(`admin.settings.theme.name${index + 1}`) }}</h3><Check v-if="selected === id" class="h-4 w-4 text-primary" /></div>
            <p class="min-h-10 text-xs leading-relaxed text-muted-foreground">{{ t(`admin.settings.theme.description${index + 1}`) }}</p>
          </label>
          <div class="flex flex-wrap items-center justify-between gap-2 px-4 pb-4 text-xs">
            <span :class="current === id ? 'text-primary' : 'text-muted-foreground'">{{ current === id ? t('admin.settings.theme.active') : selected === id ? t('admin.settings.theme.pending') : t('admin.settings.theme.supportsModes') }}</span>
            <a v-if="previewUrl(id)" :href="previewUrl(id)!" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 rounded px-2 py-1 text-primary hover:bg-primary/10">{{ t('admin.settings.theme.preview') }}<ExternalLink :size="12" /></a>
          </div>
        </div>
      </fieldset>
      <div class="flex flex-wrap items-center justify-between gap-4 border-t px-6 py-4">
        <p class="text-xs text-muted-foreground">{{ t('admin.settings.theme.saveHint') }}</p>
        <Button :disabled="!dirty || submitting" @click="apply"><LoaderCircle v-if="submitting" class="h-4 w-4 animate-spin" />{{ submitting ? t('admin.settings.actions.saving') : t('admin.settings.theme.apply') }}</Button>
      </div>
    </template>
  </section>
</template>
<style scoped>
.theme-choice{min-width:0;border:1px solid var(--border);border-radius:12px;transition:border-color 180ms,box-shadow 180ms;overflow:hidden}
.theme-choice.selected{border-color:var(--primary);box-shadow:0 0 0 1px var(--primary)}
.theme-choice:focus-within{outline:2px solid var(--primary);outline-offset:3px}
.theme-choice-label{display:flex;flex-direction:column;gap:10px;padding:12px 16px;cursor:pointer}
.theme-miniature{--m-bg:#f4f7fa;--m-card:#fff;--m-ink:#172632;--m-line:#dce5eb;--m-accent:#0071e3;background:var(--m-bg);color:var(--m-ink);border:1px solid var(--m-line);border-radius:7px;overflow:hidden;margin:0 -4px 4px;aspect-ratio:1.55;display:flex;flex-direction:column}
.theme-miniature.dark{--m-bg:#121820;--m-card:#202d37;--m-ink:#e3edf5;--m-line:#354450}
.theme-2{--m-accent:#6366f1}.theme-3{--m-accent:#12cbd8}.theme-4{--m-accent:#ff2a6d}
.theme-4.dark{--m-bg:#0b0614;--m-card:#150d26;--m-ink:#f3eeff;--m-line:#2d2047}
.theme-4 .mini-nav{border-bottom-color:var(--m-accent)}.theme-4 .mini-hero small{color:#05d9e8}.theme-4 .mini-hero .mini-cta{border-radius:0;clip-path:polygon(0 0,calc(100% - 4px) 0,100% 4px,100% 100%,4px 100%,0 calc(100% - 4px))}
.theme-4 .mini-art{background:repeating-linear-gradient(0deg,color-mix(in srgb,var(--m-accent) 18%,transparent) 0 1px,transparent 1px 5px)}.theme-4 .mini-art i{border-radius:0;transform:skewX(-14deg);border-color:#05d9e8}
.theme-4 .mini-products>div{border-radius:0;border-top:2px solid var(--m-accent)}.theme-4 .mini-tabs i{border-radius:0}
.mini-nav{display:flex;align-items:center;gap:8px;height:25px;padding:7px 10px;border-bottom:1px solid var(--m-line)}
.mini-nav b{width:9px;height:9px;background:var(--m-accent);border-radius:2px}.mini-nav span{height:2px;width:20px;background:var(--m-line)}.mini-nav i{margin-left:auto;width:7px;height:7px;border:1px solid var(--m-line);border-radius:50%}
.mini-hero{display:flex;padding:14px 12px 8px;gap:8px;flex:1;min-height:0}.mini-hero>div:first-child{display:flex;flex-direction:column;gap:6px;flex:1}.mini-hero small{font-size:5px;letter-spacing:.08em;color:var(--m-accent)}.mini-hero strong{font-size:13px}.mini-hero em{height:3px;width:70%;background:var(--m-line)}.mini-hero .mini-cta{height:12px;width:37px;background:var(--m-accent);border-radius:3px;margin-top:2px}
.mini-art{position:relative;flex:1;min-height:45px}.mini-art i{position:absolute;width:40px;height:49px;border:1px solid var(--m-accent);border-radius:5px;left:12%;top:0;background:color-mix(in srgb,var(--m-accent) 15%,var(--m-card));transform:rotate(15deg) skewY(-10deg);z-index:3}.mini-art i:nth-child(2){left:29%;top:5px;opacity:.5;z-index:2}.mini-art i:nth-child(3){left:45%;top:10px;opacity:.25;z-index:1}
.theme-1 .mini-art i{transform:rotate(-10deg);border-radius:12px}.theme-2 .mini-hero{margin:6px;border-radius:6px;background:color-mix(in srgb,var(--m-accent) 15%,var(--m-card))}
.mini-tabs{display:flex;gap:5px;padding:6px 12px}.mini-tabs i{width:25px;height:7px;border-radius:2px;background:var(--m-line)}.mini-tabs i:first-child{background:var(--m-accent)}
.mini-products{display:flex;gap:6px;padding:0 12px 12px}.mini-products>div{flex:1;display:flex;flex-direction:column;gap:4px;padding:5px;background:var(--m-card);border:1px solid var(--m-line);border-radius:4px}.mini-products i{height:17px;background:color-mix(in srgb,var(--m-accent) 10%,var(--m-bg));border-radius:3px}.mini-products span{height:3px;width:70%;background:var(--m-line)}.mini-products small{height:3px;width:40%;background:var(--m-accent)}
</style>
