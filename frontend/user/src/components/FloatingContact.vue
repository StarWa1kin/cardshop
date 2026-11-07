<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Headset, MessageCircle, Send, X } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import ContactAccounts from '@/components/ContactAccounts.vue'

const props = defineProps<{
  /** classic 主题底部还有移动端导航与「返回顶部」按钮，需要向上让位 */
  stacked?: boolean
}>()

const { t } = useI18n()
const appStore = useAppStore()

const open = ref(false)
const rootEl = ref<HTMLElement | null>(null)

const contact = computed(() => appStore.config?.contact as Record<string, unknown> | undefined)
const contactText = (key: string) => {
  const value = contact.value?.[key]
  return typeof value === 'string' ? value.trim() : ''
}

/** 可直接跳转的外链渠道（后台填写的是完整链接） */
const links = computed(() =>
  [
    { key: 'telegram', label: 'Telegram', icon: Send, tone: 'text-sky-500', url: contactText('telegram') },
    { key: 'whatsapp', label: 'WhatsApp', icon: MessageCircle, tone: 'text-emerald-500', url: contactText('whatsapp') },
  ].filter((item) => item.url !== ''),
)

/** QQ / 微信走弹窗展示账号与二维码，交给 ContactAccounts 复用 */
const hasAccountChannels = computed(() =>
  (['qq', 'wechat'] as const).some((key) => contactText(key) !== '' || contactText(`${key}_qrcode`) !== ''),
)

/** 后台未配置任何联系方式时不渲染，避免出现空面板 */
const visible = computed(() => links.value.length > 0 || hasAccountChannels.value)

watch(visible, (value) => {
  if (!value) open.value = false
})

// 必须用 pointerdown：click 冒泡到 document 前 Vue 已重渲染，
// 按钮内的图标元素被替换成另一个节点，contains() 会把自己误判成「点击外部」而立刻收起。
const onPointerDown = (event: Event) => {
  if (!open.value) return
  const target = event.target as Node | null
  if (target && rootEl.value && !rootEl.value.contains(target)) open.value = false
}

const onKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') open.value = false
}

onMounted(() => {
  document.addEventListener('pointerdown', onPointerDown)
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onPointerDown)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div
    v-if="visible"
    ref="rootEl"
    class="floating-contact fixed right-4 z-40 flex flex-col items-end gap-3 md:right-6"
    :class="{ 'floating-contact--stacked': props.stacked }"
  >
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 translate-y-2 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 translate-y-2 scale-95"
    >
      <div
        v-if="open"
        id="floating-contact-panel"
        class="w-64 max-w-[calc(100vw-2rem)] origin-bottom-right rounded-xl border bg-card p-4 text-foreground shadow-xl"
      >
        <p class="text-sm font-bold">{{ t('siteContact.floatingTitle') }}</p>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('siteContact.floatingSubtitle') }}</p>
        <div class="mt-3 flex flex-col gap-1">
          <a
            v-for="link in links"
            :key="link.key"
            :href="link.url"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <component :is="link.icon" :class="['h-4 w-4', link.tone]" />
            {{ link.label }}
          </a>
          <ContactAccounts v-if="hasAccountChannels" variant="panel" />
        </div>
      </div>
    </Transition>

    <button
      type="button"
      class="grid h-12 w-12 place-items-center rounded-full bg-primary text-primary-foreground shadow-lg transition-all hover:-translate-y-0.5 hover:bg-primary/90 hover:shadow-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      :aria-label="open ? t('siteContact.floatingClose') : t('siteContact.floatingOpen')"
      :title="open ? t('siteContact.floatingClose') : t('siteContact.floatingOpen')"
      aria-controls="floating-contact-panel"
      :aria-expanded="open"
      @click="open = !open"
    >
      <X v-if="open" class="h-5 w-5" />
      <Headset v-else class="h-5 w-5" />
    </button>
  </div>
</template>

<style scoped>
.floating-contact {
  bottom: calc(1.5rem + env(safe-area-inset-bottom, 0px));
}

/* classic：移动端让开底部导航(3.5rem)与其上的「返回顶部」按钮(2.75rem) */
.floating-contact--stacked {
  bottom: calc(3.5rem + env(safe-area-inset-bottom, 0px) + 1rem + 2.75rem + 0.75rem);
}

@media (min-width: 1024px) {
  .floating-contact--stacked {
    bottom: calc(1.5rem + 2.75rem + 0.75rem);
  }
}
</style>
