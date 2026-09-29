<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, ArrowRight, Layers3, Search, SlidersHorizontal, ShoppingBag } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import { getLocalizedText } from '../../utils/resellerSiteConfig'
const { t } = useI18n()
const app = useAppStore()
const description = computed(() => getLocalizedText(app.config?.brand?.site_description, app.locale) || t('storefrontTheme.heroSubtitle'))
</script>
<template>
  <section class="tech-hero" aria-labelledby="tech-hero-title">
    <div class="tech-hero-copy">
      <span class="tech-eyebrow"><span class="tech-signal" /> DIGITAL GOODS, REIMAGINED.</span>
      <h1 id="tech-hero-title">{{ t('storefrontTheme.heroTitle') }}<br /><span>{{ t('storefrontTheme.heroTitleAccent') }}</span></h1>
      <p>{{ description }}</p>
      <div class="tech-hero-actions">
        <a href="#storefront-products" class="tech-primary-button">{{ t('storefrontTheme.explore') }} <ArrowUpRight :size="18" /></a>
        <RouterLink to="/guest/orders" class="tech-text-button">{{ t('navbar.guestOrders') }} <ArrowRight :size="16" /></RouterLink>
      </div>
    </div>
    <div class="tech-sculpture" aria-hidden="true">
      <div class="tech-orbit" /><div class="tech-grid" />
      <span class="tech-art-coordinate">01 / DIGITAL ACCESS</span>
      <div class="tech-glass-card glass-back" />
      <div class="tech-glass-card glass-middle" />
      <div class="tech-glass-card glass-front">
        <Layers3 :size="45" :stroke-width="1.2" />
        <div class="tech-chip"><i /><i /><i /></div>
        <span>YOUR NEXT<br />DIGITAL EXPERIENCE.</span>
        <div class="tech-card-line" /><small>C / S <ArrowUpRight :size="14" /></small>
      </div>
      <span class="tech-art-caption">LESS FRICTION.<br />MORE POSSIBILITIES.</span>
      <span class="tech-cross cross-one">+</span><span class="tech-cross cross-two">+</span>
    </div>
  </section>
  <div class="tech-benefits">
    <span><Search :size="20" /><span>{{ t('storefrontTheme.browse') }}<small>{{ t('storefrontTheme.browseHint') }}</small></span></span>
    <span><SlidersHorizontal :size="20" /><span>{{ t('storefrontTheme.choose') }}<small>{{ t('storefrontTheme.chooseHint') }}</small></span></span>
    <RouterLink to="/guest/orders"><ShoppingBag :size="20" /><span>{{ t('navbar.guestOrders') }}<small>{{ t('storefrontTheme.ordersHint') }}</small></span><ArrowUpRight :size="14" /></RouterLink>
  </div>
</template>
