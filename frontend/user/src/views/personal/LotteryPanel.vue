<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Dices } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import PaginationNav from '../../components/PaginationNav.vue'
import EmptyState from '../../components/EmptyState.vue'
import { lotteryAPI, type LotteryDraw } from '../../api/lottery'

const { t } = useI18n()

const loading = ref(true)
const errorMessage = ref('')
const draws = ref<LotteryDraw[]>([])
const onlyWin = ref(false)
const pagination = ref({ page: 1, page_size: 10, total: 0, total_page: 1 })

const totalPage = computed(() => Math.max(1, Number(pagination.value.total_page || 1)))

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

const prizeLabel = (draw: LotteryDraw) => (draw.is_win ? draw.prize_name : t('lottery.missLabel'))

const prizeDetail = (draw: LotteryDraw) => {
  if (!draw.is_win) return '-'
  switch (draw.prize_type) {
    case 'balance':
      return `${draw.amount} ${draw.currency || ''}`.trim()
    case 'coupon':
      return draw.coupon_code || '-'
    case 'product':
      return draw.product_name || (draw.product_id ? `#${draw.product_id}` : '-')
    default:
      // 自定义奖品没有额外内容可展示，奖品列已经写了名字，这里不再重复。
      return '-'
  }
}

const deliveryVariant = (draw: LotteryDraw) => {
  if (draw.delivery_status === 'pending') return 'warning'
  if (draw.delivery_status === 'delivered') return 'success'
  return 'neutral'
}

const fetchDraws = async (page = 1) => {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await lotteryAPI.records({
      page,
      page_size: pagination.value.page_size,
      only_win: onlyWin.value ? 'true' : undefined,
    })
    draws.value = (response.data.data || []) as LotteryDraw[]
    pagination.value = response.data.pagination || pagination.value
  } catch (err: any) {
    errorMessage.value = err?.message || t('lottery.errors.recordsFailed')
    draws.value = []
  } finally {
    loading.value = false
  }
}

const toggleOnlyWin = () => {
  onlyWin.value = !onlyWin.value
  fetchDraws(1)
}

const changePage = (page: number) => {
  if (page < 1 || page > totalPage.value) return
  fetchDraws(page)
}

onMounted(() => {
  fetchDraws()
})
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <PanelHeading :title="t('lottery.panel.title')" :description="t('lottery.panel.subtitle')" :icon="Dices">
        <template #actions>
          <Button size="sm" :variant="onlyWin ? 'default' : 'outline'" @click="toggleOnlyWin">
            {{ onlyWin ? t('lottery.panel.showAll') : t('lottery.panel.onlyWin') }}
          </Button>
        </template>
      </PanelHeading>

      <Alert v-if="errorMessage" variant="destructive" class="mb-5">
        <AlertDescription>{{ errorMessage }}</AlertDescription>
      </Alert>

      <div v-if="loading" class="space-y-3">
        <div v-for="index in 4" :key="index" class="h-12 rounded-xl theme-skeleton"></div>
      </div>

      <EmptyState
        v-else-if="draws.length === 0"
        icon="inbox"
        :title="t('lottery.panel.empty')"
        :description="t('lottery.panel.emptyHint')"
      />

      <div v-else class="overflow-x-auto rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted/50">
              <TableHead class="px-4">{{ t('lottery.panel.table.prize') }}</TableHead>
              <TableHead class="px-4">{{ t('lottery.panel.table.detail') }}</TableHead>
              <TableHead class="px-4">{{ t('lottery.panel.table.orderNo') }}</TableHead>
              <TableHead class="px-4">{{ t('lottery.panel.table.delivery') }}</TableHead>
              <TableHead class="px-4">{{ t('lottery.panel.table.time') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="draw in draws" :key="draw.id">
              <TableCell class="px-4 text-sm font-medium text-foreground">{{ prizeLabel(draw) }}</TableCell>
              <TableCell class="px-4 font-mono text-xs text-muted-foreground break-all">{{ prizeDetail(draw) }}</TableCell>
              <TableCell class="px-4 text-xs text-muted-foreground">
                <router-link class="hover:text-primary" :to="`/orders/${draw.order_no}`">{{ draw.order_no }}</router-link>
              </TableCell>
              <TableCell class="px-4">
                <Badge v-if="draw.is_win" :variant="deliveryVariant(draw)" size="sm">
                  {{ t(`lottery.delivery.${draw.delivery_status || 'none'}`) }}
                </Badge>
                <span v-else class="text-xs text-muted-foreground">-</span>
              </TableCell>
              <TableCell class="px-4 text-xs text-muted-foreground whitespace-nowrap">{{ formatDate(draw.created_at) }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <PaginationNav
        v-if="!loading && totalPage > 1"
        class="mt-5"
        :current-page="pagination.page"
        :total-pages="totalPage"
        @change-page="changePage"
      />
    </div>
  </div>
</template>
