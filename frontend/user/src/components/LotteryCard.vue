<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Gift, Sparkles } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import LotteryWheel from './LotteryWheel.vue'
import { copyText } from '../utils/clipboard'
import {
  lotteryAPI,
  type LotteryChance,
  type LotteryDraw,
  type LotteryWheel as LotteryWheelData,
} from '../api/lottery'

const props = defineProps<{ orderNo?: string }>()

const { t } = useI18n()

/** 转盘动画时长，与 LotteryWheel 的 transition 保持一致。 */
const SPIN_DURATION_MS = 4000
const EXTRA_TURNS = 5

const loading = ref(true)
const wheel = ref<LotteryWheelData | null>(null)
const chance = ref<LotteryChance | null>(null)
const drawing = ref(false)
const spinning = ref(false)
const rotation = ref(0)
const errorMessage = ref('')
const latestDraw = ref<LotteryDraw | null>(null)
const resultSliceIndex = ref<number | null>(null)
const copied = ref(false)

const slices = computed(() => wheel.value?.slices || [])

/** 活动关了、订单不是本人的、或根本没付款，就完全不渲染入口。 */
const visible = computed(() => {
  if (!wheel.value?.enabled || !chance.value) return false
  return chance.value.reason !== 'order_not_found' && chance.value.reason !== 'not_paid'
})

const canDraw = computed(() => Boolean(chance.value?.eligible) && !drawing.value && !spinning.value)

const activityTitle = computed(() => wheel.value?.title?.trim() || t('lottery.defaultTitle'))

const reasonText = computed(() => {
  const reason = chance.value?.reason
  if (!reason || reason === 'ok') return ''
  return t(`lottery.reasons.${reason}`)
})

const historyDraws = computed(() => chance.value?.draws || [])

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

const prizeLabel = (draw: LotteryDraw) => {
  if (!draw.is_win) return t('lottery.missLabel')
  return draw.prize_name || t('lottery.prizeTypes.custom')
}

const prizeDetail = (draw: LotteryDraw) => {
  if (!draw.is_win) return ''
  switch (draw.prize_type) {
    case 'balance':
      return t('lottery.result.balanceCredited', {
        amount: `${draw.amount} ${draw.currency || ''}`.trim(),
      })
    case 'coupon':
      return draw.coupon_code ? t('lottery.result.couponCode', { code: draw.coupon_code }) : ''
    case 'product':
      return draw.product_name || ''
    default:
      return ''
  }
}

const deliveryText = (draw: LotteryDraw) => {
  if (!draw.is_win) return ''
  return t(`lottery.delivery.${draw.delivery_status || 'none'}`)
}

const loadWheel = async () => {
  try {
    const response = await lotteryAPI.wheel()
    wheel.value = (response.data.data || null) as LotteryWheelData | null
  } catch {
    wheel.value = null
  }
}

const loadChance = async () => {
  const orderNo = String(props.orderNo || '').trim()
  if (!orderNo) {
    chance.value = null
    return
  }
  try {
    const response = await lotteryAPI.chance(orderNo)
    chance.value = (response.data.data || null) as LotteryChance | null
  } catch {
    chance.value = null
  }
}

const reload = async () => {
  loading.value = true
  try {
    await loadWheel()
    if (wheel.value?.enabled) await loadChance()
  } finally {
    loading.value = false
  }
}

/**
 * 把指针转到服务端返回的扇区。
 *
 * 命中结果完全由服务端决定，这里只负责动画：累计角度只增不减，
 * 所以连续抽奖时转盘始终向前转，不会出现倒转。
 */
const spinTo = (sliceIndex: number) => {
  const total = slices.value.length
  if (total <= 0) return
  const safeIndex = sliceIndex >= 0 && sliceIndex < total ? sliceIndex : total - 1
  const sectorAngle = 360 / total
  const center = safeIndex * sectorAngle + sectorAngle / 2
  const current = rotation.value
  const base = Math.ceil(current / 360) * 360
  rotation.value = base + EXTRA_TURNS * 360 + ((360 - center) % 360)
}

const draw = async () => {
  const orderNo = String(props.orderNo || '').trim()
  if (!orderNo || !canDraw.value) return
  errorMessage.value = ''
  latestDraw.value = null
  resultSliceIndex.value = null
  copied.value = false
  drawing.value = true
  try {
    const response = await lotteryAPI.draw(orderNo)
    const result = response.data.data
    const sliceIndex = Number(result?.slice_index ?? -1)
    spinning.value = true
    spinTo(sliceIndex)
    // 转盘停下再揭晓结果，否则用户先看到文字就没有悬念了。
    window.setTimeout(async () => {
      latestDraw.value = (result?.draw || null) as LotteryDraw | null
      resultSliceIndex.value = sliceIndex
      spinning.value = false
      await loadChance()
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

watch(() => props.orderNo, () => { reload() })

onMounted(() => {
  reload()
})
</script>

<template>
  <div v-if="!loading && visible" class="rounded-2xl border bg-card p-6 shadow-sm">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <Gift class="h-5 w-5 text-primary" />
        <h2 class="text-lg font-bold">{{ activityTitle }}</h2>
      </div>
      <Badge :variant="chance?.eligible ? 'accent' : 'neutral'" size="sm">
        {{ t('lottery.remainingBadge', { count: chance?.remaining || 0 }) }}
      </Badge>
    </div>

    <p v-if="wheel?.notice" class="mb-4 whitespace-pre-wrap text-sm text-muted-foreground">{{ wheel.notice }}</p>

    <div class="grid grid-cols-1 gap-6 md:grid-cols-2 md:items-center">
      <LotteryWheel
        :slices="slices"
        :rotation="rotation"
        :spinning="spinning"
        :disabled="!canDraw"
        :highlight-index="resultSliceIndex"
        :action-label="spinning ? t('lottery.spinning') : ''"
        @spin="draw"
      />

      <div class="space-y-4">
        <div class="space-y-1 text-sm text-muted-foreground">
          <p>{{ t('lottery.info.perOrder', { count: wheel?.draws_per_order || 1 }) }}</p>
          <p v-if="chance?.expires_at">{{ t('lottery.info.expiresAt', { time: formatDate(chance.expires_at) }) }}</p>
          <p v-if="(wheel?.daily_draw_limit || 0) > 0">
            {{ t('lottery.info.dailyLimit', { count: wheel?.daily_draw_limit }) }}
          </p>
        </div>

        <p v-if="canDraw" class="flex items-center gap-1.5 text-sm font-medium text-primary">
          <Sparkles class="h-4 w-4" />
          {{ t('lottery.hubHint') }}
        </p>

        <Alert v-if="!chance?.eligible && reasonText" variant="default">
          <AlertDescription>{{ reasonText }}</AlertDescription>
        </Alert>
        <Alert v-if="errorMessage" variant="destructive">
          <AlertDescription>{{ errorMessage }}</AlertDescription>
        </Alert>

        <div
          v-if="latestDraw"
          class="rounded-xl border p-4"
          :class="latestDraw.is_win ? 'border-success/30 bg-success/10' : 'border-border bg-muted/40'"
        >
          <p class="text-sm font-bold" :class="latestDraw.is_win ? 'text-success' : 'text-muted-foreground'">
            {{ latestDraw.is_win ? t('lottery.result.winTitle') : t('lottery.result.missTitle') }}
          </p>
          <p class="mt-1 text-sm text-foreground">{{ prizeLabel(latestDraw) }}</p>
          <p v-if="prizeDetail(latestDraw)" class="mt-1 text-xs text-muted-foreground break-all">
            {{ prizeDetail(latestDraw) }}
          </p>
          <div v-if="latestDraw.prize_type === 'coupon' && latestDraw.coupon_code" class="mt-2">
            <Button size="sm" variant="outline" @click="copyCoupon(latestDraw.coupon_code)">
              {{ copied ? t('lottery.result.copied') : t('lottery.result.copyCoupon') }}
            </Button>
          </div>
          <p v-if="latestDraw.delivery_status === 'pending'" class="mt-2 text-xs text-muted-foreground">
            {{ t('lottery.result.pendingHint') }}
          </p>
        </div>
      </div>
    </div>

    <div v-if="historyDraws.length" class="mt-6 border-t pt-4">
      <p class="mb-3 text-sm font-semibold text-foreground">{{ t('lottery.historyTitle') }}</p>
      <ul class="space-y-2">
        <li
          v-for="item in historyDraws"
          :key="item.id"
          class="flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-xs"
        >
          <span class="font-medium text-foreground">{{ prizeLabel(item) }}</span>
          <span class="text-muted-foreground">{{ prizeDetail(item) }}</span>
          <span v-if="item.is_win" class="text-muted-foreground">{{ deliveryText(item) }}</span>
          <span class="text-muted-foreground">{{ formatDate(item.created_at) }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>
