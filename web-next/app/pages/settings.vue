<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useSettingsStore } from '~/stores/settings'
import { systemService } from '~/services/system'

const settings = useSettingsStore()
const toast = useToast()

const {
  systemInfo,
  passwordForm,
  telegramForm,
  feishuForm,
  qqForm,
  webhookSettings,
  barkSettings,
  emailForm,
  pushplusForm,
  loadingSystemInfo,
  loadingNotifications,
  savingNotifications,
  testingWebhook,
  testingBark,
  testingEmail,
  changingPassword
} = storeToRefs(settings)

const tab = ref('system')
const tabs = [
  { label: '系统', value: 'system', icon: 'i-lucide-info' },
  { label: '通知', value: 'notify', icon: 'i-lucide-bell' },
  { label: '安全', value: 'security', icon: 'i-lucide-shield' }
]

async function saveNotifications() {
  const result = await settings.saveNotificationsFromForms()
  toast.add(
    result?.ok === false
      ? { title: '保存失败', description: String(result?.error?.message ?? ''), color: 'error' }
      : { title: '通知设置已保存', color: 'success' }
  )
}

async function runTest(kind: 'webhook' | 'bark' | 'email') {
  const fn = {
    webhook: settings.testWebhookFromForm,
    bark: settings.testBarkFromForm,
    email: settings.testEmailFromForm
  }[kind]
  const result = await fn()
  toast.add(
    result?.ok === false
      ? { title: '测试失败', description: String(result?.error?.message ?? ''), color: 'error' }
      : { title: '测试消息已发送', color: 'success' }
  )
}

async function changePassword() {
  const result = await settings.changePasswordFromForm()
  if (result?.ok === false) {
    toast.add({ title: '修改失败', description: String(result?.error?.message ?? ''), color: 'error' })
    return
  }
  toast.add({ title: '密码已修改', color: 'success' })
  settings.resetPasswordForm()
}

const infoItems = computed(() => [
  { label: '版本', value: systemInfo.value?.version },
  { label: '构建时间', value: systemInfo.value?.build_time },
  { label: 'Go 版本', value: systemInfo.value?.go_version },
  { label: '运行平台', value: systemInfo.value?.platform },
  { label: '运行时长', value: systemInfo.value?.uptime },
  { label: '部署方式', value: systemInfo.value?.deploy_mode }
].filter(i => i.value))

/* 数组/映射类字段用多行文本编辑，读写两侧各转一次 */
function linesToArray(text: string): string[] {
  return text.split('\n').map(l => l.trim()).filter(Boolean)
}

const webhookUrlsText = computed(() => (webhookSettings.value.urls ?? []).join('\n'))
const barkUrlsText = computed(() => (barkSettings.value.urls ?? []).join('\n'))

const webhookHeadersText = computed(() =>
  Object.entries(webhookSettings.value.headers ?? {}).map(([k, v]) => `${k}: ${v}`).join('\n'))

function textToHeaders(text: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const line of text.split('\n')) {
    const idx = line.indexOf(':')
    if (idx <= 0) continue
    const key = line.slice(0, idx).trim()
    if (key) out[key] = line.slice(idx + 1).trim()
  }
  return out
}

const barkLevelItems = [
  { label: '时效性 (timeSensitive)', value: 'timeSensitive' },
  { label: '积极 (active)', value: 'active' },
  { label: '被动 (passive)', value: 'passive' }
]

const pushplusChannelItems = [
  { label: '微信 (wechat)', value: 'wechat' },
  { label: 'Webhook (webhook)', value: 'webhook' },
  { label: '企业微信 (cp)', value: 'cp' },
  { label: '邮件 (mail)', value: 'mail' }
]

onMounted(() => {
  void settings.fetchSystemInfo()
  void settings.fetchNotifications()
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="设置" description="系统信息、通知渠道与账户安全" />

    <UTabs v-model="tab" :items="tabs" :content="false" class="mb-5" />

    <!-- 系统信息 -->
    <div v-if="tab === 'system'">
      <div class="flex items-center justify-between gap-4 mb-3">
        <h2 class="text-sm font-semibold">系统信息</h2>
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          size="sm"
          :loading="loadingSystemInfo"
          @click="settings.fetchSystemInfo()"
        />
      </div>

      <dl class="tile grid divide-y divide-default sm:grid-cols-2 sm:divide-x">
        <div v-for="i in infoItems" :key="i.label" class="flex flex-col gap-1 px-4 py-3">
          <dt class="text-xs uppercase tracking-wide text-dimmed">{{ i.label }}</dt>
          <dd class="text-sm font-mono break-all">{{ i.value }}</dd>
        </div>
      </dl>

      <div v-if="!infoItems.length" class="tile">
        <EmptyState icon="i-lucide-info" title="暂无系统信息" />
      </div>

    </div>

    <!-- 通知 -->
    <template v-else-if="tab === 'notify'">
      <div>
        <div class="flex items-center justify-between gap-4 mb-3">
          <div>
            <h2 class="text-sm font-semibold">通知渠道</h2>
            <p class="text-xs text-muted mt-0.5">短信到达、设备异常等事件的推送方式</p>
          </div>
          <UButton
            icon="i-lucide-save"
            size="sm"
            label="保存全部"
            :loading="savingNotifications"
            @click="saveNotifications"
          />
        </div>
        <div class="tile px-4">

        <div v-if="loadingNotifications" class="flex flex-col gap-4">
          <USkeleton v-for="i in 4" :key="i" class="h-20 w-full" />
        </div>

        <UAccordion
          v-else
          :items="[
            { label: 'Telegram', icon: 'i-lucide-send', slot: 'telegram' },
            { label: '飞书', icon: 'i-lucide-message-circle', slot: 'feishu' },
            { label: 'QQ', icon: 'i-lucide-message-square', slot: 'qq' },
            { label: 'Webhook', icon: 'i-lucide-webhook', slot: 'webhook' },
            { label: 'Bark', icon: 'i-lucide-smartphone', slot: 'bark' },
            { label: '邮件', icon: 'i-lucide-mail', slot: 'email' },
            { label: 'PushPlus', icon: 'i-lucide-bell-ring', slot: 'pushplus' }
          ]"
        >
          <template #telegram>
            <div class="flex flex-col gap-3 pb-2">
              <USwitch v-model="telegramForm.enabled" label="启用 Telegram 通知" />
              <UFormField label="Bot Token">
                <UInput v-model="telegramForm.bot_token" type="password" class="w-full" />
              </UFormField>
              <div class="grid gap-3 sm:grid-cols-2">
                <UFormField label="Chat ID" help="接收通知的会话">
                  <UInputNumber v-model="telegramForm.chat_id" class="w-full" />
                </UFormField>
                <UFormField label="Admin ID" help="允许下发指令的管理员">
                  <UInputNumber v-model="telegramForm.admin_id" class="w-full" />
                </UFormField>
              </div>
              <UFormField label="API 地址" help="留空用官方 api.telegram.org；自建反代时填写">
                <UInput v-model="telegramForm.base_url" placeholder="https://api.telegram.org" class="w-full" />
              </UFormField>
              <UFormField label="代理" help="形如 socks5://127.0.0.1:1080，留空直连">
                <UInput v-model="telegramForm.proxy" class="w-full font-mono" />
              </UFormField>
            </div>
          </template>

          <template #feishu>
            <div class="flex flex-col gap-3 pb-2">
              <USwitch v-model="feishuForm.enabled" label="启用飞书通知" />
              <div class="grid gap-3 sm:grid-cols-2">
                <UFormField label="App ID">
                  <UInput v-model="feishuForm.app_id" class="w-full font-mono" />
                </UFormField>
                <UFormField label="App Secret">
                  <UInput v-model="feishuForm.app_secret" type="password" class="w-full" />
                </UFormField>
              </div>
              <UFormField label="群 Chat ID" help="多个用英文逗号分隔">
                <UInput v-model="feishuForm.chat_ids" placeholder="oc_xxx,oc_yyy" class="w-full font-mono" />
              </UFormField>
            </div>
          </template>

          <template #qq>
            <div class="flex flex-col gap-3 pb-2">
              <USwitch v-model="qqForm.enabled" label="启用 QQ 通知" />
              <div class="grid gap-3 sm:grid-cols-2">
                <UFormField label="App ID">
                  <UInput v-model="qqForm.app_id" class="w-full font-mono" />
                </UFormField>
                <UFormField label="App Secret">
                  <UInput v-model="qqForm.app_secret" type="password" class="w-full" />
                </UFormField>
              </div>
              <UFormField label="群号" help="多个用英文逗号分隔">
                <UInput v-model="qqForm.group_ids" class="w-full font-mono" />
              </UFormField>
              <UFormField label="私聊 ID" help="多个用英文逗号分隔">
                <UInput v-model="qqForm.direct_ids" class="w-full font-mono" />
              </UFormField>
            </div>
          </template>

          <template #webhook>
            <div class="flex flex-col gap-3 pb-2">
              <USwitch v-model="webhookSettings.enabled" label="启用 Webhook" />
              <UFormField label="回调地址" help="一行一个，可配置多个接收端">
                <UTextarea
                  :model-value="webhookUrlsText"
                  :rows="2"
                  autoresize
                  :maxrows="6"
                  placeholder="https://example.com/hook"
                  class="w-full font-mono"
                  @update:model-value="v => webhookSettings.urls = linesToArray(String(v))"
                />
              </UFormField>
              <UFormField
                label="签名密钥"
                help="若配置，将通过请求头 X-Nyantenna-Signature 携带 HMAC-SHA256 签名"
              >
                <UInput v-model="webhookSettings.secret" type="password" class="w-full" />
              </UFormField>
              <div class="grid gap-3 sm:grid-cols-2">
                <UFormField label="超时 (ms)">
                  <UInputNumber v-model="webhookSettings.timeout_ms" :min="500" :max="60000" :step="500" class="w-full" />
                </UFormField>
                <UFormField label="最大重试次数">
                  <UInputNumber v-model="webhookSettings.retry_max" :min="0" :max="10" class="w-full" />
                </UFormField>
              </div>
              <UFormField label="文本模板" help="可用变量：{{device_label}}、{{text}}、{{sender}}">
                <UInput v-model="webhookSettings.text_template" class="w-full font-mono" />
              </UFormField>
              <UFormField label="自定义请求头" help="每行 Name: Value；Content-Type 与签名头由系统接管，写了也会被忽略">
                <UTextarea
                  :model-value="webhookHeadersText"
                  :rows="2"
                  autoresize
                  :maxrows="6"
                  placeholder="X-Token: abc123"
                  class="w-full font-mono"
                  @update:model-value="v => webhookSettings.headers = textToHeaders(String(v))"
                />
              </UFormField>
              <div>
                <UButton
                  size="sm"
                  variant="soft"
                  icon="i-lucide-send"
                  label="发送测试"
                  :loading="testingWebhook"
                  @click="runTest('webhook')"
                />
              </div>
            </div>
          </template>

          <template #bark>
            <div class="flex flex-col gap-3 pb-2">
              <USwitch v-model="barkSettings.enabled" label="启用 Bark" />
              <UFormField label="推送地址" help="一行一个，形如 https://api.day.app/你的DeviceKey">
                <UTextarea
                  :model-value="barkUrlsText"
                  :rows="2"
                  autoresize
                  :maxrows="6"
                  placeholder="https://api.day.app/xxxxxxxx"
                  class="w-full font-mono"
                  @update:model-value="v => barkSettings.urls = linesToArray(String(v))"
                />
              </UFormField>
              <div class="grid gap-3 sm:grid-cols-2">
                <UFormField label="分组 (Group)" help="iOS 通知中心里的分组名">
                  <UInput v-model="barkSettings.group" class="w-full" />
                </UFormField>
                <UFormField label="通知级别 (Level)" help="决定是否亮屏、是否穿透专注模式">
                  <USelect v-model="barkSettings.level" :items="barkLevelItems" value-key="value" class="w-full" />
                </UFormField>
              </div>
              <UFormField label="图标 URL" help="可选">
                <UInput v-model="barkSettings.icon" class="w-full font-mono" />
              </UFormField>
              <div>
                <UButton
                  size="sm"
                  variant="soft"
                  icon="i-lucide-send"
                  label="发送测试"
                  :loading="testingBark"
                  @click="runTest('bark')"
                />
              </div>
            </div>
          </template>

          <template #email>
            <div class="flex flex-col gap-3 pb-2">
              <USwitch v-model="emailForm.enabled" label="启用邮件通知" />
              <div class="grid gap-3 sm:grid-cols-2">
                <UFormField label="SMTP 主机">
                  <UInput v-model="emailForm.smtp_host" class="w-full" />
                </UFormField>
                <UFormField label="端口">
                  <UInputNumber v-model="emailForm.smtp_port" :min="1" :max="65535" class="w-full" />
                </UFormField>
              </div>
              <USwitch v-model="emailForm.use_ssl" label="使用 SSL/TLS" />
              <div class="grid gap-3 sm:grid-cols-2">
                <UFormField label="用户名">
                  <UInput v-model="emailForm.username" class="w-full" />
                </UFormField>
                <UFormField label="密码">
                  <UInput v-model="emailForm.password" type="password" class="w-full" />
                </UFormField>
              </div>
              <UFormField label="发件地址">
                <UInput v-model="emailForm.from_address" placeholder="bot@example.com" class="w-full" />
              </UFormField>
              <UFormField label="收件地址" help="多个用英文逗号分隔">
                <UInput v-model="emailForm.to_addresses" placeholder="a@example.com,b@example.com" class="w-full" />
              </UFormField>
              <div>
                <UButton
                  size="sm"
                  variant="soft"
                  icon="i-lucide-send"
                  label="发送测试"
                  :loading="testingEmail"
                  @click="runTest('email')"
                />
              </div>
            </div>
          </template>

          <template #pushplus>
            <div class="flex flex-col gap-3 pb-2">
              <USwitch v-model="pushplusForm.enabled" label="启用 PushPlus" />
              <UFormField label="Token">
                <UInput v-model="pushplusForm.token" type="password" class="w-full" />
              </UFormField>
              <UFormField label="群组编码 (Topic)" help="留空则发给个人">
                <UInput v-model="pushplusForm.topic" class="w-full font-mono" />
              </UFormField>
              <UFormField label="渠道 (Channel)">
                <USelect v-model="pushplusForm.channel" :items="pushplusChannelItems" value-key="value" class="w-full" />
              </UFormField>
            </div>
          </template>

        </UAccordion>
        </div>
      </div>
    </template>

    <!-- 安全 -->
    <div v-else>
      <h2 class="text-sm font-semibold mb-3">修改密码</h2>
      <form class="tile flex flex-col gap-4 max-w-md p-4" @submit.prevent="changePassword">
        <UFormField label="当前密码">
          <UInput
            v-model="passwordForm.old_password"
            type="password"
            autocomplete="current-password"
            class="w-full"
          />
        </UFormField>
        <UFormField label="新密码">
          <UInput
            v-model="passwordForm.new_password"
            type="password"
            autocomplete="new-password"
            class="w-full"
          />
        </UFormField>
        <UFormField label="确认新密码">
          <UInput
            v-model="passwordForm.confirm_password"
            type="password"
            autocomplete="new-password"
            class="w-full"
          />
        </UFormField>
        <div>
          <UButton type="submit" label="修改密码" :loading="changingPassword" />
        </div>
      </form>
    </div>
  </div>
</template>
