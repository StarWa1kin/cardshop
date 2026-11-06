<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  adminAPI,
  type AdminLotteryConfig,
  type AdminLotteryConfigPayload,
  type AdminLotteryPrize,
  type AdminLotteryPrizePayload,
  type AdminLotteryPrizeType,
} from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { confirmAction } from '@/utils/confirm'

const { t } = useI18n()

/** 概率在后端以万分比存储（1 = 0.01%），后台表单用百分数输入，这里做换算。 */
const PROBABILITY_SCALE = 10000
const prizeTypes: AdminLotteryPrizeType[] = ['none', 'balance', 'coupon', 'product', 'custom']
/** 与前台转盘默认配色保持一致，留空时从这里挑一个还没被占用的颜色。 */
const PRIZE_COLOR_PALETTE = ['#f97316', '#fbbf24', '#10b981', '#3b82f6', '#8b5cf6', '#ec4899', '#14b8a6', '#f43f5e']

const configLoading = ref(true)
const configSaving = ref(false)
const configError = ref('')
const configSuccess = ref('')
const configForm = reactive({
  enabled: false,
  title: '',
  notice: '',
  minOrderAmount: '0',
  startsAt: '',
  drawsPerOrder: 1,
  dailyDrawLimit: 0,
  validHours: 72,
})

const prizesLoading = ref(true)
const prizes = ref<AdminLotteryPrize[]>([])
const configuredTotal = ref(0)
const remainderProbability = ref(PROBABILITY_SCALE)
const prizeError = ref('')
const prizeSuccess = ref('')

const showPrizeModal = ref(false)
const prizeSubmitting = ref(false)
const prizeFormError = ref('')
const editingPrize = ref<AdminLotteryPrize | null>(null)
const prizeForm = reactive({
  name: '',
  type: 'none' as AdminLotteryPrizeType,
  probabilityPercent: '0',
  unlimitedStock: true,
  stock: 0,
  amount: '',
  couponID: '',
  productID: '',
  icon: '',
  color: '',
  description: '',
  sortOrder: 0,
  enabled: true,
})

/** 奖品留空颜色时自动分配的颜色，开弹窗时就算好，便于在取色器里预览。 */
const autoColor = ref(PRIZE_COLOR_PALETTE[0])

const hslToHex = (h: number, s: number, l: number) => {
  const a = (s / 100) * Math.min(l / 100, 1 - l / 100)
  const channel = (n: number) => {
    const k = (n + h / 30) % 12
    const value = l / 100 - a * Math.max(-1, Math.min(k - 3, Math.min(9 - k, 1)))
    return Math.round(255 * value)
      .toString(16)
      .padStart(2, '0')
  }
  return `#${channel(0)}${channel(8)}${channel(4)}`
}

/** 随机挑一个当前奖品列表里没用过的颜色；调色板用尽时按随机色相生成。 */
const randomUnusedColor = () => {
  const editingID = editingPrize.value?.id ? Number(editingPrize.value.id) : 0
  const used = new Set(
    prizes.value
      .filter((item) => !editingID || Number(item.id) !== editingID)
      .map((item) => String(item.color || '').trim().toLowerCase())
      .filter(Boolean),
  )
  const available = PRIZE_COLOR_PALETTE.filter((hex) => !used.has(hex))
  const pool = available.length > 0 ? available : []
  if (pool.length > 0) return pool[Math.floor(Math.random() * pool.length)]
  return hslToHex(Math.floor(Math.random() * 360), 72, 55)
}

const colorSwatch = computed(() => prizeForm.color || autoColor.value)

const onColorPick = (event: Event) => {
  prizeForm.color = String((event.target as HTMLInputElement).value || '').toLowerCase()
}

const clearPrizeColor = () => {
  prizeForm.color = ''
  autoColor.value = randomUnusedColor()
}

const toISO = (raw?: string) => {
  const text = String(raw || '').trim()
  if (!text) return ''
  const date = new Date(text)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString()
}

const toLocalInput = (raw?: string | null) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  const offset = date.getTimezoneOffset()
  return new Date(date.getTime() - offset * 60000).toISOString().slice(0, 16)
}

const toPercent = (basisPoints: number) => {
  const value = Number(basisPoints || 0) / (PROBABILITY_SCALE / 100)
  return String(Number(value.toFixed(2)))
}

const toBasisPoints = (percent: string) => {
  const value = Number(String(percent || '').trim())
  if (!Number.isFinite(value) || value < 0) return -1
  return Math.round(value * (PROBABILITY_SCALE / 100))
}

const configuredPercent = computed(() => toPercent(configuredTotal.value))
const remainderPercent = computed(() => toPercent(remainderProbability.value))
const probabilityExceeded = computed(() => configuredTotal.value > PROBABILITY_SCALE)

const prizeTypeLabel = (type: string) => t(`admin.lottery.prizeTypes.${type || 'none'}`)

const stockText = (prize: AdminLotteryPrize) =>
  Number(prize.stock) < 0 ? t('admin.lottery.unlimited') : String(prize.stock)

const prizeValueText = (prize: AdminLotteryPrize) => {
  switch (prize.type) {
    case 'balance':
      return prize.amount || '-'
    case 'coupon':
      return prize.coupon_id ? `#${prize.coupon_id}` : '-'
    case 'product':
      return prize.product_id ? `#${prize.product_id}` : '-'
    default:
      return '-'
  }
}

const fetchConfig = async () => {
  configLoading.value = true
  configError.value = ''
  try {
    const response = await adminAPI.getLotteryConfig()
    const config = (response.data.data || {}) as AdminLotteryConfig
    configForm.enabled = Boolean(config.enabled)
    configForm.title = String(config.title || '')
    configForm.notice = String(config.notice || '')
    configForm.minOrderAmount = String(config.min_order_amount ?? '0')
    configForm.startsAt = toLocalInput(config.starts_at)
    configForm.drawsPerOrder = Number(config.draws_per_order || 1)
    configForm.dailyDrawLimit = Number(config.daily_draw_limit || 0)
    configForm.validHours = Number(config.valid_hours ?? 72)
  } catch (err: any) {
    configError.value = err?.message || t('admin.lottery.errors.configFetchFailed')
  } finally {
    configLoading.value = false
  }
}

const saveConfig = async () => {
  configError.value = ''
  configSuccess.value = ''
  configSaving.value = true
  try {
    const payload: AdminLotteryConfigPayload = {
      enabled: configForm.enabled,
      title: String(configForm.title || '').trim(),
      notice: String(configForm.notice || '').trim(),
      min_order_amount: String(configForm.minOrderAmount || '0').trim() || '0',
      starts_at: configForm.startsAt ? toISO(configForm.startsAt) : '',
      draws_per_order: Number(configForm.drawsPerOrder || 1),
      daily_draw_limit: Number(configForm.dailyDrawLimit || 0),
      valid_hours: Number(configForm.validHours || 0),
    }
    await adminAPI.updateLotteryConfig(payload)
    configSuccess.value = t('admin.lottery.alerts.configSaved')
    await fetchConfig()
  } catch (err: any) {
    configError.value = err?.message || t('admin.lottery.errors.configSaveFailed')
  } finally {
    configSaving.value = false
  }
}

const fetchPrizes = async () => {
  prizesLoading.value = true
  try {
    const response = await adminAPI.getLotteryPrizes()
    const overview = response.data.data || {}
    prizes.value = (overview.prizes || []) as AdminLotteryPrize[]
    configuredTotal.value = Number(overview.configured_total || 0)
    remainderProbability.value = Number(overview.remainder_probability || 0)
  } catch (err: any) {
    prizeError.value = err?.message || t('admin.lottery.errors.prizeFetchFailed')
    prizes.value = []
  } finally {
    prizesLoading.value = false
  }
}

const openPrizeModal = (prize?: AdminLotteryPrize) => {
  prizeFormError.value = ''
  editingPrize.value = prize || null
  prizeForm.name = String(prize?.name || '')
  prizeForm.type = (prize?.type || 'none') as AdminLotteryPrizeType
  prizeForm.probabilityPercent = toPercent(Number(prize?.probability || 0))
  prizeForm.unlimitedStock = prize ? Number(prize.stock) < 0 : true
  prizeForm.stock = prize && Number(prize.stock) >= 0 ? Number(prize.stock) : 0
  prizeForm.amount = String(prize?.amount || '')
  prizeForm.couponID = prize?.coupon_id ? String(prize.coupon_id) : ''
  prizeForm.productID = prize?.product_id ? String(prize.product_id) : ''
  prizeForm.icon = String(prize?.icon || '')
  prizeForm.color = String(prize?.color || '').trim()
  autoColor.value = randomUnusedColor()
  prizeForm.description = String(prize?.description || '')
  prizeForm.sortOrder = Number(prize?.sort_order || 0)
  prizeForm.enabled = prize ? Boolean(prize.enabled) : true
  showPrizeModal.value = true
}

const closePrizeModal = () => {
  showPrizeModal.value = false
  prizeSubmitting.value = false
  editingPrize.value = null
  prizeFormError.value = ''
}

const submitPrize = async () => {
  prizeFormError.value = ''
  const name = String(prizeForm.name || '').trim()
  if (!name) {
    prizeFormError.value = t('admin.lottery.errors.nameRequired')
    return
  }
  const probability = toBasisPoints(prizeForm.probabilityPercent)
  if (probability < 0 || probability > PROBABILITY_SCALE) {
    prizeFormError.value = t('admin.lottery.errors.probabilityInvalid')
    return
  }
  const payload: AdminLotteryPrizePayload = {
    name,
    type: prizeForm.type,
    probability,
    stock: prizeForm.unlimitedStock ? -1 : Math.max(0, Number(prizeForm.stock || 0)),
    icon: String(prizeForm.icon || '').trim(),
    // 颜色非必填，留空时自动分配一个未被占用的扇区颜色
    color: String(prizeForm.color || '').trim() || randomUnusedColor(),
    description: String(prizeForm.description || '').trim(),
    sort_order: Number(prizeForm.sortOrder || 0),
    enabled: prizeForm.enabled,
  }
  if (prizeForm.type === 'balance') {
    const amount = String(prizeForm.amount || '').trim()
    if (!amount || Number(amount) <= 0) {
      prizeFormError.value = t('admin.lottery.errors.amountRequired')
      return
    }
    payload.amount = amount
  }
  if (prizeForm.type === 'coupon') {
    const couponID = Number(prizeForm.couponID)
    if (!Number.isFinite(couponID) || couponID <= 0) {
      prizeFormError.value = t('admin.lottery.errors.couponRequired')
      return
    }
    payload.coupon_id = Math.floor(couponID)
  }
  if (prizeForm.type === 'product') {
    const productID = Number(prizeForm.productID)
    if (!Number.isFinite(productID) || productID <= 0) {
      prizeFormError.value = t('admin.lottery.errors.productRequired')
      return
    }
    payload.product_id = Math.floor(productID)
  }

  prizeSubmitting.value = true
  try {
    if (editingPrize.value?.id) {
      await adminAPI.updateLotteryPrize(Number(editingPrize.value.id), payload)
    } else {
      await adminAPI.createLotteryPrize(payload)
    }
    prizeSuccess.value = t('admin.lottery.alerts.prizeSaved')
    prizeError.value = ''
    closePrizeModal()
    await fetchPrizes()
  } catch (err: any) {
    prizeFormError.value = err?.message || t('admin.lottery.errors.prizeSaveFailed')
  } finally {
    prizeSubmitting.value = false
  }
}

const deletePrize = async (prize: AdminLotteryPrize) => {
  const confirmed = await confirmAction({
    description: t('admin.lottery.confirmDeletePrize', { name: prize.name }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteLotteryPrize(Number(prize.id))
    prizeSuccess.value = t('admin.lottery.alerts.prizeDeleted')
    prizeError.value = ''
    await fetchPrizes()
  } catch (err: any) {
    prizeError.value = err?.message || t('admin.lottery.errors.prizeDeleteFailed')
  }
}

const togglePrizeEnabled = async (prize: AdminLotteryPrize, enabled: boolean) => {
  try {
    await adminAPI.updateLotteryPrize(Number(prize.id), {
      name: prize.name,
      type: prize.type,
      probability: Number(prize.probability || 0),
      stock: Number(prize.stock),
      amount: prize.amount,
      coupon_id: prize.coupon_id ?? null,
      product_id: prize.product_id ?? null,
      icon: prize.icon,
      color: prize.color,
      description: prize.description,
      sort_order: Number(prize.sort_order || 0),
      enabled,
    })
    prizeError.value = ''
    await fetchPrizes()
  } catch (err: any) {
    prizeError.value = err?.message || t('admin.lottery.errors.prizeSaveFailed')
    await fetchPrizes()
  }
}

onMounted(() => {
  fetchConfig()
  fetchPrizes()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.lottery.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.lottery.subtitle') }}</p>
      </div>
      <Button size="sm" :disabled="configSaving || configLoading" @click="saveConfig">
        {{ configSaving ? t('admin.common.submitting') : t('admin.lottery.saveConfig') }}
      </Button>
    </div>

    <!-- 规则配置 -->
    <div class="rounded-xl border border-border bg-card p-4 shadow-sm space-y-4">
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium">{{ t('admin.lottery.form.enabled') }}</p>
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.form.enabledHint') }}</p>
        </div>
        <Switch v-model="configForm.enabled" :disabled="configLoading" />
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="space-y-2 md:col-span-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.form.activityTitle') }}</label>
          <Input v-model="configForm.title" maxlength="100" :placeholder="t('admin.lottery.form.activityTitlePlaceholder')" />
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.form.minOrderAmount') }}</label>
          <Input v-model="configForm.minOrderAmount" type="text" inputmode="decimal" />
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.form.minOrderAmountHint') }}</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.form.startsAt') }}</label>
          <Input v-model="configForm.startsAt" type="datetime-local" />
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.form.startsAtHint') }}</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.form.drawsPerOrder') }}</label>
          <Input v-model.number="configForm.drawsPerOrder" type="number" min="1" max="20" />
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.form.drawsPerOrderHint') }}</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.form.dailyDrawLimit') }}</label>
          <Input v-model.number="configForm.dailyDrawLimit" type="number" min="0" max="1000" />
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.form.dailyDrawLimitHint') }}</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.form.validHours') }}</label>
          <Input v-model.number="configForm.validHours" type="number" min="0" max="8760" />
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.form.validHoursHint') }}</p>
        </div>
        <div class="space-y-2 md:col-span-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.form.notice') }}</label>
          <Textarea v-model="configForm.notice" rows="4" maxlength="2000" :placeholder="t('admin.lottery.form.noticePlaceholder')" />
        </div>
      </div>

      <div class="rounded-lg border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
        {{ t('admin.lottery.form.guestHint') }}
      </div>

      <div v-if="configError" class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {{ configError }}
      </div>
      <div v-if="configSuccess" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
        {{ configSuccess }}
      </div>
    </div>

    <!-- 奖品配置 -->
    <div class="rounded-xl border border-border bg-card shadow-sm">
      <div class="flex flex-col gap-3 border-b border-border p-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-lg font-semibold">{{ t('admin.lottery.prizes.title') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.lottery.prizes.subtitle') }}</p>
        </div>
        <Button size="sm" class="w-full sm:w-auto" @click="openPrizeModal()">{{ t('admin.lottery.prizes.add') }}</Button>
      </div>

      <div class="grid grid-cols-1 gap-3 border-b border-border p-4 sm:grid-cols-3">
        <div class="rounded-lg border border-border bg-muted/20 p-3">
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.configuredTotal') }}</p>
          <p class="mt-1 text-lg font-semibold" :class="probabilityExceeded ? 'text-destructive' : ''">{{ configuredPercent }}%</p>
        </div>
        <div class="rounded-lg border border-border bg-muted/20 p-3">
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.remainder') }}</p>
          <p class="mt-1 text-lg font-semibold">{{ remainderPercent }}%</p>
        </div>
        <div class="rounded-lg border border-border bg-muted/20 p-3">
          <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.count') }}</p>
          <p class="mt-1 text-lg font-semibold">{{ prizes.length }}</p>
        </div>
      </div>

      <div class="border-b border-border px-4 py-3 text-xs text-muted-foreground">
        {{ t('admin.lottery.prizes.remainderHint') }}
      </div>

      <div v-if="prizeError" class="mx-4 mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {{ prizeError }}
      </div>
      <div v-if="prizeSuccess" class="mx-4 mt-4 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
        {{ prizeSuccess }}
      </div>

      <div class="overflow-x-auto">
        <Table class="min-w-[960px]">
          <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
            <TableRow>
              <TableHead class="min-w-[70px] px-4 py-3">{{ t('admin.lottery.prizes.table.sort') }}</TableHead>
              <TableHead class="min-w-[160px] px-4 py-3">{{ t('admin.lottery.prizes.table.name') }}</TableHead>
              <TableHead class="min-w-[100px] px-4 py-3">{{ t('admin.lottery.prizes.table.type') }}</TableHead>
              <TableHead class="min-w-[110px] px-4 py-3">{{ t('admin.lottery.prizes.table.probability') }}</TableHead>
              <TableHead class="min-w-[110px] px-4 py-3">{{ t('admin.lottery.prizes.table.value') }}</TableHead>
              <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.lottery.prizes.table.stock') }}</TableHead>
              <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.lottery.prizes.table.awarded') }}</TableHead>
              <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.lottery.prizes.table.enabled') }}</TableHead>
              <TableHead class="min-w-[120px] px-4 py-3 text-right">{{ t('admin.lottery.prizes.table.action') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border">
            <TableRow v-if="prizesLoading">
              <TableCell :colspan="9" class="p-0">
                <TableSkeleton :columns="9" :rows="4" />
              </TableCell>
            </TableRow>
            <TableRow v-else-if="prizes.length === 0">
              <TableCell colspan="9" class="px-4 py-8 text-center text-muted-foreground">{{ t('admin.lottery.prizes.empty') }}</TableCell>
            </TableRow>
            <TableRow v-for="prize in prizes" :key="prize.id" class="hover:bg-muted/30">
              <TableCell class="px-4 py-3 text-sm text-muted-foreground">{{ prize.sort_order }}</TableCell>
              <TableCell class="px-4 py-3">
                <div class="flex items-center gap-2">
                  <span v-if="prize.color" class="inline-block h-3 w-3 shrink-0 rounded-full border border-border" :style="{ backgroundColor: prize.color }"></span>
                  <div>
                    <p class="text-sm font-medium">{{ prize.name }}</p>
                    <p v-if="prize.description" class="text-xs text-muted-foreground">{{ prize.description }}</p>
                  </div>
                </div>
              </TableCell>
              <TableCell class="px-4 py-3 text-sm">{{ prizeTypeLabel(prize.type) }}</TableCell>
              <TableCell class="px-4 py-3 text-sm">{{ toPercent(prize.probability) }}%</TableCell>
              <TableCell class="px-4 py-3 text-sm">{{ prizeValueText(prize) }}</TableCell>
              <TableCell class="px-4 py-3 text-sm">{{ stockText(prize) }}</TableCell>
              <TableCell class="px-4 py-3 text-sm">{{ prize.awarded_count }}</TableCell>
              <TableCell class="px-4 py-3">
                <Switch :model-value="prize.enabled" @update:model-value="(v) => togglePrizeEnabled(prize, Boolean(v))" />
              </TableCell>
              <TableCell class="px-4 py-3 text-right">
                <div class="flex justify-end gap-2">
                  <Button size="sm" variant="outline" @click="openPrizeModal(prize)">{{ t('admin.common.edit') }}</Button>
                  <Button size="sm" variant="outline" class="text-destructive" @click="deletePrize(prize)">{{ t('admin.common.delete') }}</Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </div>

    <Dialog v-model:open="showPrizeModal" @update:open="(value) => { if (!value) closePrizeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-2xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ editingPrize ? t('admin.lottery.prizes.editTitle') : t('admin.lottery.prizes.createTitle') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="submitPrize">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.name') }}</label>
              <Input v-model="prizeForm.name" maxlength="100" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.type') }}</label>
              <Select v-model="prizeForm.type">
                <SelectTrigger class="h-10 w-full">
                  <SelectValue :placeholder="t('admin.lottery.prizes.form.type')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="type in prizeTypes" :key="type" :value="type">{{ prizeTypeLabel(type) }}</SelectItem>
                </SelectContent>
              </Select>
              <p class="text-xs text-muted-foreground">{{ t(`admin.lottery.prizes.form.typeHint.${prizeForm.type}`) }}</p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.probability') }}</label>
              <Input v-model="prizeForm.probabilityPercent" type="text" inputmode="decimal" />
              <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.form.probabilityHint') }}</p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.sortOrder') }}</label>
              <Input v-model.number="prizeForm.sortOrder" type="number" />
            </div>

            <div class="space-y-2 md:col-span-2">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <p class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.unlimitedStock') }}</p>
                  <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.form.stockHint') }}</p>
                </div>
                <Switch v-model="prizeForm.unlimitedStock" />
              </div>
              <Input v-if="!prizeForm.unlimitedStock" v-model.number="prizeForm.stock" type="number" min="0" />
            </div>

            <div v-if="prizeForm.type === 'balance'" class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.amount') }}</label>
              <Input v-model="prizeForm.amount" type="text" inputmode="decimal" />
              <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.form.amountHint') }}</p>
            </div>
            <div v-if="prizeForm.type === 'coupon'" class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.couponID') }}</label>
              <Input v-model="prizeForm.couponID" type="number" min="1" />
              <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.form.couponHint') }}</p>
            </div>
            <div v-if="prizeForm.type === 'product'" class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.productID') }}</label>
              <Input v-model="prizeForm.productID" type="number" min="1" />
              <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.form.productHint') }}</p>
            </div>

            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.color') }}</label>
              <div class="flex h-9 items-center gap-2 rounded-md border border-input bg-background px-2">
                <input
                  type="color"
                  :value="colorSwatch"
                  class="h-6 w-8 shrink-0 cursor-pointer rounded border-0 bg-transparent p-0"
                  @input="onColorPick"
                />
                <span class="truncate text-sm" :class="prizeForm.color ? 'font-mono' : 'text-muted-foreground'">
                  {{ prizeForm.color || t('admin.lottery.prizes.form.colorAuto') }}
                </span>
                <Button
                  v-if="prizeForm.color"
                  type="button"
                  variant="ghost"
                  size="xs"
                  class="ml-auto shrink-0 text-xs"
                  @click="clearPrizeColor"
                >
                  {{ t('admin.lottery.prizes.form.colorReset') }}
                </Button>
              </div>
              <p class="text-xs text-muted-foreground">{{ t('admin.lottery.prizes.form.colorHint') }}</p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.icon') }}</label>
              <Input v-model="prizeForm.icon" maxlength="255" />
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.description') }}</label>
              <Input v-model="prizeForm.description" maxlength="255" />
            </div>
            <div class="flex items-center justify-between gap-4 md:col-span-2">
              <p class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.prizes.form.enabled') }}</p>
              <Switch v-model="prizeForm.enabled" />
            </div>
          </div>

          <div v-if="prizeFormError" class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {{ prizeFormError }}
          </div>
          <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="closePrizeModal">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" class="w-full sm:w-auto" :disabled="prizeSubmitting">
              {{ prizeSubmitting ? t('admin.common.submitting') : t('admin.common.confirm') }}
            </Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
