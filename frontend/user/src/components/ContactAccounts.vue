<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogRoot, DialogTrigger, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription, DialogClose } from 'reka-ui'
import { MessageCircle, Copy, X } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { getImageUrl } from '@/utils/image'

const props = defineProps<{ compact?: boolean }>()
const { t } = useI18n()
const appStore = useAppStore()
const feedback = ref('')
const imageFailed = ref(false)
const channels = computed(() => {
  const contact = appStore.config?.contact
  return (['qq', 'wechat'] as const).map(key => ({
    key,
    label: key === 'qq' ? 'QQ' : t('siteContact.wechat'),
    account: typeof contact?.[key] === 'string' ? contact[key].trim() : '',
    qr: typeof contact?.[`${key}_qrcode`] === 'string' ? contact[`${key}_qrcode`].trim() : '',
  })).filter(item => item.account || item.qr)
})
async function copyAccount(account: string) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(account)
    } else {
      throw new Error('Clipboard unavailable')
    }
    feedback.value = t('siteContact.copied')
  } catch {
    feedback.value = t('siteContact.copyFailed')
  }
}
</script>

<template>
  <DialogRoot v-for="channel in channels" :key="channel.key" @update:open="feedback = ''; imageFailed = false">
    <DialogTrigger as-child>
      <button type="button" :class="props.compact
        ? 'flex items-center gap-2 py-1 text-sm text-muted-foreground transition-colors hover:text-primary'
        : 'flex items-center justify-center gap-3 rounded-xl border bg-secondary px-6 py-4 font-bold text-muted-foreground transition-colors hover:text-foreground'">
        <MessageCircle :class="[props.compact ? 'h-4 w-4' : 'h-6 w-6', channel.key === 'qq' ? 'text-sky-500' : 'text-emerald-500']" />
        {{ channel.label }}
      </button>
    </DialogTrigger>
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[100] bg-black/50" />
      <DialogContent class="fixed left-1/2 top-1/2 z-[101] max-h-[85dvh] w-[calc(100%-2rem)] max-w-sm -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-2xl border bg-card p-6 text-foreground shadow-xl">
        <DialogTitle class="pr-8 text-xl font-bold">{{ channel.label }}</DialogTitle>
        <DialogDescription class="mt-2 text-sm text-muted-foreground">{{ t(channel.qr ? 'siteContact.scanHint' : 'siteContact.accountHint', { channel: channel.label }) }}</DialogDescription>
        <div v-if="channel.qr" class="mt-5">
          <img v-if="!imageFailed" :src="getImageUrl(channel.qr)" :alt="t('siteContact.qrAlt', { channel: channel.label })" class="mx-auto aspect-square w-full max-w-64 rounded-lg bg-white p-2 object-contain" @error="imageFailed = true" />
          <p v-else role="status" class="rounded-lg border p-4 text-sm text-muted-foreground">{{ t('siteContact.imageFailed') }}</p>
        </div>
        <div v-if="channel.account" class="mt-5 space-y-3">
          <p class="select-all break-all rounded-lg bg-secondary p-3 text-center font-mono text-base">{{ channel.account }}</p>
          <button type="button" class="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90" @click="copyAccount(channel.account)">
            <Copy class="h-4 w-4" />{{ t('siteContact.copyAccount') }}
          </button>
          <p role="status" aria-live="polite" class="text-center text-sm text-muted-foreground">{{ feedback }}</p>
        </div>
        <DialogClose :aria-label="t('siteContact.close')" class="absolute right-4 top-4 rounded-md p-1 text-muted-foreground hover:text-foreground"><X class="h-5 w-5" /></DialogClose>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
