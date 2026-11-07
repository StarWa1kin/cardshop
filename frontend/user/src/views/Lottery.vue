<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Gift, Ticket, Trophy, Clock, Search } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import BreadcrumbNav from '../components/BreadcrumbNav.vue'
import EmptyState from '../components/EmptyState.vue'
import LotteryWheel from '../components/LotteryWheel.vue'
import { copyText } from '../utils/clipboard'
import { useUserAuthStore } from '../stores/userAuth'
import {
  lotteryAPI,
  type LotteryDraw,
  type LotteryDrawableOrder,
  type LotteryWheel as LotteryWheelData,
} from '../api/lottery'

const { t } = useI18n()
const authStore = useUserAuthStore()

/** 转盘动画时长，与 LotteryWheel 的 transition 保持一致。 */
const SPIN_DURATION_MS = 4200
const EXTRA_TURNS = 5

const loading = ref(true)
const wheel = ref<LotteryWheelData | null>(null)
const orders = ref<LotteryDrawableOrder[]>([])
const records = ref<LotteryDraw[]>([])
const selectedOrderNo = ref('')
const drawing = ref(false)
const spinning = ref(false)
const rotation = ref(0)
const resultSliceIndex = ref<number | null>(null)
const latestDraw = ref<LotteryDraw | null>(null)
const errorMessage = ref('')
const copied = ref(false)

const loggedIn = computed(() => authStore.isAuthenticated)
const slices = computed(() => wheel.value?.slices || [])
const activityTitle = computed(() => wheel.value?.title?.trim() || t('lottery.defaultTitle'))
const activityEnabled = computed(() => Boolean(wheel.value?.enabled))
const selectedOrder = computed(() => orders.value.find((item) => item.order_no === selectedOrderNo.value) || null)
const canDraw = computed(() => Boolean(selectedOrder.value) && !drawing.value && !spinning.value)
const wonRecords = computed(() => records.value.filter((item) => item.is_win))

/** 订单多的时候列表要能筛选和滚动，否则右栏会把页面撑得很长。 */
const orderKeyword = ref('')
const showOrderSearch = computed(() => orders.value.length > 6)
const filteredOrders = computed(() => {
  const keyword = orderKeyword.value.trim().toLowerCase()
  if (!keyword) return orders.value
  return orders.value.filter((item) => item.order_no.toLowerCase().includes(keyword))
})
const totalRemaining = computed(() =>
  orders.value.reduce((sum, item) => sum + (Number(item.remaining) || 0), 0),
)

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

const prizeLabel = (draw: LotteryDraw) => (draw.is_win ? draw.prize_name : t('lottery.missLabel'))

const prizeDetail = (draw: LotteryDraw) => {
  if (!draw.is_win) return ''
  switch (draw.prize_type) {
    case 'balance':
      return t('lottery.result.balanceCredited', { amount: `${draw.amount} ${draw.currency || ''}`.trim() })
    case 'coupon':
      return draw.coupon_code ? t('lottery.result.couponCode', { code: draw.coupon_code }) : ''
    case 'product':
      return draw.product_name || ''
    default:
      return ''
  }
}

const loadAll = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const wheelResponse = await lotteryAPI.wheel()
    wheel.value = (wheelResponse.data.data || null) as LotteryWheelData | null
    if (!wheel.value?.enabled) return

    const [orderResponse, recordResponse] = await Promise.all([
      lotteryAPI.drawableOrders(),
      lotteryAPI.records({ page: 1, page_size: 10 }),
    ])
    orders.value = (orderResponse.data.data || []) as LotteryDrawableOrder[]
    records.value = (recordResponse.data.data || []) as LotteryDraw[]
    if (!selectedOrderNo.value || !orders.value.some((item) => item.order_no === selectedOrderNo.value)) {
      selectedOrderNo.value = orders.value[0]?.order_no || ''
    }
  } catch (err: any) {
    errorMessage.value = err?.message || t('lottery.errors.loadFailed')
  } finally {
    loading.value = false
  }
}

/**
 * 把指针转到服务端返回的扇区。
 * 命中结果完全由服务端决定，这里只负责动画；累计角度只增不减，连续抽奖不会倒转。
 */
const spinTo = (sliceIndex: number) => {
  const total = slices.value.length
  if (total <= 0) return
  const safeIndex = sliceIndex >= 0 && sliceIndex < total ? sliceIndex : total - 1
  const sectorAngle = 360 / total
  const center = safeIndex * sectorAngle + sectorAngle / 2
  const base = Math.ceil(rotation.value / 360) * 360
  rotation.value = base + EXTRA_TURNS * 360 + ((360 - center) % 360)
}

const draw = async () => {
  if (!canDraw.value || !selectedOrderNo.value) return
  errorMessage.value = ''
  latestDraw.value = null
  resultSliceIndex.value = null
  copied.value = false
  drawing.value = true
  try {
    const response = await lotteryAPI.draw(selectedOrderNo.value)
    const result = response.data.data
    const sliceIndex = Number(result?.slice_index ?? -1)
    spinning.value = true
    spinTo(sliceIndex)
    window.setTimeout(async () => {
      latestDraw.value = (result?.draw || null) as LotteryDraw | null
      resultSliceIndex.value = sliceIndex
      spinning.value = false
      await loadAll()
    }, SPIN_DURATION_MS)
  } catch (err: any) {
    errorMessage.value = err?.message || t('lottery.errors.drawFailed')
    spinning.value = false
  } finally {
    drawing.value = false
  }
}

const copyCoupon = async (code?: string) => {
  if (!code) return
  try {
    await copyText(code)
    copied.value = true
  } catch {
    copied.value = false
  }
}

onMounted(() => {
  if (loggedIn.value) loadAll()
  else loading.value = false
})
</script>

<template>
  <div class="min-h-screen bg-background text-foreground pt-24 pb-16">
    <div class="container mx-auto px-4">
      <BreadcrumbNav
        class="mb-6"
        :items="[{ label: t('nav.home'), to: '/' }, { label: t('nav.lottery') }]"
      />

      <!-- 活动头图 -->
      <section class="relative mb-8 overflow-hidden rounded-3xl border bg-card p-8 shadow-sm">
        <div class="pointer-events-none absolute -right-16 -top-16 h-56 w-56 rounded-full bg-primary/10 blur-3xl"></div>
        <div class="pointer-events-none absolute -bottom-20 -left-10 h-48 w-48 rounded-full bg-primary/5 blur-3xl"></div>
        <div class="relative">
          <p class="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.24em] text-primary">
            <Gift class="h-4 w-4" />
            {{ t('nav.lottery') }}
          </p>
          <h1 class="mt-3 text-3xl font-black lg:text-4xl">{{ activityTitle }}</h1>
          <p v-if="wheel?.notice" class="mt-3 max-w-2xl whitespace-pre-wrap text-sm text-muted-foreground">
            {{ wheel.notice }}
          </p>
        </div>
      </section>

      <div v-if="loading" class="rounded-3xl border bg-card p-8 shadow-sm">
        <div class="mx-auto h-64 w-64 rounded-full theme-skeleton"></div>
      </div>

      <!-- 未登录 -->
      <EmptyState
        v-else-if="!loggedIn"
        icon="alert"
        :title="t('lottery.page.loginRequired')"
        :description="t('lottery.page.loginRequiredHint')"
        :action-label="t('navbar.login')"
        action-to="/auth/login"
      />

      <!-- 活动未开启 -->
      <EmptyState
        v-else-if="!activityEnabled"
        icon="inbox"
        :title="t('lottery.reasons.disabled')"
        :description="t('lottery.page.disabledHint')"
      />

      <div v-else class="grid grid-cols-1 gap-6 lg:grid-cols-12">
        <!-- 转盘 -->
        <section class="self-start rounded-3xl border bg-card p-6 shadow-sm sm:p-8 lg:sticky lg:top-24 lg:col-span-7">
          <LotteryWheel
            :slices="slices"
            :rotation="rotation"
            :spinning="spinning"
            :disabled="!canDraw"
            :highlight-index="resultSliceIndex"
            :action-label="spinning ? t('lottery.spinning') : ''"
            @spin="draw"
          />

          <p
            v-if="selectedOrder"
            class="mt-6 flex flex-wrap items-center justify-center gap-x-2 gap-y-1 text-xs text-muted-foreground"
          >
            <span>{{ t('lottery.page.currentOrder') }}</span>
            <span class="font-mono text-foreground">{{ selectedOrder.order_no }}</span>
            <Badge variant="accent" size="sm">
              {{ t('lottery.remainingBadge', { count: selectedOrder.remaining }) }}
            </Badge>
          </p>

          <div
            v-if="latestDraw"
            class="mt-8 rounded-2xl border p-5 text-center"
            :class="latestDraw.is_win ? 'border-success/30 bg-success/10' : 'border-border bg-muted/40'"
          >
            <p class="text-base font-bold" :class="latestDraw.is_win ? 'text-success' : 'text-muted-foreground'">
              {{ latestDraw.is_win ? t('lottery.result.winTitle') : t('lottery.result.missTitle') }}
            </p>
            <p class="mt-1 text-lg font-black">{{ prizeLabel(latestDraw) }}</p>
            <p v-if="prizeDetail(latestDraw)" class="mt-1 break-all text-sm text-muted-foreground">
              {{ prizeDetail(latestDraw) }}
            </p>
            <Button
              v-if="latestDraw.prize_type === 'coupon' && latestDraw.coupon_code"
              size="sm"
              variant="outline"
              class="mt-3"
              @click="copyCoupon(latestDraw.coupon_code)"
            >
              {{ copied ? t('lottery.result.copied') : t('lottery.result.copyCoupon') }}
            </Button>
            <p v-if="latestDraw.delivery_status === 'pending'" class="mt-2 text-xs text-muted-foreground">
              {{ t('lottery.result.pendingHint') }}
            </p>
          </div>

          <Alert v-if="errorMessage" variant="destructive" class="mt-6">
            <AlertDescription>{{ errorMessage }}</AlertDescription>
          </Alert>
        </section>

        <!-- 右栏：选订单 + 中奖记录 -->
        <aside class="space-y-6 lg:col-span-5">
          <section class="rounded-3xl border bg-card p-6 shadow-sm">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="flex items-center gap-2 text-base font-bold">
                <Ticket class="h-4 w-4 text-primary" />
                {{ t('lottery.page.pickOrder') }}
              </h2>
              <Badge v-if="orders.length > 0" variant="neutral" size="sm">
                {{ t('lottery.page.orderSummary', { orders: orders.length, count: totalRemaining }) }}
              </Badge>
            </div>

            <p v-if="orders.length === 0" class="mt-4 text-sm text-muted-foreground">
              {{ t('lottery.page.noDrawableOrder') }}
            </p>

            <template v-else>
              <div v-if="showOrderSearch" class="relative mt-4">
                <Search class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  v-model="orderKeyword"
                  class="h-9 pl-8 text-xs"
                  :placeholder="t('lottery.page.searchOrder')"
                />
              </div>

              <p v-if="filteredOrders.length === 0" class="mt-4 text-sm text-muted-foreground">
                {{ t('lottery.page.noMatchedOrder') }}
              </p>

              <div
                v-else
                class="lottery-order-list mt-4 space-y-2 overflow-y-auto pr-1"
                :class="orders.length > 4 ? 'max-h-[19rem] lg:max-h-[21rem]' : ''"
              >
                <button
                  v-for="order in filteredOrders"
                  :key="order.order_no"
                  type="button"
                  class="flex w-full items-center justify-between gap-3 rounded-xl border px-3 py-2.5 text-left transition-colors"
                  :class="order.order_no === selectedOrderNo
                    ? 'border-primary bg-primary/5'
                    : 'border-border hover:border-primary/40 hover:bg-accent'"
                  @click="selectedOrderNo = order.order_no"
                >
                  <span class="min-w-0">
                    <span class="block truncate font-mono text-xs text-foreground">{{ order.order_no }}</span>
                    <span class="mt-0.5 block truncate text-[11px] text-muted-foreground">
                      {{ order.total_amount }} {{ order.currency }}
                      <span v-if="order.expires_at"> · {{ t('lottery.info.expiresAt', { time: formatDate(order.expires_at) }) }}</span>
                    </span>
                  </span>
                  <Badge :variant="order.order_no === selectedOrderNo ? 'accent' : 'neutral'" size="sm" class="shrink-0">
                    {{ t('lottery.remainingBadge', { count: order.remaining }) }}
                  </Badge>
                </button>
              </div>
            </template>

            <p class="mt-4 flex items-start gap-1.5 text-xs text-muted-foreground">
              <Clock class="mt-0.5 h-3.5 w-3.5 shrink-0" />
              <span>
                {{ t('lottery.info.perOrder', { count: wheel?.draws_per_order || 1 }) }}
                <template v-if="(wheel?.daily_draw_limit || 0) > 0">
                  · {{ t('lottery.info.dailyLimit', { count: wheel?.daily_draw_limit }) }}
                </template>
              </span>
            </p>
          </section>

          <section class="rounded-3xl border bg-card p-6 shadow-sm">
            <div class="flex items-center justify-between">
              <h2 class="flex items-center gap-2 text-base font-bold">
                <Trophy class="h-4 w-4 text-primary" />
                {{ t('lottery.page.recentWins') }}
              </h2>
              <Button as-child size="sm" variant="ghost">
                <router-link to="/me/lottery">{{ t('lottery.page.allRecords') }}</router-link>
              </Button>
            </div>

            <p v-if="wonRecords.length === 0" class="mt-4 text-sm text-muted-foreground">
              {{ t('lottery.page.noWinYet') }}
            </p>
            <ul v-else class="lottery-order-list mt-4 max-h-[16rem] space-y-2 overflow-y-auto pr-1">
              <li
                v-for="item in wonRecords"
                :key="item.id"
                class="flex items-center justify-between gap-3 rounded-xl border px-4 py-2.5 text-xs"
              >
                <span class="font-semibold text-foreground">{{ item.prize_name }}</span>
                <span class="text-muted-foreground">{{ t(`lottery.delivery.${item.delivery_status || 'none'}`) }}</span>
                <span class="text-muted-foreground">{{ formatDate(item.created_at) }}</span>
              </li>
            </ul>
          </section>
        </aside>
      </div>
    </div>
  </div>
</template>

<style scoped>
/*
 * 订单多的时候列表内部滚动，滚动条收窄，避免把右栏拉得过长。
 * 颜色用 --ui-* 令牌（深浅主题各自覆盖），本项目的 token 存的是完整色值，
 * 不能写成 hsl(var(--x))。
 */
.lottery-order-list {
  scrollbar-width: thin;
  scrollbar-color: var(--ui-border-strong) transparent;
  overscroll-behavior: contain;
}

.lottery-order-list::-webkit-scrollbar {
  width: 6px;
}

.lottery-order-list::-webkit-scrollbar-track {
  background: transparent;
}

.lottery-order-list::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background-color: var(--ui-border-strong);
  border: none;
}

.lottery-order-list::-webkit-scrollbar-thumb:hover {
  background-color: var(--ui-text-muted);
}
</style>
