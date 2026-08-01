// GSM 03.38 基本表与扩展表：扩展表里的字符要占两个 septet
const GSM7_BASIC_CHARS = new Set(Array.from(
  `@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞ !"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà`
))
const GSM7_EXT_CHARS = new Set(Array.from('^{}\\[~]|€'))

export type SegmentEstimate = {
  encoding: 'GSM7' | 'UCS2'
  parts: number
  units: number
  unitName: string
}

/** 估算短信编码与分段数：只要出现一个非 GSM7 字符，整条就退化为 UCS2 */
export function estimateSegments(text: string): SegmentEstimate {
  const raw = String(text || '')
  let gsm7Units = 0
  let isGSM7 = true
  for (const ch of Array.from(raw)) {
    if (GSM7_BASIC_CHARS.has(ch)) { gsm7Units += 1; continue }
    if (GSM7_EXT_CHARS.has(ch)) { gsm7Units += 2; continue }
    isGSM7 = false
    break
  }
  if (isGSM7) {
    const parts = gsm7Units <= 160 ? 1 : Math.ceil(gsm7Units / 153)
    return { encoding: 'GSM7', parts, units: gsm7Units, unitName: 'septets' }
  }
  const ucs2Units = Array.from(raw).length
  const parts = ucs2Units <= 70 ? 1 : Math.ceil(ucs2Units / 67)
  return { encoding: 'UCS2', parts, units: ucs2Units, unitName: 'chars' }
}
