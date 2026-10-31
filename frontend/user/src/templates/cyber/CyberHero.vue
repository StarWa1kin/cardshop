<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ChevronRight, Search, SlidersHorizontal, ShoppingBag } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import { getLocalizedText } from '../../utils/resellerSiteConfig'
const { t } = useI18n()
const app = useAppStore()
const accent = computed(() => t('storefrontTheme.cyber.titleAccent'))
const description = computed(() => getLocalizedText(app.config?.brand?.site_description, app.locale) || t('storefrontTheme.cyber.subtitle'))
const bootLines = computed(() => [1, 2, 3].map(n => t(`storefrontTheme.cyber.boot${n}`)))
</script>
<template>
  <section class="cyber-hero" aria-labelledby="cyber-hero-title">
    <div class="cyber-hero-bg" aria-hidden="true">
      <div class="cyber-sun" /><div class="cyber-floor" /><div class="cyber-scan" />
    </div>
    <div class="cyber-hero-inner">
      <div class="cyber-hero-copy">
        <span class="cyber-eyebrow"><i class="cyber-blink" />{{ t('storefrontTheme.cyber.eyebrow') }}</span>
        <h1 id="cyber-hero-title">
          {{ t('storefrontTheme.cyber.title') }}
          <span class="cyber-glitch" :data-text="accent">{{ accent }}</span>
        </h1>
        <p>{{ description }}</p>
        <div class="cyber-hero-actions">
          <a href="#storefront-products" class="cyber-btn">{{ t('storefrontTheme.cyber.explore') }} <ArrowRight :size="18" /></a>
          <RouterLink to="/guest/orders" class="cyber-link">{{ t('navbar.guestOrders') }} <ChevronRight :size="16" /></RouterLink>
        </div>
      </div>
      <div class="cyber-terminal" aria-hidden="true">
        <div class="cyber-terminal-bar"><i /><i /><i /><span>NET://NIGHT-MARKET</span></div>
        <div class="cyber-terminal-body">
          <div v-for="(line, i) in bootLines" :key="i" class="cyber-line" :style="{ '--i': i }">
            <b>&gt;</b><span>{{ line }}</span><span class="cyber-leader" /><em>OK</em>
          </div>
          <div class="cyber-line" :style="{ '--i': 3 }"><b>&gt;</b><span>{{ t('storefrontTheme.cyber.boot4') }}</span><i class="cyber-cursor" /></div>
        </div>
        <span class="cyber-terminal-tag">SECTOR 07 // NEON DISTRICT</span>
      </div>
    </div>
  </section>
  <div class="cyber-benefits">
    <span><Search :size="20" /><span>{{ t('storefrontTheme.browse') }}<small>{{ t('storefrontTheme.browseHint') }}</small></span></span>
    <span><SlidersHorizontal :size="20" /><span>{{ t('storefrontTheme.choose') }}<small>{{ t('storefrontTheme.chooseHint') }}</small></span></span>
    <RouterLink to="/guest/orders"><ShoppingBag :size="20" /><span>{{ t('navbar.guestOrders') }}<small>{{ t('storefrontTheme.ordersHint') }}</small></span></RouterLink>
  </div>
</template>
