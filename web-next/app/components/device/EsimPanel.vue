<script setup lang="ts">
import { devicesService } from '~/services/devices'
import { api } from '~/stores/auth'
import { applyOptimisticActiveState } from '~/utils/deviceEsimOptimistic'
import { pickNextDownloadAid } from '~/utils/deviceEsimOverviewRefresh'
import { describeDeleteResultNotice, describeDownloadTerminalNotice, describeSpaceDelta } from '~/utils/deviceEsimOperationNotice'
import { formatEsimNotificationEvent } from '~/utils/deviceEsimNotifications'
import type {
  EsimChipInfo, EsimEUICCProfiles, EsimNotificationItem, EsimProfileItem, EsimSpaceDelta
} from '~/types/api'

const props = defineProps<{
  deviceId: string
  deviceImei?: string
  deviceOnline?: boolean
}>()

const toast = useToast()

const chipInfo = ref<EsimChipInfo | null>(null)
const groups = ref<EsimEUICCProfiles[]>([])
const notifications = ref<EsimNotificationItem[]>([])
const notificationsLoading = ref(false)
const loading = ref(false)
const busy = ref('')

/** 删除/下载后短暂展示的空间变化提示，按 eUICC 归属 */
const recentSpaceDelta = ref<{ aidHex: string; message: string } | null>(null)
let recentSpaceDeltaTimer: number | null = null

function normalizeAidHex(aidHex: string | undefined | null): string {
  return (aidHex || '').trim().toUpperCase()
}

function clearRecentSpaceDelta() {
  if (recentSpaceDeltaTimer !== null) {
    window.clearTimeout(recentSpaceDeltaTimer)
    recentSpaceDeltaTimer = null
  }
}

function showRecentSpaceDelta(aidHex: string, spaceDelta?: Partial<EsimSpaceDelta>) {
  const normalized = normalizeAidHex(aidHex)
  const message = describeSpaceDelta(spaceDelta)
  if (!normalized || !message) return
  clearRecentSpaceDelta()
  recentSpaceDelta.value = { aidHex: normalized, message }
  recentSpaceDeltaTimer = window.setTimeout(() => {
    recentSpaceDelta.value = null
    recentSpaceDeltaTimer = null
  }, 75_000)
}

onBeforeUnmount(clearRecentSpaceDelta)

async function load(refresh = false) {
  loading.value = true
  const result = await devicesService.getEsimOverview(props.deviceId, { refresh })
  if (result.ok) {
    chipInfo.value = result.data.chipInfo
    groups.value = result.data.profiles
    if (!download.value.aid_hex) download.value.aid_hex = pickNextDownloadAid(chipInfo.value, '')
  } else {
    toast.add({ title: 'eSIM 信息读取失败', description: String(result.error?.message ?? ''), color: 'error' })
  }
  loading.value = false
}

async function loadNotifications() {
  notificationsLoading.value = true
  const result = await devicesService.getEsimNotifications(props.deviceId)
  if (result.ok) notifications.value = result.data
  notificationsLoading.value = false
}

/** eSIM 操作普遍耗时且互斥，统一置忙并在结束后刷新 */
async function run(key: string, label: string, fn: () => Promise<{ ok?: boolean; error?: { message?: string } }>) {
  busy.value = key
  try {
    const result = await fn()
    toast.add(result?.ok === false
      ? { title: `${label}失败`, description: String(result?.error?.message ?? ''), color: 'error' }
      : { title: `${label}成功`, color: 'success' })
    if (result?.ok !== false) await load(true)
  } finally {
    busy.value = ''
  }
}

/* 切换 profile：切换会短暂断网，先确认 */
const switchTarget = ref<{ iccid: string; aid: string; state: number } | null>(null)
const switchAction = computed(() => (switchTarget.value?.state === 1 ? '禁用' : '启用'))

async function confirmSwitch() {
  const t = switchTarget.value
  if (!t) return
  switchTarget.value = null
  busy.value = t.iccid
  const result = await devicesService.switchEsimProfile(props.deviceId, { iccid: t.iccid, aid_hex: t.aid })
  busy.value = ''
  if (result.ok === false) {
    toast.add({ title: '切换失败', description: String(result.error?.message ?? ''), color: 'error' })
    return
  }
  toast.add({ title: 'Profile 切换成功', color: 'success' })
  // 后端刷新有延迟，先本地乐观置位，避免列表短暂显示旧的激活卡
  groups.value = applyOptimisticActiveState(groups.value, t.iccid, t.aid)
  await load(true)
}

/* 重命名 */
const renameOpen = ref(false)
const renameTarget = ref<{ iccid: string; aid: string } | null>(null)
const renameValue = ref('')

function openRename(p: EsimProfileItem, aid: string) {
  renameTarget.value = { iccid: p.iccid, aid }
  renameValue.value = p.name || ''
  renameOpen.value = true
}

async function submitRename() {
  const t = renameTarget.value
  if (!t || !renameValue.value.trim()) return
  renameOpen.value = false
  await run(t.iccid, '重命名', () =>
    devicesService.renameEsimProfile(props.deviceId, t.iccid, { name: renameValue.value.trim(), aid_hex: t.aid }))
}

/* 删除：不可逆，要求输入 ICCID 后 4 位 */
const deleteTarget = ref<{ iccid: string; aid: string; name: string } | null>(null)
const deleteConfirm = ref('')
const deleteLast4 = computed(() => String(deleteTarget.value?.iccid || '').slice(-4))
const deleteReady = computed(() => !!deleteLast4.value && deleteConfirm.value.trim() === deleteLast4.value)

function openDelete(p: EsimProfileItem, aid: string) {
  deleteTarget.value = { iccid: p.iccid, aid, name: p.name || p.iccid }
  deleteConfirm.value = ''
}

async function confirmDelete() {
  const t = deleteTarget.value
  if (!t || !deleteReady.value) return
  deleteTarget.value = null
  busy.value = t.iccid
  const result = await devicesService.deleteEsimProfile(props.deviceId, t.iccid, t.aid)
  busy.value = ''
  if (result.ok === false) {
    toast.add({ title: '删除失败', description: String(result.error?.message ?? ''), color: 'error' })
    return
  }
  showRecentSpaceDelta(t.aid, result.data?.space_delta)
  const notice = describeDeleteResultNotice(result.data ?? {})
  toast.add({ title: notice.message, color: notice.tone === 'warning' ? 'warning' : 'success' })
  await load(true)
}

/* 下载：服务端是 SSE 流式进度，必须逐事件读，不能当普通 GET 用 */
const downloadOpen = ref(false)
const downloading = ref(false)
const downloadProgress = ref(0)
const downloadMsg = ref('')
const downloadError = ref('')
const download = ref({ smdp: '', matching_id: '', confirmation_code: '', aid_hex: '', imei: '' })

watch(() => props.deviceImei, (imei) => {
  if (imei && !download.value.imei) download.value.imei = imei
}, { immediate: true })

// 粘贴完整 LPA 激活码时自动拆成 SM-DP+ 与 Matching ID；带协议头的地址去掉前缀
watch(() => download.value.smdp, (value) => {
  if (!value) return
  if (value.startsWith('LPA:')) {
    const parts = value.split('$')
    if (parts.length >= 3) {
      download.value.smdp = parts[1] ?? ''
      download.value.matching_id = parts[2] ?? ''
      toast.add({ title: '已自动解析完整的 LPA 激活码', color: 'success' })
    }
  } else if (/^https?:\/\//i.test(value)) {
    download.value.smdp = value.replace(/^https?:\/\//i, '')
  }
})

const euiccItems = computed(() =>
  (chipInfo.value?.eids ?? [])
    .filter(e => e.aid)
    .map((e, i) => ({ label: `eUICC #${i + 1} · ${e.aid}`, value: e.aid }))
)

async function startDownload() {
  const smdp = download.value.smdp.trim()
  if (!smdp) {
    toast.add({ title: '请输入 SM-DP+ 地址', color: 'warning' })
    return
  }
  const targetAid = download.value.aid_hex || pickNextDownloadAid(chipInfo.value, '')

  downloading.value = true
  downloadProgress.value = 0
  downloadMsg.value = '正在连接…'
  downloadError.value = ''

  const params = new URLSearchParams({ smdp })
  if (download.value.matching_id) params.set('matching_id', download.value.matching_id)
  if (download.value.confirmation_code) params.set('confirmation_code', download.value.confirmation_code)
  if (targetAid) params.set('aid_hex', targetAid)
  if (download.value.imei.trim()) params.set('imei', download.value.imei.trim())

  const base = api.defaults.baseURL || ''
  const token = localStorage.getItem('token') || ''

  try {
    const res = await fetch(`${base}/devices/${props.deviceId}/esim/actions/download?${params}`, {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' }
    })
    if (!res.ok) throw new Error((await res.text()) || `HTTP ${res.status}`)
    if (!res.body) throw new Error('响应没有流式内容')

    const reader = res.body.getReader()
    const decoder = new TextDecoder('utf-8')
    let buffer = ''

    outer: while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      while (true) {
        const nl = buffer.indexOf('\n')
        if (nl < 0) break
        let line = buffer.slice(0, nl)
        buffer = buffer.slice(nl + 1)
        if (line.endsWith('\r')) line = line.slice(0, -1)
        if (!line.startsWith('data:')) continue

        try {
          const evt = JSON.parse(line.slice(5).trim()) as {
            step: string; msg: string; pct: number; code?: string
            warning?: string; space_delta?: Partial<EsimSpaceDelta>
          }
          if (evt.step === 'error') {
            downloadError.value = evt.code === 'euicc_insufficient_memory'
              ? 'eUICC 安装 profile 时空间不足，请删除未使用的 profile 后重试。'
              : evt.msg
            break outer
          }
          downloadProgress.value = evt.pct
          downloadMsg.value = evt.msg
          if (evt.step === 'done') {
            showRecentSpaceDelta(targetAid, evt.space_delta)
            const notice = describeDownloadTerminalNotice(evt)
            toast.add({ title: notice.message, color: notice.tone === 'warning' ? 'warning' : 'success' })
            download.value = {
              smdp: '', matching_id: '', confirmation_code: '',
              aid_hex: targetAid, imei: download.value.imei
            }
            downloadOpen.value = false
            await load(true)
            break outer
          }
        } catch { /* 非 JSON 行（注释/心跳），忽略 */ }
      }
    }
  } catch (e: unknown) {
    if (!downloadError.value) downloadError.value = e instanceof Error ? e.message : '下载失败'
  } finally {
    downloading.value = false
  }
}

/* 每张卡的策略内嵌展开 */
const expandedPolicyIccid = ref('')
function togglePolicy(iccid: string) {
  expandedPolicyIccid.value = expandedPolicyIccid.value === iccid ? '' : iccid
}

function stateTone(state: number) {
  return state === 1 ? 'success' : 'neutral'
}

onMounted(() => {
  void load()
  void loadNotifications()
})

watch(() => props.deviceId, () => {
  chipInfo.value = null
  groups.value = []
  expandedPolicyIccid.value = ''
  void load()
  void loadNotifications()
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold">
          eSIM 配置文件
        </h2>
        <p class="text-xs text-muted mt-0.5">
          下载、切换、重命名与删除 eUICC 上的配置文件
        </p>
      </div>
      <div class="flex gap-2">
        <UButton
          icon="i-lucide-bell"
          color="neutral"
          variant="outline"
          size="sm"
          :loading="notificationsLoading"
          :label="notifications.length ? `通知 ${notifications.length}` : '通知'"
          @click="loadNotifications"
        />
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          size="sm"
          :loading="loading"
          label="重新读取"
          @click="load(true)"
        />
        <UButton icon="i-lucide-download" size="sm" label="下载配置文件" @click="downloadOpen = true" />
      </div>
    </div>

    <!-- 芯片信息 -->
    <div v-if="chipInfo?.eids?.length" class="tile divide-y divide-default">
      <div v-for="e in chipInfo.eids" :key="e.eid" class="px-4 py-3">
        <div class="grid gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="row in [
              { k: 'EID', v: e.eid },
              { k: '剩余空间', v: e.free_nvram },
              { k: '制造商', v: e.manufacturer },
              { k: '固件', v: e.firmware },
              { k: 'AID', v: e.aid }
            ].filter(r => r.v)"
            :key="row.k"
            class="flex min-w-0 flex-col gap-1"
          >
            <dt class="text-xs uppercase tracking-wide text-dimmed">
              {{ row.k }}
            </dt>
            <dd class="break-all font-mono text-sm">
              {{ row.v }}
            </dd>
          </div>
        </div>
        <p
          v-if="recentSpaceDelta && recentSpaceDelta.aidHex === (e.aid || '').toUpperCase()"
          class="mt-2 text-xs text-success"
        >
          {{ recentSpaceDelta.message }}
        </p>
      </div>
    </div>

    <UAlert
      v-else-if="!loading"
      color="warning"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="未检测到 eUICC"
      description="此 SIM 卡可能不支持 eUICC 功能。"
    />

    <!-- 配置文件（按 eUICC 分组） -->
    <div v-if="loading && !groups.length" class="flex flex-col gap-2">
      <USkeleton v-for="i in 3" :key="i" class="h-16 w-full rounded-lg" />
    </div>

    <div v-else-if="!groups.some(g => g.profiles.length)" class="tile">
      <EmptyState
        icon="i-lucide-credit-card"
        title="暂无 Profile"
        description="点击右上角下载一个 eSIM 配置文件"
      />
    </div>

    <template v-for="g in groups" v-else :key="g.aid_hex">
      <section v-if="g.profiles.length">
        <p v-if="euiccItems.length > 1" class="mb-2 font-mono text-xs text-dimmed">
          AID {{ g.aid_hex }}
        </p>
        <div class="tile divide-y divide-default">
          <div v-for="p in g.profiles" :key="p.iccid">
            <div class="flex flex-wrap items-center gap-3 px-4 py-3">
              <StatusDot :tone="stateTone(p.state)" :pulse="p.state === 1" />
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="truncate font-medium">{{ p.name || p.service_provider_name || p.iccid }}</span>
                  <UBadge v-if="p.state === 1" color="success" variant="subtle" size="sm" label="使用中" />
                  <UBadge v-if="p.class_text" color="neutral" variant="outline" size="sm" :label="p.class_text" />
                </div>
                <p class="mt-0.5 truncate font-mono text-xs text-muted">
                  {{ p.iccid }}
                </p>
              </div>

              <div class="flex shrink-0 items-center gap-1">
                <UButton
                  size="xs"
                  :icon="p.state === 1 ? 'i-lucide-toggle-right' : 'i-lucide-toggle-left'"
                  variant="soft"
                  :label="p.state === 1 ? '禁用' : '启用'"
                  :loading="busy === p.iccid"
                  @click="switchTarget = { iccid: p.iccid, aid: g.aid_hex, state: p.state }"
                />
                <UButton
                  size="xs"
                  icon="i-lucide-sliders-horizontal"
                  color="neutral"
                  :variant="expandedPolicyIccid === p.iccid ? 'soft' : 'ghost'"
                  aria-label="卡策略"
                  @click="togglePolicy(p.iccid)"
                />
                <UButton
                  size="xs"
                  icon="i-lucide-pencil"
                  color="neutral"
                  variant="ghost"
                  aria-label="重命名"
                  @click="openRename(p, g.aid_hex)"
                />
                <UButton
                  size="xs"
                  icon="i-lucide-trash-2"
                  color="error"
                  variant="ghost"
                  aria-label="删除"
                  :disabled="p.state === 1"
                  @click="openDelete(p, g.aid_hex)"
                />
              </div>
            </div>

            <div v-if="expandedPolicyIccid === p.iccid" class="px-4 pb-3">
              <DeviceEsimCardPolicyInline
                :device-id="deviceId"
                :iccid="p.iccid"
                :is-active-card="p.state === 1"
                :device-online="deviceOnline === true"
              />
            </div>
          </div>
        </div>
      </section>
    </template>

    <!-- 待处理通知 -->
    <section v-if="notifications.length">
      <h2 class="mb-3 text-sm font-semibold">
        待发送通知
      </h2>
      <div class="tile divide-y divide-default">
        <div v-for="n in notifications" :key="n.sequence_number" class="flex items-center gap-3 px-4 py-3">
          <UBadge color="neutral" variant="subtle" size="sm" :label="`#${n.sequence_number}`" />
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm">
              {{ formatEsimNotificationEvent(n.event) }}
            </p>
            <p v-if="n.iccid" class="mt-0.5 truncate font-mono text-xs text-muted">
              {{ n.iccid }}
            </p>
          </div>
          <UButton
            v-if="n.can_retry"
            size="xs"
            icon="i-lucide-send"
            variant="soft"
            label="重试"
            :loading="busy === `n${n.sequence_number}`"
            @click="run(`n${n.sequence_number}`, '通知重试',
                        () => devicesService.retryEsimNotification(deviceId, n.sequence_number, n.aid_hex))"
          />
        </div>
      </div>
    </section>

    <!-- 下载 -->
    <UModal v-model:open="downloadOpen" title="下载 eSIM 配置文件" :dismissible="!downloading">
      <template #body>
        <div class="flex flex-col gap-4">
          <UFormField label="SM-DP+ 地址" help="可直接粘贴完整 LPA 激活码，会自动拆分" required>
            <UInput v-model="download.smdp" placeholder="例如 rsp.truphone.com" :disabled="downloading" class="w-full" />
          </UFormField>
          <UFormField label="激活码 (Matching ID)">
            <UInput v-model="download.matching_id" :disabled="downloading" class="w-full font-mono" />
          </UFormField>
          <UFormField label="确认码" help="运营商要求时才需要填写">
            <UInput v-model="download.confirmation_code" :disabled="downloading" class="w-full" />
          </UFormField>
          <UFormField v-if="euiccItems.length > 1" label="目标 eUICC">
            <USelect
              v-model="download.aid_hex"
              :items="euiccItems"
              value-key="value"
              :disabled="downloading"
              class="w-full"
            />
          </UFormField>
          <UFormField label="IMEI" help="部分 SM-DP+ 会校验设备 IMEI">
            <UInput v-model="download.imei" :disabled="downloading" class="w-full font-mono" />
          </UFormField>

          <div v-if="downloading || downloadProgress > 0" class="flex flex-col gap-1.5">
            <UProgress :model-value="downloadProgress" :max="100" />
            <p class="text-xs text-muted">
              {{ downloadMsg }} · {{ downloadProgress }}%
            </p>
          </div>

          <UAlert
            v-if="downloadError"
            color="error"
            variant="subtle"
            icon="i-lucide-triangle-alert"
            title="下载失败"
            :description="downloadError"
          />
          <UAlert
            v-else
            color="warning"
            variant="subtle"
            icon="i-lucide-clock"
            title="下载耗时较长"
            description="过程中请勿断电或拔出模组，完成后配置文件列表会自动刷新。"
          />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="downloading" @click="downloadOpen = false" />
          <UButton
            label="开始下载"
            :loading="downloading"
            :disabled="!download.smdp.trim()"
            @click="startDownload"
          />
        </div>
      </template>
    </UModal>

    <!-- 切换确认 -->
    <UModal
      :open="!!switchTarget"
      :title="`${switchAction} Profile`"
      @update:open="v => { if (!v) switchTarget = null }"
    >
      <template #body>
        <p class="text-sm">
          确定要{{ switchAction }}此 Profile
          <span class="font-mono">{{ switchTarget?.iccid }}</span> 吗？
        </p>
        <p class="mt-2 text-sm text-muted">
          切换后设备会短暂断网。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" @click="switchTarget = null" />
          <UButton color="warning" :label="switchAction" @click="confirmSwitch" />
        </div>
      </template>
    </UModal>

    <!-- 重命名 -->
    <UModal v-model:open="renameOpen" title="重命名配置文件">
      <template #body>
        <UFormField label="名称">
          <UInput v-model="renameValue" class="w-full" autofocus @keydown.enter="submitRename" />
        </UFormField>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" @click="renameOpen = false" />
          <UButton label="保存名称" :disabled="!renameValue.trim()" @click="submitRename" />
        </div>
      </template>
    </UModal>

    <!-- 删除确认：不可逆，要求输入 ICCID 后 4 位 -->
    <UModal
      :open="!!deleteTarget"
      title="删除 Profile"
      @update:open="v => { if (!v) deleteTarget = null }"
    >
      <template #body>
        <div class="flex flex-col gap-3">
          <UAlert
            color="error"
            variant="subtle"
            icon="i-lucide-triangle-alert"
            title="此操作不可逆"
            :description="`删除后无法恢复，需要重新从运营商下载配置文件。`"
          />
          <p class="text-sm">
            请输入 ICCID 后 4 位
            <span class="font-mono font-semibold">{{ deleteLast4 }}</span>
            以确认删除 Profile「{{ deleteTarget?.name }}」
          </p>
          <UInput
            v-model="deleteConfirm"
            :placeholder="`输入 ${deleteLast4}`"
            class="w-full font-mono"
            autofocus
            @keydown.enter="confirmDelete"
          />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" @click="deleteTarget = null" />
          <UButton color="error" label="确认删除" :disabled="!deleteReady" @click="confirmDelete" />
        </div>
      </template>
    </UModal>
  </div>
</template>
