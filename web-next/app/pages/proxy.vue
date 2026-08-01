<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useProxyStore } from '~/stores/proxy'
import { useUpstreamProxyStore } from '~/stores/upstream-proxy'
import { usePollingScheduler } from '~/composables/usePollingScheduler'
import type { ProxyInstance, UpstreamProxy } from '~/types/api'

const proxy = useProxyStore()
const upstream = useUpstreamProxyStore()
const toast = useToast()

const { instances, devices, statusMap, loading, error } = storeToRefs(proxy)
const { proxies, countries, countryRules, loading: upstreamLoading } = storeToRefs(upstream)

const tab = ref('instances')
const tabs = [
  { label: '代理实例', value: 'instances', icon: 'i-lucide-server' },
  { label: '前置代理', value: 'upstream', icon: 'i-lucide-network' }
]

/* ---------- 代理实例 ---------- */

const editing = ref<ProxyInstance | null>(null)
const editOpen = ref(false)

const modes = [
  { label: 'SOCKS5', value: 'socks5' },
  { label: 'HTTP', value: 'http' }
]

const deviceItems = computed(() =>
  devices.value.map(d => ({ label: `${d.name || d.id}（${d.interface || '—'}）`, value: d.id }))
)

function newInstance() {
  editing.value = {
    id: '',
    name: '',
    device_id: devices.value[0]?.id ?? '',
    enabled: true,
    mode: 'socks5',
    listen_addr: '0.0.0.0',
    listen_port: 1080,
    auth_enabled: false,
    username: '',
    password: ''
  } as ProxyInstance
  editOpen.value = true
}

async function editInstance(item: ProxyInstance) {
  // 概览接口返回的是精简字段，直接拿来编辑会把未返回的项（鉴权等）覆盖掉
  editing.value = { ...item }
  editOpen.value = true
  const result = await proxy.fetchInstance(item.id)
  if (result.ok) editing.value = { ...result.data, mode: result.data.mode || 'socks5' }
  else toast.add({ title: '读取完整实例配置失败，已使用概览数据', color: 'warning' })
}

async function restartInstance(item: ProxyInstance) {
  const result = await proxy.restartInstance(item.id)
  toast.add(result.ok === false
    ? { title: '重启失败', description: String(result.error?.message ?? ''), color: 'error' }
    : { title: '已重启', color: 'success' })
  await proxy.fetchOverview()
}

async function saveInstance() {
  if (!editing.value) return
  const draft = editing.value
  const next = draft.id
    ? instances.value.map(i => (i.id === draft.id ? draft : i))
    : [...instances.value, { ...draft, id: `proxy-${Date.now()}` }]
  const ok = await proxy.saveConfig(next)
  if (ok?.ok !== false) {
    toast.add({ title: '已保存', color: 'success' })
    editOpen.value = false
    await proxy.fetchOverview()
  } else {
    toast.add({ title: '保存失败', description: String(ok?.error?.message ?? ''), color: 'error' })
  }
}

async function removeInstance(item: ProxyInstance) {
  const next = instances.value.filter(i => i.id !== item.id)
  await proxy.saveConfig(next)
  await proxy.fetchOverview()
  toast.add({ title: '已删除', color: 'neutral' })
}

async function toggleRun(item: ProxyInstance) {
  const running = statusMap.value[item.id]?.running
  await (running ? proxy.stopInstance(item.id) : proxy.startInstance(item.id))
  await proxy.fetchOverview()
}

/* ---------- 前置代理 ---------- */

const upstreamEditing = ref<UpstreamProxy | null>(null)
const upstreamOpen = ref(false)

function newUpstream() {
  upstreamEditing.value = { id: '', name: '', addr: '', username: '', password: '', enabled: true }
  upstreamOpen.value = true
}

function editUpstream(item: UpstreamProxy) {
  upstreamEditing.value = { ...item, password: '' }
  upstreamOpen.value = true
}

async function saveUpstream() {
  const draft = upstreamEditing.value
  if (!draft) return
  const result = draft.id
    ? await upstream.updateProxy(draft.id, draft)
    : await upstream.createProxy(draft)
  if (result?.ok !== false) {
    toast.add({ title: '已保存', color: 'success' })
    upstreamOpen.value = false
    await upstream.fetchAll()
  } else {
    toast.add({ title: '保存失败', color: 'error' })
  }
}

async function removeUpstream(item: UpstreamProxy) {
  await upstream.deleteProxy(item.id)
  await upstream.fetchAll()
  toast.add({ title: '已删除', color: 'neutral' })
}

/* ---------- 国家路由规则 ---------- */

const countryItems = computed(() =>
  countries.value.map(c => ({
    label: `${c.country_name}（${c.country_code}）`,
    value: c.country_code
  }))
)
const proxyItems = computed(() =>
  proxies.value.map(p => ({ label: p.name || p.id, value: p.id }))
)

const ruleCountry = ref('')
const ruleProxy = ref('')

async function addRule() {
  if (!ruleCountry.value || !ruleProxy.value) {
    toast.add({ title: '请选择国家与前置代理', color: 'warning' })
    return
  }
  await upstream.upsertCountryRule(ruleCountry.value, { upstream_proxy_id: ruleProxy.value })
  await upstream.fetchAll()
  ruleCountry.value = ''
  ruleProxy.value = ''
  toast.add({ title: '规则已保存', color: 'success' })
}

async function removeRule(code: string) {
  await upstream.deleteCountryRule(code)
  await upstream.fetchAll()
}

function proxyName(id: string) {
  return proxies.value.find(p => p.id === id)?.name || id || '—'
}

usePollingScheduler(() => proxy.fetchOverview(), 10000, { immediate: true, backgroundIntervalMs: 30000 })
onMounted(() => { void upstream.fetchAll() })
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="代理" description="设备出口代理与 VoWiFi 前置代理" />

    <UTabs v-model="tab" :items="tabs" :content="false" class="mb-5" />

    <!-- 代理实例 -->
    <div v-if="tab === 'instances'">
      <div class="flex items-center justify-between gap-4 mb-3">
        <h2 class="text-sm font-semibold">代理实例</h2>
        <div class="flex gap-2">
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            size="sm"
            :loading="loading"
            @click="proxy.fetchOverview()"
          />
          <UButton icon="i-lucide-plus" size="sm" label="新建" @click="newInstance" />
        </div>
      </div>

      <UAlert
        v-if="error"
        color="error"
        variant="subtle"
        icon="i-lucide-triangle-alert"
        title="加载失败"
        :description="String(error.message || error)"
        class="mb-4"
      />

      <div v-if="!instances.length" class="tile">
        <EmptyState icon="i-lucide-server-off" title="还没有代理实例">
          <UButton size="sm" variant="soft" label="新建实例" @click="newInstance" />
        </EmptyState>
      </div>

      <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <div v-for="item in instances" :key="item.id" class="tile flex flex-col px-4 py-3.5">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="font-medium truncate">{{ item.name || item.id }}</p>
              <p class="text-xs text-muted font-mono truncate mt-0.5">
                {{ item.listen_addr }}:{{ item.listen_port }}
              </p>
            </div>
            <UBadge
              :color="statusMap[item.id]?.running ? 'success' : 'neutral'"
              variant="subtle"
              :label="statusMap[item.id]?.running ? '运行中' : '已停止'"
            />
          </div>

          <dl class="mt-3 flex flex-col gap-1.5 text-sm">
            <div class="flex justify-between gap-2">
              <dt class="text-muted">模式</dt>
              <dd class="uppercase">{{ item.mode }}</dd>
            </div>
            <div class="flex justify-between gap-2">
              <dt class="text-muted">绑定设备</dt>
              <dd class="truncate">{{ item.device_id || '—' }}</dd>
            </div>
            <div class="flex justify-between gap-2">
              <dt class="text-muted">认证</dt>
              <dd>{{ item.auth_enabled ? '已启用' : '关闭' }}</dd>
            </div>
          </dl>

          <p
            v-if="statusMap[item.id]?.last_error"
            class="mt-3 text-xs text-error break-all"
          >
            {{ statusMap[item.id]?.last_error }}
          </p>

          <div class="mt-3.5 flex gap-1.5 border-t border-default pt-3">
              <UButton
                size="sm"
                :icon="statusMap[item.id]?.running ? 'i-lucide-square' : 'i-lucide-play'"
                :color="statusMap[item.id]?.running ? 'warning' : 'primary'"
                variant="soft"
                :label="statusMap[item.id]?.running ? '停止' : '启动'"
                @click="toggleRun(item)"
              />
              <UButton
                size="sm"
                icon="i-lucide-rotate-cw"
                color="neutral"
                variant="soft"
                aria-label="重启"
                :disabled="!item.enabled"
                @click="restartInstance(item)"
              />
              <UButton
                size="sm"
                icon="i-lucide-pencil"
                color="neutral"
                variant="ghost"
                label="编辑"
                @click="editInstance(item)"
              />
              <UButton
                size="sm"
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                class="ml-auto"
                @click="removeInstance(item)"
              />
          </div>
        </div>
      </div>
    </div>

    <!-- 前置代理 -->
    <template v-else>
      <div>
        <div class="flex items-center justify-between gap-4 mb-3">
          <div>
            <h2 class="text-sm font-semibold">前置代理</h2>
            <p class="text-xs text-muted mt-0.5">VoWiFi 隧道出海使用的 SOCKS5 上游</p>
          </div>
          <UButton icon="i-lucide-plus" size="sm" label="新建" @click="newUpstream" />
        </div>

        <div v-if="!proxies.length" class="tile">
          <EmptyState icon="i-lucide-network" title="还没有前置代理" />
        </div>

        <div v-else class="tile divide-y divide-default">
          <div
            v-for="p in proxies"
            :key="p.id"
            class="flex items-center gap-4 px-4 py-3"
          >
            <div class="min-w-0 flex-1">
              <p class="font-medium truncate">{{ p.name || p.id }}</p>
              <p class="text-xs text-muted font-mono truncate mt-0.5">{{ p.addr }}</p>
            </div>
            <UBadge
              :color="p.enabled ? 'success' : 'neutral'"
              variant="subtle"
              :label="p.enabled ? '启用' : '停用'"
            />
            <UButton
              size="sm"
              icon="i-lucide-pencil"
              color="neutral"
              variant="ghost"
              @click="editUpstream(p)"
            />
            <UButton
              size="sm"
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              @click="removeUpstream(p)"
            />
          </div>
        </div>
      </div>

      <div>
        <div class="mb-3">
          <h2 class="text-sm font-semibold">国家路由规则</h2>
          <p class="text-xs text-muted mt-0.5">按 SIM 归属国家选择前置代理出口</p>
        </div>
        <div class="tile p-4">

        <div class="flex flex-wrap items-end gap-3 mb-4">
          <UFormField label="国家" class="flex-1 min-w-48">
            <USelectMenu
              v-model="ruleCountry"
              :items="countryItems"
              value-key="value"
              placeholder="选择国家"
              searchable
              class="w-full"
            />
          </UFormField>
          <UFormField label="前置代理" class="flex-1 min-w-48">
            <USelectMenu
              v-model="ruleProxy"
              :items="proxyItems"
              value-key="value"
              placeholder="选择代理"
              class="w-full"
            />
          </UFormField>
          <UButton icon="i-lucide-plus" label="添加规则" @click="addRule" />
        </div>

        <div
          v-if="!countryRules.length"
          class="text-center py-8 text-sm text-muted"
        >
          暂无规则，VoWiFi 将不使用前置代理
        </div>

        <div v-else class="flex flex-wrap gap-2">
          <UBadge
            v-for="r in countryRules"
            :key="r.country_code"
            color="neutral"
            variant="outline"
            size="lg"
            class="gap-2"
          >
            <span class="font-medium">{{ r.country_code }}</span>
            <UIcon name="i-lucide-arrow-right" class="size-3 text-dimmed" />
            <span>{{ proxyName(r.upstream_proxy_id) }}</span>
            <UButton
              icon="i-lucide-x"
              size="xs"
              color="neutral"
              variant="ghost"
              class="-mr-1"
              @click="removeRule(r.country_code)"
            />
          </UBadge>
        </div>
        </div>
      </div>
    </template>

    <!-- 实例编辑 -->
    <UModal v-model:open="editOpen" :title="editing?.id ? '编辑代理实例' : '新建代理实例'">
      <template #body>
        <div v-if="editing" class="flex flex-col gap-4">
          <UFormField label="名称">
            <UInput v-model="editing.name" placeholder="例如 ec20-socks" class="w-full" />
          </UFormField>
          <UFormField label="绑定设备">
            <USelectMenu
              v-model="editing.device_id"
              :items="deviceItems"
              value-key="value"
              class="w-full"
            />
          </UFormField>
          <div class="grid grid-cols-2 gap-3">
            <UFormField label="模式">
              <USelect v-model="editing.mode" :items="modes" value-key="value" class="w-full" />
            </UFormField>
            <UFormField label="监听端口">
              <UInputNumber v-model="editing.listen_port" :min="1" :max="65535" class="w-full" />
            </UFormField>
          </div>
          <UFormField label="监听地址">
            <UInput v-model="editing.listen_addr" placeholder="0.0.0.0" class="w-full" />
          </UFormField>
          <USwitch v-model="editing.enabled" label="启用该实例" />
          <USwitch v-model="editing.auth_enabled" label="启用用户名密码认证" />
          <template v-if="editing.auth_enabled">
            <UFormField label="用户名">
              <UInput v-model="editing.username" class="w-full" />
            </UFormField>
            <UFormField label="密码">
              <UInput v-model="editing.password" type="password" class="w-full" />
            </UFormField>
          </template>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton color="neutral" variant="ghost" label="取消" @click="editOpen = false" />
          <UButton label="保存" @click="saveInstance" />
        </div>
      </template>
    </UModal>

    <!-- 前置代理编辑 -->
    <UModal v-model:open="upstreamOpen" :title="upstreamEditing?.id ? '编辑前置代理' : '新建前置代理'">
      <template #body>
        <div v-if="upstreamEditing" class="flex flex-col gap-4">
          <UFormField label="名称">
            <UInput v-model="upstreamEditing.name" placeholder="例如 uk" class="w-full" />
          </UFormField>
          <UFormField label="地址" help="host:port，IPv6 用 [::1]:1080 形式">
            <UInput v-model="upstreamEditing.addr" placeholder="127.0.0.1:7890" class="w-full" />
          </UFormField>
          <UFormField label="用户名">
            <UInput v-model="upstreamEditing.username" class="w-full" />
          </UFormField>
          <UFormField label="密码" :help="upstreamEditing.id ? '留空表示不修改' : undefined">
            <UInput v-model="upstreamEditing.password" type="password" class="w-full" />
          </UFormField>
          <USwitch v-model="upstreamEditing.enabled" label="启用" />
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton color="neutral" variant="ghost" label="取消" @click="upstreamOpen = false" />
          <UButton label="保存" @click="saveUpstream" />
        </div>
      </template>
    </UModal>
  </div>
</template>
