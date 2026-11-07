<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'

const { t } = useI18n()

const loading = ref(false)
const submitting = ref(false)

// 后端只返回是否已配置和末 4 位，从不返回明文 Key
const form = reactive({
  api_key: '',
  has_api_key: false,
  api_key_masked: '',
})

const applyResponse = (data: Record<string, unknown> | undefined) => {
  form.api_key = ''
  form.has_api_key = !!data?.has_api_key
  form.api_key_masked = String(data?.api_key_masked || '')
}

const loadConfig = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getTinyPNGSettings()
    applyResponse(res.data?.data as Record<string, unknown> | undefined)
  } catch {
    // 请求层已提示错误
  } finally {
    loading.value = false
  }
}

const notifyIfNeeded = (err: unknown) => {
  const known = err as Error & { __notified?: boolean }
  if (!known?.__notified) {
    notifyError(known?.message || t('admin.settings.alerts.saveFailed'))
  }
}

const save = async () => {
  const apiKey = form.api_key.trim()
  if (!apiKey) {
    notifySuccess(t('admin.settings.tinypng.nothingToSave'))
    return
  }
  submitting.value = true
  try {
    const res = await adminAPI.updateTinyPNGSettings({ api_key: apiKey })
    applyResponse(res.data?.data as Record<string, unknown> | undefined)
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
  } catch (err) {
    notifyIfNeeded(err)
  } finally {
    submitting.value = false
  }
}

const clearKey = async () => {
  const confirmed = await confirmAction({
    description: t('admin.settings.tinypng.clearConfirm'),
    variant: 'destructive',
  })
  if (!confirmed) return
  submitting.value = true
  try {
    const res = await adminAPI.updateTinyPNGSettings({ clear_api_key: true })
    applyResponse(res.data?.data as Record<string, unknown> | undefined)
    notifySuccess(t('admin.settings.tinypng.cleared'))
  } catch (err) {
    notifyIfNeeded(err)
  } finally {
    submitting.value = false
  }
}

defineExpose({ save, submitting })

onMounted(() => {
  loadConfig()
})
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-lg border p-6">
      <h2 class="text-lg font-semibold">{{ t('admin.settings.tinypng.title') }}</h2>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.tinypng.subtitle') }}</p>
    </div>

    <div class="rounded-lg border p-6 space-y-4">
      <div class="space-y-2">
        <label class="text-xs font-medium text-muted-foreground" for="tinypng-api-key">{{ t('admin.settings.tinypng.apiKey') }}</label>
        <Input
          id="tinypng-api-key"
          v-model="form.api_key"
          type="password"
          autocomplete="new-password"
          :disabled="loading || submitting"
          :placeholder="form.has_api_key ? t('admin.settings.tinypng.apiKeyPlaceholderKeep') : t('admin.settings.tinypng.apiKeyPlaceholder')"
        />
        <p class="text-xs text-muted-foreground">
          <template v-if="form.has_api_key">
            {{ t('admin.settings.tinypng.apiKeyHintKeep', { masked: form.api_key_masked }) }}
          </template>
          <template v-else>{{ t('admin.settings.tinypng.apiKeyHintEmpty') }}</template>
        </p>
      </div>
      <div v-if="form.has_api_key">
        <Button size="sm" variant="outline" :disabled="loading || submitting" @click="clearKey">
          {{ t('admin.settings.tinypng.clear') }}
        </Button>
      </div>
      <div class="rounded-md border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground space-y-1">
        <p>{{ t('admin.settings.tinypng.howTo') }}</p>
        <p>{{ t('admin.settings.tinypng.quotaHint') }}</p>
        <p>{{ t('admin.settings.tinypng.cacheHint') }}</p>
      </div>
    </div>
  </div>
</template>
