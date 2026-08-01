import { getMccMncIndex, isoToFlagEmoji, type MccMncRow } from '~/utils/mcc-mnc'
import type { ModemStatus, PNNRecord } from '~/types/api'

// MCC/MNC 表只在首次使用时拉一次，模块级缓存跨组件共享
const index = ref<Map<string, MccMncRow> | null>(null)
let started = false

function normalizeSPN(v: unknown): string {
  return String(v ?? '').trim()
}

function nativeMccMnc(modem: ModemStatus | undefined | null): string {
  const mcc = String(modem?.native_mcc ?? '').trim()
  const mnc = String(modem?.native_mnc ?? '').trim()
  return mcc && mnc ? `${mcc}${mnc}` : ''
}

function pnnDisplayName(record: PNNRecord | undefined): string {
  return normalizeSPN(record?.full_name) || normalizeSPN(record?.short_name)
}

function firstPNNName(records: PNNRecord[] | undefined): string {
  if (!Array.isArray(records)) return ''
  for (const r of records) {
    const name = pnnDisplayName(r)
    if (name) return name
  }
  return ''
}

// OPL 的 PLMN 允许用 'D' 作通配位，逐位比对
function oplMatchesNativePLMN(oplPLMN: string | undefined, nativePLMN: string): boolean {
  const opl = normalizeSPN(oplPLMN).toUpperCase()
  if (!opl || opl.length !== nativePLMN.length) return false
  for (let i = 0; i < opl.length; i++) {
    const c = opl[i]
    if (c === 'D') continue
    if (c !== nativePLMN[i]) return false
  }
  return true
}

// 归属运营商名优先从 SIM 的 OPL/PNN 里查，查不到再退回 PNN 首条
function pnnNameFromOPL(modem: ModemStatus | undefined | null): string {
  const nativePLMN = nativeMccMnc(modem)
  if (!nativePLMN || !Array.isArray(modem?.opl) || !Array.isArray(modem?.pnn)) return ''
  for (const opl of modem.opl) {
    if (!oplMatchesNativePLMN(opl?.plmn, nativePLMN)) continue
    const pnnRecord = Number(opl?.pnn_record ?? 0)
    if (!pnnRecord) continue
    const name = pnnDisplayName(modem.pnn.find(record => record.record === pnnRecord))
    if (name) return name
  }
  return ''
}

export function useOperatorDisplay() {
  if (!started) {
    started = true
    getMccMncIndex().then((i) => { index.value = i }).catch(() => {})
  }

  function flagForMccMnc(code: string): string {
    const row = index.value?.get(code)
    return row ? isoToFlagEmoji(row.iso) : ''
  }

  function formatNamedOperator(name: string, code: string): string {
    const flag = flagForMccMnc(code)
    if (!code) return flag ? `${flag} ${name}` : name
    return `${flag ? flag + ' ' : ''}${name} (${code})`
  }

  function formatMccMncOperator(code: string): string {
    const idx = index.value
    if (!idx || !code) return code
    const row = idx.get(code)
    if (!row) return code
    const name = normalizeSPN(row.network) || normalizeSPN(row.country)
    return name ? formatNamedOperator(name, code) : code
  }

  /** SIM 卡自身的归属运营商：SPN > OPL/PNN > PNN 首条 > 纯 PLMN */
  function simOperatorDisplay(modem: ModemStatus | undefined | null): string {
    if (!modem) return '--'
    const spn = normalizeSPN(modem.native_spn)
    const pnn = pnnNameFromOPL(modem) || firstPNNName(modem.pnn)
    const mccmnc = nativeMccMnc(modem)
    if (spn) return formatNamedOperator(spn, mccmnc)
    if (pnn) return formatNamedOperator(pnn, mccmnc)
    return mccmnc ? formatMccMncOperator(mccmnc) : '--'
  }

  return { index, simOperatorDisplay, formatNamedOperator, formatMccMncOperator }
}
