import * as XLSX from 'xlsx'

export interface RowData {
  [key: string]: any
}

/**
 * 桌面应用（WebView2 内）不能直接用 blob <a download> 触发系统下载，
 * 统一把生成的文件字节交给本地后端，再导航到一次性下载链接，由系统下载管理器接管。
 * 浏览器开发环境下此链路同样可用。
 */
async function saveViaServer(bytes: ArrayBuffer, filename: string) {
  const token = localStorage.getItem('ledger_token') || ''
  const res = await fetch(`/api/export/save?name=${encodeURIComponent(filename)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: bytes,
  })
  const data = await res.json().catch(() => null)
  if (res.status === 401) {
    localStorage.removeItem('ledger_token')
    localStorage.removeItem('ledger_user')
    location.href = '/login'
    throw new Error('登录已过期')
  }
  if (!res.ok || !data?.url) throw new Error(data?.message || '导出失败')
  const a = document.createElement('a')
  a.href = data.url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
}

/** 读取 xlsx / csv 文件首个工作表为对象数组（以表头为键） */
export async function readExcel(file: File): Promise<RowData[]> {
  const buf = await file.arrayBuffer()
  const wb = XLSX.read(buf, { type: 'array', cellDates: false })
  const sheetName = wb.SheetNames[0]
  if (!sheetName) return []
  const ws = wb.Sheets[sheetName]
  return XLSX.utils.sheet_to_json<RowData>(ws, { defval: '' })
}

/** 导出为 xlsx */
export function exportExcel(rows: RowData[], filename: string, sheetName = '数据') {
  const ws = XLSX.utils.json_to_sheet(rows)
  // 自适应列宽
  const headers = rows.length ? Object.keys(rows[0]!) : []
  ws['!cols'] = headers.map((h) => {
    const max = Math.max(h.length, ...rows.map((r) => String(r[h] ?? '').length))
    return { wch: Math.min(50, Math.max(10, max * 2)) }
  })
  const wb = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(wb, ws, sheetName.slice(0, 31))
  const name = filename.endsWith('.xlsx') ? filename : `${filename}.xlsx`
  const bytes = XLSX.write(wb, { type: 'array', bookType: 'xlsx' }) as ArrayBuffer
  void saveViaServer(bytes, name)
}

/** 导出为 csv（带 BOM，Excel 打开中文不乱码） */
export function exportCsv(rows: RowData[], filename: string) {
  const ws = XLSX.utils.json_to_sheet(rows)
  const csv = XLSX.utils.sheet_to_csv(ws)
  const bytes = new TextEncoder().encode('﻿' + csv)
  void saveViaServer(bytes.buffer as ArrayBuffer, filename.endsWith('.csv') ? filename : `${filename}.csv`)
}

/** 下载导入模板（一行示例） */
export function downloadTemplate(sample: RowData[], filename: string) {
  exportExcel(sample, filename)
}

/** 把中文表头的行映射为目标字段 */
export function mapHeaders(
  rows: RowData[],
  columns: { key: string; label: string; synonyms?: string[] }[],
): RowData[] {
  return rows.map((raw) => {
    const out: RowData = {}
    for (const col of columns) {
      const keys = [col.label, ...(col.synonyms ?? [])]
      for (const k of keys) {
        const hit = Object.keys(raw).find((h) => h.trim() === k)
        if (hit !== undefined) {
          out[col.key] = raw[hit]
          break
        }
      }
      if (out[col.key] === undefined) out[col.key] = ''
    }
    return out
  })
}
