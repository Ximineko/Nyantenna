<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const colorMode = useColorMode()

const nav = [[
  { label: '总览', icon: 'i-lucide-layout-dashboard', to: '/' },
  { label: '设备', icon: 'i-lucide-cpu', to: '/devices' },
  { label: '短信', icon: 'i-lucide-message-square', to: '/sms' }
], [
  { label: '代理', icon: 'i-lucide-globe', to: '/proxy' },
  { label: '日志', icon: 'i-lucide-scroll-text', to: '/logs' },
  { label: '设置', icon: 'i-lucide-settings', to: '/settings' }
]]

const isDark = computed({
  get: () => colorMode.value === 'dark',
  set: (v: boolean) => { colorMode.preference = v ? 'dark' : 'light' }
})

function logout() {
  auth.logout?.()
  router.push('/login')
}

/* Ctrl+Shift+D 呼出诊断面板 */
const debugOpen = ref(false)
function onKeydown(e: KeyboardEvent) {
  if (e.ctrlKey && e.shiftKey && String(e.key || '').toLowerCase() === 'd') {
    e.preventDefault()
    debugOpen.value = !debugOpen.value
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

const authed = computed(() => auth.isAuthenticated)
const current = computed(() =>
  nav.flat().find(l => l.to === route.path)?.label ?? 'Nyantenna'
)
</script>

<template>
  <UDashboardGroup v-if="authed" unit="rem">
    <UDashboardSidebar
      collapsible
      resizable
      :min-size="12"
      :default-size="14"
      :max-size="18"
      :ui="{ footer: 'border-t border-default' }"
    >
      <template #header="{ collapsed }">
        <div class="flex items-center gap-2.5 px-1 py-0.5">
          <div
            class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 ring-1 ring-primary/20"
          >
            <UIcon name="i-lucide-radio-tower" class="size-4 text-primary" />
          </div>
          <div v-if="!collapsed" class="min-w-0 leading-tight">
            <p class="text-sm font-semibold tracking-tight truncate">Nyantenna</p>
            <p class="text-[11px] text-dimmed truncate">模组与 IMS 控制台</p>
          </div>
        </div>
      </template>

      <template #default="{ collapsed }">
        <UNavigationMenu
          :items="nav"
          :collapsed="collapsed"
          orientation="vertical"
          highlight
          :ui="{ link: 'gap-2.5' }"
        />
      </template>

      <template #footer="{ collapsed }">
        <div class="flex w-full items-center gap-1" :class="collapsed ? 'flex-col' : ''">
          <UButton
            :icon="isDark ? 'i-lucide-moon' : 'i-lucide-sun'"
            color="neutral"
            variant="ghost"
            size="sm"
            square
            :aria-label="isDark ? '切换到浅色' : '切换到深色'"
            @click="isDark = !isDark"
          />
          <UButton
            icon="i-lucide-stethoscope"
            color="neutral"
            variant="ghost"
            size="sm"
            square
            aria-label="诊断面板"
            @click="debugOpen = true"
          />
          <UButton
            icon="i-lucide-log-out"
            color="neutral"
            variant="ghost"
            size="sm"
            :square="collapsed"
            :label="collapsed ? undefined : '退出'"
            :class="collapsed ? '' : 'ml-auto'"
            @click="logout"
          />
        </div>
      </template>
    </UDashboardSidebar>

    <UDashboardPanel :ui="{ body: 'p-4 sm:p-6 lg:p-8' }">
      <template #header>
        <UDashboardNavbar
          :title="current"
          :ui="{ root: 'border-b border-default', title: 'text-sm font-medium text-muted' }"
        >
          <template #leading>
            <UDashboardSidebarCollapse />
          </template>
        </UDashboardNavbar>
      </template>

      <template #body>
        <!-- min-h-full + flex：让需要撑满视口的页面（如短信）能用 flex-1 占满剩余高度，
             同时不限制内容更长的页面继续向下增长 -->
        <div class="mx-auto flex min-h-full w-full max-w-7xl flex-col">
          <slot />
        </div>
      </template>
    </UDashboardPanel>

    <DebugPanel v-model:open="debugOpen" />
  </UDashboardGroup>

  <slot v-else />
</template>
