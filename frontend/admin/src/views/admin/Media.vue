<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminMedia, MediaCompressResult } from '@/api/types'
import type { NotifiedError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import ListPagination from '@/components/ListPagination.vue'
import { getImageUrl } from '@/utils/image'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'
import FileInput from '@/components/FileInput.vue'
import { DEFAULT_UPLOAD_MAX_SIZE_BYTES, formatFileSizeLimit, splitFilesBySize } from '@/utils/upload'

const { t } = useI18n()
const loading = ref(false)
const uploading = ref(false)
const uploadProgress = ref({ current: 0, total: 0 })
const batchMode = ref(false)
const batchOperating = ref(false)
const selectedIds = ref<Set<number>>(new Set())
const compressingIds = ref<Set<number>>(new Set())
const replacingIds = ref<Set<number>>(new Set())
const replaceInput = ref<HTMLInputElement | null>(null)
const replaceTarget = ref<AdminMedia | null>(null)
// 文件原地替换后 URL 不变，用版本号让缩略图绕过浏览器缓存
const imageVersions = ref<Record<number, number>>({})
const batchCompressing = ref(false)
const batchCompressProgress = ref({ current: 0, total: 0 })
const batchCompressSummary = ref<{ success: number; failed: number; skipped: number; before: number; after: number; stopped: string } | null>(null)

const items = ref<AdminMedia[]>([])
const pagination = reactive({
  page: 1,
  page_size: 24,
  total: 0,
  total_page: 0,
})
const filters = reactive({
  search: '',
  scene: '__all__',
})

const allSelected = computed(() => items.value.length > 0 && items.value.every((item) => selectedIds.value.has(item.id)))

// Inline rename state
const editingId = ref<number | null>(null)
const editingName = ref('')

const sceneOptions = [
  { label: () => t('admin.media.filters.sceneAll'), value: '__all__' },
  { label: () => t('admin.media.scenes.product'), value: 'product' },
  { label: () => t('admin.media.scenes.banner'), value: 'banner' },
  { label: () => t('admin.media.scenes.category'), value: 'category' },
  { label: () => t('admin.media.scenes.post'), value: 'post' },
  { label: () => t('admin.media.scenes.editor'), value: 'editor' },
  { label: () => t('admin.media.scenes.common'), value: 'common' },
  { label: () => t('admin.media.scenes.telegram'), value: 'telegram' },
  { label: () => t('admin.media.scenes.upstream'), value: 'upstream' },
]

async function fetchMedia(page = 1) {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page,
      page_size: pagination.page_size,
    }
    if (filters.search) params.search = filters.search
    if (filters.scene !== '__all__') params.scene = filters.scene

    const res = await adminAPI.getMedia(params)
    items.value = (res.data.data as any)?.items || []
    const visibleIds = new Set(items.value.map((item) => item.id))
    selectedIds.value = new Set(Array.from(selectedIds.value).filter((id) => visibleIds.has(id)))
    const total = (res.data.data as any)?.total || 0
    pagination.total = total
    pagination.page = page
    pagination.total_page = Math.ceil(total / pagination.page_size)
  } catch {
    // silent on fetch error
  } finally {
    loading.value = false
  }
}

function clearSelection() {
  selectedIds.value = new Set()
}

function toggleBatchMode() {
  batchMode.value = !batchMode.value
  editingId.value = null
  batchCompressSummary.value = null
  if (!batchMode.value) {
    clearSelection()
  }
}

function toggleSelect(id: number) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  selectedIds.value = next
}

function toggleSelectAll() {
  selectedIds.value = allSelected.value
    ? new Set()
    : new Set(items.value.map((item) => item.id))
}

function changePage(page: number) {
  if (page < 1 || page > pagination.total_page) return
  fetchMedia(page)
}

const pageSizeOptions = [10, 20, 50, 100]

function changePageSize(size: number) {
  if (size === pagination.page_size) return
  pagination.page_size = size
  fetchMedia(1)
}

const debouncedFetch = useDebounceFn(() => fetchMedia(1), 300)
watch(() => filters.search, debouncedFetch)
watch(() => filters.scene, () => fetchMedia(1))

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

// Compress (TinyPNG)
const COMPRESSIBLE_EXTENSIONS = ['png', 'jpg', 'jpeg', 'webp']
// 这些错误对后续图片同样会失败，批量压缩时遇到就停止
const FATAL_COMPRESS_ERRORS = ['tinypng_key_missing', 'tinypng_unauthorized', 'tinypng_quota_exceeded']

function isCompressible(item: AdminMedia): boolean {
  const path = item.path.split('?')[0] || ''
  const ext = path.split('.').pop()?.toLowerCase() || ''
  return COMPRESSIBLE_EXTENSIONS.includes(ext)
}

function isCompressed(item: AdminMedia): boolean {
  return !!item.compressed_at
}

function savedPercent(before: number, after: number): string {
  if (!before || after >= before) return '0'
  return (((before - after) / before) * 100).toFixed(1)
}

// 压缩或替换后文件原地覆盖、URL 不变，加上版本参数避免浏览器继续显示旧缓存
function mediaImageSrc(item: AdminMedia): string {
  const url = getImageUrl(item.path)
  const version = imageVersions.value[item.id] || (item.compressed_at ? Date.parse(item.compressed_at) : NaN)
  if (!version || Number.isNaN(version)) return url
  return `${url}${url.includes('?') ? '&' : '?'}v=${version}`
}

function isBusy(item: AdminMedia): boolean {
  return compressingIds.value.has(item.id) || replacingIds.value.has(item.id)
}

function setCompressing(id: number, active: boolean) {
  const next = new Set(compressingIds.value)
  if (active) next.add(id)
  else next.delete(id)
  compressingIds.value = next
}

// Replace file
function fileExtension(name: string): string {
  const ext = (name.split('?')[0] || '').split('.').pop()?.toLowerCase() || ''
  return ext === 'jpeg' ? 'jpg' : ext
}

function replaceAccept(item: AdminMedia): string {
  const ext = fileExtension(item.path)
  if (!ext) return 'image/*'
  return ext === 'jpg' ? '.jpg,.jpeg' : `.${ext}`
}

function startReplace(item: AdminMedia) {
  if (isBusy(item) || !replaceInput.value) return
  replaceTarget.value = item
  replaceInput.value.accept = replaceAccept(item)
  replaceInput.value.value = ''
  replaceInput.value.click()
}

async function handleReplaceFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const item = replaceTarget.value
  input.value = ''
  replaceTarget.value = null
  if (!file || !item) return

  // 地址不变，所以新文件必须和原文件同一格式；后端也会再校验一次
  const targetExt = fileExtension(item.path)
  const newExt = fileExtension(file.name)
  if (targetExt && newExt !== targetExt) {
    notifyError(t('admin.media.replace.formatMismatch', { expected: `.${targetExt}`, actual: newExt ? `.${newExt}` : '-' }))
    return
  }
  if (splitFilesBySize([file]).rejected.length > 0) {
    notifyError(t('admin.media.replace.tooLarge', { max: formatFileSizeLimit(DEFAULT_UPLOAD_MAX_SIZE_BYTES) }))
    return
  }

  const next = new Set(replacingIds.value)
  next.add(item.id)
  replacingIds.value = next
  try {
    const res = await adminAPI.replaceMediaFile(item.id, file)
    const updated = res.data.data as AdminMedia | null
    if (updated) {
      item.size = updated.size
      item.width = updated.width
      item.height = updated.height
      item.mime_type = updated.mime_type
      item.original_size = updated.original_size
      item.compressed_at = updated.compressed_at
    }
    imageVersions.value = { ...imageVersions.value, [item.id]: Date.now() }
    notifySuccess(t('admin.media.replace.success'))
  } catch {
    // 错误提示已由请求层统一弹出
  } finally {
    const rest = new Set(replacingIds.value)
    rest.delete(item.id)
    replacingIds.value = rest
  }
}

function applyCompressResult(item: AdminMedia, result: MediaCompressResult) {
  item.size = result.media?.size ?? result.after_size
  item.original_size = result.media?.original_size ?? item.original_size
  item.compressed_at = result.media?.compressed_at ?? new Date().toISOString()
}

function compressErrorCode(err: unknown): string {
  const data = (err as NotifiedError | undefined)?.data as { error_code?: string } | undefined
  return data?.error_code || ''
}

async function compressItem(item: AdminMedia): Promise<MediaCompressResult> {
  setCompressing(item.id, true)
  try {
    const res = await adminAPI.compressMedia(item.id)
    const result = res.data.data as MediaCompressResult
    applyCompressResult(item, result)
    return result
  } finally {
    setCompressing(item.id, false)
  }
}

async function handleCompress(item: AdminMedia) {
  if (isBusy(item)) return
  if (isCompressed(item)) {
    const confirmed = await confirmAction({
      title: t('admin.media.compress.recompressTitle'),
      description: t('admin.media.compress.recompressWarning'),
    })
    if (!confirmed) return
  }
  try {
    const result = await compressItem(item)
    if (result.replaced) {
      notifySuccess(t('admin.media.compress.success', {
        before: formatFileSize(result.before_size),
        after: formatFileSize(result.after_size),
        percent: result.saved_percent.toFixed(1),
      }))
    } else {
      notifySuccess(t('admin.media.compress.alreadyOptimal'))
    }
  } catch {
    // 错误提示已由请求层统一弹出
  }
}

async function handleBatchCompress() {
  const selected = items.value.filter((item) => selectedIds.value.has(item.id))
  const candidates = selected.filter((item) => isCompressible(item) && !isCompressed(item))
  const skipped = selected.length - candidates.length
  if (candidates.length === 0) {
    notifyError(t('admin.media.compress.batchNothing'))
    return
  }
  const confirmed = await confirmAction({
    description: t('admin.media.compress.batchConfirm', { count: candidates.length, skipped }),
    confirmText: t('admin.media.compress.action'),
  })
  if (!confirmed) return

  batchCompressing.value = true
  batchCompressProgress.value = { current: 0, total: candidates.length }
  const summary = { success: 0, failed: 0, skipped, before: 0, after: 0, stopped: '' }
  batchCompressSummary.value = null
  // 逐个压缩：单个请求耗时可控，不会撞上反向代理（如 Cloudflare）的超时
  for (const item of candidates) {
    try {
      const result = await compressItem(item)
      summary.success++
      summary.before += result.before_size
      summary.after += result.after_size
    } catch (err) {
      summary.failed++
      const code = compressErrorCode(err)
      if (FATAL_COMPRESS_ERRORS.includes(code)) {
        summary.stopped = (err as Error)?.message || code
        break
      }
    } finally {
      batchCompressProgress.value.current++
    }
  }
  batchCompressSummary.value = summary
  batchCompressing.value = false
  if (summary.success > 0) {
    notifySuccess(t('admin.media.compress.batchResult', {
      success: summary.success,
      total: candidates.length,
      saved: formatFileSize(summary.before - summary.after),
      percent: savedPercent(summary.before, summary.after),
    }))
  }
}

// Upload
async function handleFileChange(files: FileList | null) {
  if (!files || files.length === 0) return
  const { accepted: fileList, rejected } = splitFilesBySize(Array.from(files))
  if (rejected.length > 0) {
    notifyError(t('admin.media.errors.fileTooLarge', {
      count: rejected.length,
      max: formatFileSizeLimit(DEFAULT_UPLOAD_MAX_SIZE_BYTES),
    }))
  }
  if (fileList.length === 0) return

  uploading.value = true
  const total = fileList.length
  uploadProgress.value = { current: 0, total }
  let failCount = 0

  for (let i = 0; i < fileList.length; i += 3) {
    const batch = fileList.slice(i, i + 3)
    await Promise.allSettled(batch.map(async (file) => {
      const formData = new FormData()
      formData.append('file', file)
      try {
        await adminAPI.upload(formData, 'common')
      } catch {
        failCount++
      } finally {
        uploadProgress.value.current++
      }
    }))
  }

  if (failCount > 0) {
    notifyError(failCount === total
      ? t('admin.media.errors.uploadFailed', { message: `${total}` })
      : t('admin.media.errors.uploadPartialFailed', { fail: failCount, total }))
  }
  fetchMedia(1)
  uploading.value = false
}

// Inline rename
function startRename(item: AdminMedia) {
  editingId.value = item.id
  editingName.value = item.name
}

async function saveRename(item: AdminMedia) {
  const name = editingName.value.trim()
  if (!name || name === item.name) {
    editingId.value = null
    return
  }
  try {
    await adminAPI.updateMedia(item.id, { name })
    item.name = name
    notifySuccess(t('admin.media.card.renameSaved'))
  } catch (err: any) {
    notifyError(t('admin.media.errors.renameFailed', { message: err?.message || '' }))
  } finally {
    editingId.value = null
  }
}

function handleRenameKeydown(e: KeyboardEvent, item: AdminMedia) {
  if (e.key === 'Enter') {
    e.preventDefault()
    saveRename(item)
  } else if (e.key === 'Escape') {
    editingId.value = null
  }
}

// Delete
async function handleDelete(item: AdminMedia) {
  const confirmed = await confirmAction({
    title: t('admin.media.confirmDelete', { name: item.name }),
    description: t('admin.media.deleteWarning'),
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteMedia(item.id)
    fetchMedia(pagination.page)
  } catch (err: any) {
    notifyError(t('admin.media.errors.deleteFailed', { message: err?.message || '' }))
  }
}

async function handleBatchDelete() {
  const ids = Array.from(selectedIds.value)
  if (ids.length === 0) return
  const confirmed = await confirmAction({
    description: t('admin.media.batch.deleteConfirm', { count: ids.length }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return

  batchOperating.value = true
  try {
    const res = await adminAPI.batchDeleteMedia(ids)
    const data = res.data.data as { success_count?: number } | null
    const successCount = data?.success_count || 0
    notifySuccess(t('admin.media.batch.deleteResult', { success: successCount, total: ids.length }))
    clearSelection()
    const nextPage = items.value.length <= successCount && pagination.page > 1 ? pagination.page - 1 : pagination.page
    await fetchMedia(nextPage)
  } catch (err: any) {
    notifyError(t('admin.media.errors.deleteFailed', { message: err?.message || '' }))
  } finally {
    batchOperating.value = false
  }
}

onMounted(() => fetchMedia(1))
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-wrap items-center justify-between gap-4">
      <h1 class="text-2xl font-bold">{{ t('admin.media.title') }}</h1>
      <div class="flex flex-wrap items-center gap-2">
        <Button size="sm" :variant="batchMode ? 'secondary' : 'outline'" :disabled="loading || uploading || batchCompressing" @click="toggleBatchMode">
          {{ batchMode ? t('admin.media.batch.exit') : t('admin.media.batch.mode') }}
        </Button>
        <FileInput
          accept="image/*"
          :multiple="true"
          :disabled="uploading"
          :button-text="uploading ? t('admin.media.uploadProgress', uploadProgress) : t('admin.media.uploadNew')"
          @change="handleFileChange"
        />
      </div>
    </div>

    <!-- 替换素材用的隐藏文件选择框 -->
    <input ref="replaceInput" type="file" class="hidden" @change="handleReplaceFileChange" />

    <!-- Filters -->
    <div class="flex flex-wrap items-center gap-3">
      <Input
        v-model="filters.search"
        :placeholder="t('admin.media.searchPlaceholder')"
        class="w-64"
      />
      <Select v-model="filters.scene">
        <SelectTrigger class="w-40">
          <SelectValue :placeholder="t('admin.media.filters.scenePlaceholder')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="opt in sceneOptions" :key="opt.value" :value="opt.value">
            {{ opt.label() }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <div v-if="batchMode" class="flex flex-wrap items-center gap-3 rounded-lg border border-primary/20 bg-primary/5 px-4 py-3">
      <span class="text-sm font-medium">{{ t('admin.media.batch.selected', { count: selectedIds.size }) }}</span>
      <Button size="sm" variant="outline" :disabled="items.length === 0 || batchOperating || batchCompressing" @click="toggleSelectAll">
        {{ allSelected ? t('admin.media.batch.deselectPage') : t('admin.media.batch.selectPage') }}
      </Button>
      <Button size="sm" variant="outline" :disabled="selectedIds.size === 0 || batchOperating || batchCompressing" @click="handleBatchCompress">
        {{ batchCompressing ? t('admin.media.compress.batchProgress', batchCompressProgress) : t('admin.media.compress.batchAction') }}
      </Button>
      <Button size="sm" variant="destructive" :disabled="selectedIds.size === 0 || batchOperating || batchCompressing" @click="handleBatchDelete">
        {{ t('admin.media.batch.delete') }}
      </Button>
      <button class="ml-auto text-xs text-muted-foreground hover:text-foreground" :disabled="batchOperating || batchCompressing" @click="clearSelection">
        {{ t('admin.media.batch.clearSelection') }}
      </button>
      <div v-if="batchCompressSummary" class="w-full space-y-1 text-xs">
        <p>
          {{ t('admin.media.compress.batchSummary', {
            success: batchCompressSummary.success,
            failed: batchCompressSummary.failed,
            skipped: batchCompressSummary.skipped,
          }) }}
          <span v-if="batchCompressSummary.success > 0" class="font-medium text-emerald-600 dark:text-emerald-400">
            {{ t('admin.media.compress.batchSaved', {
              before: formatFileSize(batchCompressSummary.before),
              after: formatFileSize(batchCompressSummary.after),
              percent: savedPercent(batchCompressSummary.before, batchCompressSummary.after),
            }) }}
          </span>
        </p>
        <p v-if="batchCompressSummary.stopped" class="text-destructive">
          {{ t('admin.media.compress.batchStopped', { reason: batchCompressSummary.stopped }) }}
        </p>
      </div>
    </div>

    <!-- Grid -->
    <div v-if="loading" class="flex items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
    </div>
    <div v-else-if="items.length === 0" class="flex flex-col items-center justify-center py-20 text-muted-foreground">
      <p>{{ t('admin.media.empty') }}</p>
    </div>
    <div v-else class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
      <div
        v-for="item in items"
        :key="item.id"
        class="group relative overflow-hidden rounded-lg border bg-card transition-colors"
        :class="selectedIds.has(item.id) ? 'border-primary ring-2 ring-primary/20' : 'border-border'"
      >
        <!-- Thumbnail -->
        <div class="relative aspect-square overflow-hidden bg-muted">
          <img
            :src="mediaImageSrc(item)"
            :alt="item.name"
            class="h-full w-full object-contain"
            loading="lazy"
          />
          <button
            v-if="batchMode"
            type="button"
            class="absolute inset-0 z-10 cursor-pointer"
            :aria-label="t('admin.media.batch.toggleItem', { name: item.name })"
            :aria-pressed="selectedIds.has(item.id)"
            @click="toggleSelect(item.id)"
          ></button>
          <div v-if="batchMode" class="absolute left-2 top-2 z-20 rounded bg-background/90 p-1 shadow-sm">
            <Checkbox :model-value="selectedIds.has(item.id)" @update:model-value="() => toggleSelect(item.id)" />
          </div>
          <!-- Compressing / replacing indicator -->
          <div v-if="isBusy(item)" class="absolute inset-0 z-30 flex flex-col items-center justify-center gap-2 bg-black/50 text-xs text-white">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-white border-t-transparent"></div>
            {{ replacingIds.has(item.id) ? t('admin.media.replace.uploading') : t('admin.media.compress.compressing') }}
          </div>
          <!-- Hover overlay with compress / replace / delete -->
          <div v-else-if="!batchMode" class="absolute inset-0 flex flex-wrap items-center justify-center gap-2 bg-black/40 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
            <Button v-if="isCompressible(item)" size="sm" variant="secondary" @click="handleCompress(item)">
              {{ t('admin.media.compress.action') }}
            </Button>
            <Button size="sm" variant="secondary" :title="t('admin.media.replace.hint')" @click="startReplace(item)">
              {{ t('admin.media.replace.action') }}
            </Button>
            <Button size="sm" variant="destructive" @click="handleDelete(item)">
              {{ t('admin.common.delete') }}
            </Button>
          </div>
        </div>
        <!-- Info -->
        <div class="space-y-1 p-2">
          <!-- Editable name -->
          <div v-if="editingId === item.id" class="flex">
            <Input
              v-model="editingName"
              class="h-7 text-xs"
              autofocus
              @blur="saveRename(item)"
              @keydown="handleRenameKeydown($event, item)"
            />
          </div>
          <p
            v-else
            class="cursor-pointer truncate text-xs font-medium hover:text-primary"
            :title="t('admin.media.card.editName')"
            @click="startRename(item)"
          >
            {{ item.name }}
          </p>
          <p class="truncate text-[10px] text-muted-foreground">
            {{ formatFileSize(item.size) }}
            <span v-if="item.width && item.height"> &middot; {{ item.width }} x {{ item.height }}</span>
          </p>
          <p
            v-if="isCompressed(item)"
            class="truncate text-[10px] text-emerald-600 dark:text-emerald-400"
            :title="t('admin.media.compress.savedTitle', {
              before: formatFileSize(item.original_size || item.size),
              after: formatFileSize(item.size),
            })"
          >
            <template v-if="item.original_size && item.original_size > item.size">
              {{ t('admin.media.compress.savedBadge', {
                percent: savedPercent(item.original_size, item.size),
                before: formatFileSize(item.original_size),
              }) }}
            </template>
            <template v-else>{{ t('admin.media.compress.optimalBadge') }}</template>
          </p>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <ListPagination
      :page="pagination.page"
      :total-page="pagination.total_page"
      :total="pagination.total"
      :page-size="pagination.page_size"
      :page-size-options="pageSizeOptions"
      @change-page="changePage"
      @change-page-size="changePageSize"
    />
  </div>
</template>
