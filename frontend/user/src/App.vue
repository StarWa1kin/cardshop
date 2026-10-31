<template>
  <div id="app" class="min-h-screen bg-background text-foreground flex flex-col">
    <ThemePreviewBar v-if="themePreview && !isResellerConsole" />
    <!-- vault 模板：自带顶栏/页脚的外壳包裹页面（控制台仍走下方分支） -->
    <component :is="storefrontLayout" v-if="storefrontLayout && !isResellerConsole">
      <ErrorBoundary>
        <RouterView v-slot="{ Component }">
          <Transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </ErrorBoundary>
    </component>

    <!-- classic 模板 / 分销控制台（保持原有结构不变） -->
    <template v-else>
      <Navbar v-if="!isResellerConsole" />
      <main class="flex-1" :class="isResellerConsole ? '' : 'pb-14 lg:pb-0'">
        <ErrorBoundary>
          <RouterView v-slot="{ Component }">
            <Transition name="page-fade" mode="out-in">
              <component :is="Component" />
            </Transition>
          </RouterView>
        </ErrorBoundary>
      </main>
      <Footer v-if="!isResellerConsole" />
      <BackToTop v-if="!isResellerConsole" />
      <MobileBottomNav v-if="!isResellerConsole" />
    </template>

    <Loading :loading="appStore.loading" />
    <Toast />
    <ConfirmDialog />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, watchEffect, type Component } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from './stores/app'
import { getActiveTheme, themePreview } from './templates/registry'
import ThemePreviewBar from './templates/ThemePreviewBar.vue'
import type { StorefrontTheme } from '../../shared/storefrontTheme'
import Navbar from './components/Navbar.vue'
import Footer from './components/Footer.vue'
import Loading from './components/Loading.vue'
import Toast from './components/Toast.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ErrorBoundary from './components/ErrorBoundary.vue'
import BackToTop from './components/BackToTop.vue'
import MobileBottomNav from './components/MobileBottomNav.vue'

// vault 外壳按需加载，classic 用户不会拉取其 chunk/样式
const VaultLayout = defineAsyncComponent(() => import('./templates/vault/layout/VaultLayout.vue'))

// config 由 router.beforeEach 统一加载，无需在此重复调用
const appStore = useAppStore()
const route = useRoute()
const isResellerConsole = computed(() => route.meta.resellerConsole === true)
// Resolve after public config, then keep the document coherent until a refresh.
const TechLayout = defineAsyncComponent(() => import('./templates/tech/TechLayout.vue'))
const CyberLayout = defineAsyncComponent(() => import('./templates/cyber/CyberLayout.vue'))
const layouts: Partial<Record<StorefrontTheme, Component>> = { 'theme-2': VaultLayout, 'theme-3': TechLayout, 'theme-4': CyberLayout }
const activeTheme = computed(() => getActiveTheme())
const storefrontLayout = computed(() => layouts[activeTheme.value] ?? null)
watchEffect(() => {
  if (isResellerConsole.value) delete document.documentElement.dataset.storefrontTheme
  else document.documentElement.dataset.storefrontTheme = activeTheme.value
})
</script>

<style>
.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 200ms ease;
}

.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>
