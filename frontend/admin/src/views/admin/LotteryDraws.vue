<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  adminAPI,
  type AdminLotteryDraw,
  type AdminLotteryDrawTotals,
} from '@/api/admin'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { formatDate } from '@/utils/format'

const { t } = useI18n()

const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const draws = ref<AdminLotteryDraw[]>([])
const totals = ref<AdminLotteryDrawTotals>({ total: 0, win_count: 0, pending: 0, won_amount: '0.00' })
const actionError = ref('')
const actionSuccess = ref('')
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const pageSizeOptions = [10, 20, 50, 100]

const filters = reactive({
  orderNo: '',
  userID: '',
  prizeType: '__all__',
  isWin: '__all__',
  deliveryStatus: '__all__',
  createdFrom: '',
  createdTo: '',
})

const showDeliverModal = ref(false)
const deliverSubmitting = ref(false)
const deliverError = ref('')
const deliverTarget = ref<AdminLotteryDraw | null>(null)
const deliverForm = reactive({ remark: '', revert: false })

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

const toISO = (raw?: string) => {
  const text = String(raw || '').trim()
  if (!text) return ''
  const date = new Date(text)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString()
}

const prizeTypeLabel = (type: string) => t(`admin.lottery.prizeTypes.${type || 'none'}`)

const deliveryLabel = (status: string) => t(`admin.lottery.draws.delivery.${status || 'none'}`)

const deliveryClass = (status: string) => {
  if (status === 'pending') return 'text-amber-700 border-amber-200 bg-amber-50'
  if (status === 'delivered') return 'text-emerald-700 border-emerald-200 bg-emerald-50'
  return 'text-muted-foreground border-border bg-muted/30'
}

const prizeValueText = (draw: AdminLotteryDraw) => {
  if (!draw.is_win) return '-'
  switch (draw.prize_type) {
    case 'balance':
      return `${draw.amount} ${draw.currency || ''}`.trim()
    case 'coupon':
      return draw.coupon_code || '-'
    case 'product':
      return draw.product_name || (draw.product_id ? `#${draw.product_id}` : '-')
    default:
      return draw.prize_name || '-'
  }
}

/** 只有商品/自定义奖品需要人工发放，其余类型在抽中时已经到账。 */
const needsDelivery = (draw: AdminLotteryDraw) =>
  draw.is_win && (draw.prize_type === 'product' || draw.prize_type === 'custom')

const fetchDraws = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  actionError.value = ''
  try {
    const params: Record<string, unknown> = {
      page,
      page_size: pagination.value.page_size,
      order_no: String(filters.orderNo || '').trim() || undefined,
      prize_type: normalizeFilterValue(filters.prizeType) || undefined,
      delivery_status: normalizeFilterValue(filters.deliveryStatus) || undefined,
      created_from: toISO(filters.createdFrom) || undefined,
      created_to: toISO(filters.createdTo) || undefined,
    }
    const isWin = normalizeFilterValue(filters.isWin)
    if (isWin) params.is_win = isWin
    const userID = Number(filters.userID)
    if (Number.isFinite(userID) && userID > 0) params.user_id = Math.floor(userID)

    const response = await adminAPI.getLotteryDraws(params)
    const payload = response.data.data || {}
    draws.value = (payload.items || []) as AdminLotteryDraw[]
    totals.value = (payload.totals || totals.value) as AdminLotteryDrawTotals
    pagination.value = response.data.pagination || pagination.value
  } catch (err: any) {
    actionError.value = err?.message || t('admin.lottery.errors.drawFetchFailed')
    if (!options.preserveRows) draws.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => fetchDraws(1, { preserveRows: true })
const refresh = () => refreshList(() => fetchDraws(pagination.value.page, { preserveRows: true }))

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchDraws(page)
}

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchDraws(1)
}

const openDeliverModal = (draw: AdminLotteryDraw, revert: boolean) => {
  deliverTarget.value = draw
  deliverForm.remark = String(draw.delivery_remark || '')
  deliverForm.revert = revert
  deliverError.value = ''
  showDeliverModal.value = true
}

const closeDeliverModal = () => {
  showDeliverModal.value = false
  deliverSubmitting.value = false
  deliverTarget.value = null
  deliverError.value = ''
}

const submitDeliver = async () => {
  if (!deliverTarget.value?.id) return
  deliverError.value = ''
  deliverSubmitting.value = true
  try {
    await adminAPI.deliverLotteryDraw(Number(deliverTarget.value.id), {
      remark: String(deliverForm.remark || '').trim(),
      revert: deliverForm.revert,
    })
    actionSuccess.value = deliverForm.revert
      ? t('admin.lottery.alerts.deliveryReverted')
      : t('admin.lottery.alerts.delivered')
    closeDeliverModal()
    await fetchDraws(pagination.value.page, { preserveRows: true })
  } catch (err: any) {
    deliverError.value = err?.message || t('admin.lottery.errors.deliverFailed')
  } finally {
    deliverSubmitting.value = false
  }
}

onMounted(() => {
  fetchDraws()
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-semibold">{{ t('admin.lottery.draws.title') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.lottery.draws.subtitle') }}</p>
    </div>

    <div class="grid grid-cols-1 gap-3 sm:grid-cols-4">
      <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <p class="text-xs text-muted-foreground">{{ t('admin.lottery.draws.stats.total') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ totals.total }}</p>
      </div>
      <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <p class="text-xs text-muted-foreground">{{ t('admin.lottery.draws.stats.win') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ totals.win_count }}</p>
      </div>
      <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <p class="text-xs text-muted-foreground">{{ t('admin.lottery.draws.stats.pending') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ totals.pending }}</p>
      </div>
      <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <p class="text-xs text-muted-foreground">{{ t('admin.lottery.draws.stats.wonAmount') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ totals.won_amount }}</p>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="grid grid-cols-1 gap-3 md:grid-cols-12">
        <div class="md:col-span-3">
          <Input v-model="filters.orderNo" :placeholder="t('admin.lottery.draws.filters.orderNo')" @keyup.enter="handleSearch" />
        </div>
        <div class="md:col-span-2">
          <Input v-model="filters.userID" type="number" min="1" :placeholder="t('admin.lottery.draws.filters.userID')" @keyup.enter="handleSearch" />
        </div>
        <div class="md:col-span-2">
          <Select v-model="filters.isWin" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.lottery.draws.filters.resultAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.lottery.draws.filters.resultAll') }}</SelectItem>
              <SelectItem value="true">{{ t('admin.lottery.draws.filters.resultWin') }}</SelectItem>
              <SelectItem value="false">{{ t('admin.lottery.draws.filters.resultMiss') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="md:col-span-2">
          <Select v-model="filters.prizeType" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.lottery.draws.filters.prizeTypeAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.lottery.draws.filters.prizeTypeAll') }}</SelectItem>
              <SelectItem value="balance">{{ prizeTypeLabel('balance') }}</SelectItem>
              <SelectItem value="coupon">{{ prizeTypeLabel('coupon') }}</SelectItem>
              <SelectItem value="product">{{ prizeTypeLabel('product') }}</SelectItem>
              <SelectItem value="custom">{{ prizeTypeLabel('custom') }}</SelectItem>
              <SelectItem value="none">{{ prizeTypeLabel('none') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="md:col-span-2">
          <Select v-model="filters.deliveryStatus" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.lottery.draws.filters.deliveryAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.lottery.draws.filters.deliveryAll') }}</SelectItem>
              <SelectItem value="pending">{{ deliveryLabel('pending') }}</SelectItem>
              <SelectItem value="delivered">{{ deliveryLabel('delivered') }}</SelectItem>
              <SelectItem value="none">{{ deliveryLabel('none') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="md:col-span-1 flex items-center justify-end">
          <Button size="sm" class="w-full md:w-auto" @click="handleSearch">{{ t('admin.common.search') }}</Button>
        </div>
        <div class="md:col-span-3">
          <Input v-model="filters.createdFrom" type="datetime-local" />
        </div>
        <div class="md:col-span-3">
          <Input v-model="filters.createdTo" type="datetime-local" />
        </div>
      </div>
    </div>

    <div v-if="actionError" class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
      {{ actionError }}
    </div>
    <div v-if="actionSuccess" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
      {{ actionSuccess }}
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[1100px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="min-w-[80px] px-4 py-3">{{ t('admin.lottery.draws.table.id') }}</TableHead>
            <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.lottery.draws.table.user') }}</TableHead>
            <TableHead class="min-w-[170px] px-4 py-3">{{ t('admin.lottery.draws.table.orderNo') }}</TableHead>
            <TableHead class="min-w-[140px] px-4 py-3">{{ t('admin.lottery.draws.table.prize') }}</TableHead>
            <TableHead class="min-w-[100px] px-4 py-3">{{ t('admin.lottery.draws.table.prizeType') }}</TableHead>
            <TableHead class="min-w-[150px] px-4 py-3">{{ t('admin.lottery.draws.table.value') }}</TableHead>
            <TableHead class="min-w-[110px] px-4 py-3">{{ t('admin.lottery.draws.table.delivery') }}</TableHead>
            <TableHead class="min-w-[150px] px-4 py-3">{{ t('admin.lottery.draws.table.createdAt') }}</TableHead>
            <TableHead class="min-w-[150px] px-4 py-3 text-right">{{ t('admin.lottery.draws.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="9" class="p-0">
              <TableSkeleton :columns="9" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="draws.length === 0">
            <TableCell colspan="9" class="px-4 py-8 text-center text-muted-foreground">{{ t('admin.lottery.draws.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="draw in draws" :key="draw.id" class="hover:bg-muted/30">
            <TableCell class="px-4 py-3"><IdCell :value="draw.id" /></TableCell>
            <TableCell class="px-4 py-3 text-sm">{{ draw.user_id ? `#${draw.user_id}` : '-' }}</TableCell>
            <TableCell class="px-4 py-3 font-mono text-xs break-all">{{ draw.order_no }}</TableCell>
            <TableCell class="px-4 py-3 text-sm">
              <span v-if="draw.is_win" class="font-medium">{{ draw.prize_name }}</span>
              <span v-else class="text-muted-foreground">{{ t('admin.lottery.draws.miss') }}</span>
            </TableCell>
            <TableCell class="px-4 py-3 text-sm">{{ draw.is_win ? prizeTypeLabel(draw.prize_type) : '-' }}</TableCell>
            <TableCell class="px-4 py-3 font-mono text-xs break-all">{{ prizeValueText(draw) }}</TableCell>
            <TableCell class="px-4 py-3">
              <span class="inline-flex items-center rounded-full border px-2 py-0.5 text-xs" :class="deliveryClass(draw.delivery_status)">
                {{ deliveryLabel(draw.delivery_status) }}
              </span>
              <p v-if="draw.delivery_remark" class="mt-1 text-xs text-muted-foreground break-words">{{ draw.delivery_remark }}</p>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatDate(draw.created_at) }}</TableCell>
            <TableCell class="px-4 py-3 text-right">
              <div v-if="needsDelivery(draw)" class="flex justify-end gap-2">
                <Button v-if="draw.delivery_status !== 'delivered'" size="sm" variant="outline" @click="openDeliverModal(draw, false)">
                  {{ t('admin.lottery.draws.markDelivered') }}
                </Button>
                <Button v-else size="sm" variant="outline" @click="openDeliverModal(draw, true)">
                  {{ t('admin.lottery.draws.revertDelivery') }}
                </Button>
              </div>
              <span v-else class="text-xs text-muted-foreground">{{ t('admin.lottery.draws.autoDelivered') }}</span>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <ListPagination
      :page="pagination.page"
      :total-page="pagination.total_page"
      :total="pagination.total"
      :page-size="pagination.page_size"
      :page-size-options="pageSizeOptions"
      @change-page="changePage"
      @change-page-size="changePageSize"
    >
      <template #actions>
        <Button size="sm" variant="outline" class="h-8" :disabled="refreshing" @click="refresh">{{ t('admin.common.refresh') }}</Button>
      </template>
    </ListPagination>

    <Dialog v-model:open="showDeliverModal" @update:open="(value) => { if (!value) closeDeliverModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>
            {{ deliverForm.revert ? t('admin.lottery.draws.revertDelivery') : t('admin.lottery.draws.markDelivered') }}
          </DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="submitDeliver">
          <div class="rounded-lg border border-border bg-muted/20 p-3 text-sm">
            <p>{{ t('admin.lottery.draws.table.orderNo') }}: <span class="font-mono text-xs">{{ deliverTarget?.order_no }}</span></p>
            <p class="mt-1">{{ t('admin.lottery.draws.table.prize') }}: {{ deliverTarget?.prize_name }}</p>
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.lottery.draws.remark') }}</label>
            <Input v-model="deliverForm.remark" maxlength="500" :placeholder="t('admin.lottery.draws.remarkPlaceholder')" />
          </div>
          <div v-if="deliverError" class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {{ deliverError }}
          </div>
          <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="closeDeliverModal">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" class="w-full sm:w-auto" :disabled="deliverSubmitting">
              {{ deliverSubmitting ? t('admin.common.submitting') : t('admin.common.confirm') }}
            </Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
