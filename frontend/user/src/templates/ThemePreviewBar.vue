<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Eye, X } from 'lucide-vue-next'
import { themePreview } from './registry'
import { exitThemePreviewUrl } from '../../../shared/storefrontTheme'
const { t } = useI18n()
const exit = () => window.location.assign(exitThemePreviewUrl(window.location.href))
</script>
<template>
  <aside class="theme-preview-bar" role="status">
    <Eye :size="15" />
    <span>{{ t('storefrontTheme.preview', { n: themePreview?.theme.slice(-1) }) }}</span>
    <span class="preview-note">{{ t('storefrontTheme.previewNote') }}</span>
    <button type="button" @click="exit">{{ t('storefrontTheme.exitPreview') }} <X :size="14" /></button>
  </aside>
</template>
<style scoped>
.theme-preview-bar { display:flex; align-items:center; justify-content:center; gap:10px; padding:9px 16px; background:var(--ui-bg-elevated); color:var(--ui-text-primary); border-bottom:1px solid var(--ui-border); font-size:12px; }
button { display:inline-flex; align-items:center; gap:6px; padding:3px 9px; border:1px solid var(--ui-border); border-radius:6px; cursor:pointer; }
@media(max-width:640px){ .preview-note{display:none} }
</style>
