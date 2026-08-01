import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AppError } from '~/types/domain'
import type { DeviceMgmtListItem } from '~/types/api'
import type { SMSMessageDTO, SmsThreadVM } from '~/types/view-model'
import { smsService, type SmsDeleteThreadPayload, type SmsSendPayload, type SmsThreadQueryParams } from '~/services/sms'

export const useSMSStore = defineStore('sms', () => {
  const devices = ref<DeviceMgmtListItem[]>([])
  const threads = ref<SmsThreadVM[]>([])
  const threadMessages = ref<SMSMessageDTO[]>([])

  const loading = ref(false)
  const lastOkAt = ref<number | null>(null)
  const error = ref<AppError | null>(null)

  async function fetchDevices() {
    const result = await smsService.listDevices()
    if (result.ok) devices.value = result.data
    return result
  }

  async function fetchThreads(deviceId?: string) {
    const result = await smsService.listContacts(deviceId)
    if (result.ok) {
      threads.value = result.data
      lastOkAt.value = Date.now()
      error.value = null
    } else {
      error.value = result.error
    }
    return result
  }

  async function fetchThread(params: SmsThreadQueryParams) {
    const result = await smsService.getThread(params)
    if (result.ok) threadMessages.value = result.data
    return result
  }

  /** 向上翻历史：结果拼在前面，不覆盖已加载的消息 */
  async function loadMoreThread(params: SmsThreadQueryParams) {
    const result = await smsService.getThread(params)
    if (result.ok) {
      const known = new Set(threadMessages.value.map(m => m.id))
      const older = result.data.filter(m => !known.has(m.id))
      threadMessages.value = [...older, ...threadMessages.value]
      return { ok: true as const, added: older.length }
    }
    return { ok: false as const, added: 0 }
  }

  async function send(payload: SmsSendPayload) {
    return smsService.send(payload)
  }

  async function deleteMessage(id: number) {
    return smsService.deleteMessage(id)
  }

  async function deleteThread(payload: SmsDeleteThreadPayload) {
    return smsService.deleteThread(payload)
  }

  return {
    devices,
    threads,
    threadMessages,
    loading,
    lastOkAt,
    error,
    fetchDevices,
    fetchThreads,
    fetchThread,
    loadMoreThread,
    send,
    deleteMessage,
    deleteThread
  }
})
