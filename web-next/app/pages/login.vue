<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'

definePageMeta({ layout: 'blank' })

const auth = useAuthStore()
const router = useRouter()
const toast = useToast()

const username = ref('')
const password = ref('')
const loading = ref(false)

async function submit() {
  if (!username.value || !password.value) {
    toast.add({ title: '请输入用户名和密码', color: 'warning' })
    return
  }
  loading.value = true
  const ok = await auth.login(username.value, password.value)
  loading.value = false
  if (ok) {
    router.push('/')
  } else {
    toast.add({ title: '登录失败', description: '请检查用户名与密码', color: 'error' })
  }
}
</script>

<template>
  <div class="relative min-h-dvh flex items-center justify-center overflow-hidden bg-default p-4">
    <!-- 背景：柔和的径向光晕，避免大片纯色显得空 -->
    <div
      aria-hidden="true"
      class="pointer-events-none absolute inset-0 opacity-70
             [background:radial-gradient(60rem_40rem_at_50%_-10%,var(--ui-primary),transparent_60%)]
             [mask-image:radial-gradient(60rem_40rem_at_50%_-10%,black,transparent_70%)]"
      style="filter: blur(80px)"
    />

    <div class="relative w-full max-w-[22rem]">
      <div class="flex flex-col items-center gap-3.5 mb-8">
        <div
          class="flex size-12 items-center justify-center rounded-xl bg-primary/10 ring-1 ring-primary/20"
        >
          <UIcon name="i-lucide-radio-tower" class="size-6 text-primary" />
        </div>
        <div class="text-center">
          <h1 class="text-2xl font-semibold tracking-tight">Nyantenna</h1>
          <p class="text-sm text-muted mt-1.5">模组与 IMS 管理控制台</p>
        </div>
      </div>

      <div class="tile p-5 shadow-sm">
        <form class="flex flex-col gap-4" @submit.prevent="submit">
          <UFormField label="用户名">
            <UInput
              v-model="username"
              icon="i-lucide-user"
              placeholder="用户名"
              autocomplete="username"
              size="lg"
              class="w-full"
            />
          </UFormField>

          <UFormField label="密码">
            <UInput
              v-model="password"
              type="password"
              icon="i-lucide-lock"
              placeholder="密码"
              autocomplete="current-password"
              size="lg"
              class="w-full"
            />
          </UFormField>

          <UButton
            type="submit"
            block
            size="lg"
            :loading="loading"
            label="登录"
          />
        </form>
      </div>

      <p class="text-center text-xs text-dimmed mt-6">Nyantenna &copy; 2026</p>
    </div>
  </div>
</template>
