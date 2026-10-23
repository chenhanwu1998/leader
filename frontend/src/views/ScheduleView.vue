<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchClasses, fetchSubjects, get, put, type SubjectInfo } from '@/api'
import { exportExcel } from '@/utils/excel'

interface ScheduleConfig {
  days: number[]
  times: string[]
  timeNotes: string[]
  breaksAfter: number[]
  cells: Record<string, string>
}

const WEEK = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
// 非学科类格子的固定底色
const EXTRA_COLORS: Record<string, string> = { 自习: '#f5f6fa', 班会: '#f0faf5' }

function defaultConfig(): ScheduleConfig {
  return {
    days: [0, 1, 2, 3, 4],
    times: ['第1节', '第2节', '第3节', '第4节', '第5节', '第6节', '第7节', '第8节'],
    timeNotes: ['08:00', '08:55', '10:05', '10:55', '14:00', '14:55', '16:00', '16:55'],
    breaksAfter: [4],
    cells: {},
  }
}

const className = ref('')
const classes = ref<string[]>([])
const subjects = ref<SubjectInfo[]>([])
const config = ref<ScheduleConfig>(defaultConfig())
const loaded = ref(false)
const saving = ref(false)
const cfgDialog = ref(false)
const cellDialog = ref(false)
const cellPos = ref<{ day: number; period: number } | null>(null)
const cellValue = ref('')

const subjectByName = computed(() => new Map(subjects.value.map((s) => [s.name, s])))
const subjectOptions = computed(() => [...new Set([...subjects.value.map((s) => s.name), '自习', '班会'])])

function cellKey(day: number, period: number) {
  return `${day}-${period}`
}

async function onClassChange() {
  if (!className.value) return
  const data = await get(`/api/schedule/${encodeURIComponent(className.value)}`)
  config.value = data.config ? { ...defaultConfig(), ...data.config } : defaultConfig()
  loaded.value = true
}

function openCell(day: number, period: number) {
  cellPos.value = { day, period }
  cellValue.value = config.value.cells[cellKey(day, period)] ?? ''
  cellDialog.value = true
}

function setCell() {
  if (!cellPos.value) return
  const key = cellKey(cellPos.value.day, cellPos.value.period)
  if (cellValue.value.trim()) config.value.cells[key] = cellValue.value.trim()
  else delete config.value.cells[key]
  cellDialog.value = false
}

async function save() {
  saving.value = true
  try {
    await put(`/api/schedule/${encodeURIComponent(className.value)}`, { config: config.value })
    ElMessage.success('课程表已保存')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

// 渲染行：节次 + 分隔条
const renderRows = computed(() => {
  const rows: ({ type: 'period'; index: number } | { type: 'break'; after: number })[] = []
  config.value.times.forEach((_t, i) => {
    rows.push({ type: 'period', index: i })
    if (config.value.breaksAfter.includes(i + 1) && i + 1 < config.value.times.length) rows.push({ type: 'break', after: i + 1 })
  })
  return rows
})

const breakLabel = (after: number) => (after === config.value.breaksAfter[0] ? '午 休' : '课 间')

function cellColor(v: string) {
  const hit = subjectByName.value.get(v)
  if (hit?.color) return hit.color
  if (EXTRA_COLORS[v]) return EXTRA_COLORS[v]
  let hash = 0
  for (const ch of v) hash = (hash * 31 + ch.charCodeAt(0)) % 360
  return `hsl(${hash} 70% 95%)`
}

function teacherOf(v: string) {
  return subjectByName.value.get(v)?.teacher ?? ''
}

function exportSchedule() {
  const header = ['节次', ...config.value.days.map((d) => WEEK[d])]
  const rows: any[] = []
  for (const r of renderRows.value) {
    if (r.type === 'break') {
      rows.push({ 节次: `— ${breakLabel(r.after)} —` })
      continue
    }
    const row: any = { 节次: `${config.value.times[r.index]}${config.value.timeNotes[r.index] ? ' ' + config.value.timeNotes[r.index] : ''}` }
    for (const d of config.value.days) {
      const v = config.value.cells[cellKey(d, r.index)] ?? ''
      const t = v ? teacherOf(v) : ''
      row[WEEK[d]!] = v ? (t ? `${v}（${t}）` : v) : ''
    }
    rows.push(row)
  }
  // json_to_sheet 需要统一表头
  const out = rows.map((r) => {
    const o: any = {}
    for (const h of header) o[h] = r[h] ?? ''
    return o
  })
  exportExcel(out, `${className.value}课程表`)
}

onMounted(async () => {
  const [cls, subs] = await Promise.all([fetchClasses(), fetchSubjects()])
  classes.value = cls
  subjects.value = subs
  if (cls.length) {
    className.value = cls[0]!
    await onClassChange()
  }
})
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">课程表</h2>
        <div class="page-sub">点击格子即可排课 / 改课，支持自定义节次与午休分隔</div>
      </div>
      <div class="toolbar">
        <el-select v-model="className" placeholder="选择班级" style="width: 160px" @change="onClassChange">
          <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
        </el-select>
        <template v-if="loaded">
          <el-button @click="cfgDialog = true"><el-icon style="margin-right: 4px"><SetUp /></el-icon>节次设置</el-button>
          <el-button @click="exportSchedule"><el-icon style="margin-right: 4px"><Download /></el-icon>导出 Excel</el-button>
          <el-button type="primary" :loading="saving" @click="save"><el-icon style="margin-right: 4px"><Check /></el-icon>保存课程表</el-button>
        </template>
      </div>
    </div>

    <div v-if="loaded" class="card">
      <div class="schedule-title">{{ className }} 课程表</div>
      <table class="schedule">
        <thead>
          <tr>
            <th class="corner">节次</th>
            <th v-for="d in config.days" :key="d">{{ WEEK[d] }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="(row, ri) in renderRows" :key="ri">
            <tr v-if="row.type === 'break'" class="break-row">
              <td :colspan="config.days.length + 1">{{ breakLabel(row.after) }}</td>
            </tr>
            <tr v-else>
              <td class="period-cell">
                <div class="period-name">{{ config.times[row.index] }}</div>
                <div class="period-time">{{ config.timeNotes[row.index] }}</div>
              </td>
              <td
                v-for="d in config.days"
                :key="d"
                class="course-cell"
                @click="openCell(d, row.index)"
              >
                <div
                  v-if="config.cells[cellKey(d, row.index)]"
                  class="course"
                  :style="{ background: cellColor(config.cells[cellKey(d, row.index)]!) }"
                >
                  <span class="course-name">{{ config.cells[cellKey(d, row.index)] }}</span>
                  <span v-if="teacherOf(config.cells[cellKey(d, row.index)]!)" class="course-teacher">
                    {{ teacherOf(config.cells[cellKey(d, row.index)]!) }}
                  </span>
                </div>
                <div v-else class="course-empty">+</div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div v-else class="card">
      <div class="empty-tip" style="padding: 60px 0">
        {{ classes.length ? '请选择班级查看课程表' : '还没有班级数据，请先在「学生花名册」中添加学生并填写班级' }}
      </div>
    </div>

    <!-- 编辑格子 -->
    <el-dialog v-model="cellDialog" title="设置课程" width="380px" v-if="cellPos">
      <div style="color: #8a93a8; font-size: 13px; margin-bottom: 10px">
        {{ WEEK[cellPos.day] }} · {{ config.times[cellPos.period] }}
      </div>
      <el-select v-model="cellValue" filterable allow-create default-first-option clearable placeholder="选择或输入课程" style="width: 100%">
        <el-option v-for="s in subjectOptions" :key="s" :value="s" :label="s" />
      </el-select>
      <template #footer>
        <el-button @click="cellValue = ''; setCell()">清空此格</el-button>
        <el-button @click="cellDialog = false">取消</el-button>
        <el-button type="primary" @click="setCell">确定</el-button>
      </template>
    </el-dialog>

    <!-- 节次设置 -->
    <el-dialog v-model="cfgDialog" title="节次与作息设置" width="560px">
      <div class="cfg-section">
        <div class="cfg-label">上课日</div>
        <el-checkbox-group v-model="config.days">
          <el-checkbox v-for="(w, i) in WEEK" :key="i" :value="i">{{ w }}</el-checkbox>
        </el-checkbox-group>
      </div>
      <div class="cfg-section">
        <div class="cfg-label">节次（名称 / 上课时间）</div>
        <div v-for="(_t, i) in config.times" :key="i" class="period-cfg">
          <el-input v-model="config.times[i]" size="small" style="width: 110px" />
          <el-time-select v-model="config.timeNotes[i]" size="small" start="06:00" step="00:05" end="22:30" placeholder="时间" style="width: 120px" />
          <el-checkbox v-if="i < config.times.length - 1" :model-value="config.breaksAfter.includes(i + 1)"
            @change="(v: boolean) => { const idx = config.breaksAfter.indexOf(i + 1); if (v && idx < 0) config.breaksAfter.push(i + 1); if (!v && idx >= 0) config.breaksAfter.splice(idx, 1) }">
            节后插分隔
          </el-checkbox>
          <el-button link type="danger" size="small" @click="config.times.splice(i, 1); config.timeNotes.splice(i, 1); config.breaksAfter = config.breaksAfter.filter((b) => b <= config.times.length);">
            删除
          </el-button>
        </div>
        <el-button size="small" plain type="primary" style="margin-top: 6px" @click="config.times.push(`第${config.times.length + 1}节`); config.timeNotes.push('')">
          <el-icon style="margin-right: 4px"><Plus /></el-icon>添加一节
        </el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="cfgDialog = false">完成（记得点保存）</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.schedule-title {
  text-align: center;
  font-size: 17px;
  font-weight: 700;
  margin-bottom: 14px;
  letter-spacing: 2px;
}

.schedule {
  width: 100%;
  border-collapse: separate;
  border-spacing: 4px;
  table-layout: fixed;
}

.schedule th {
  background: #f0f3fb;
  color: #5a6378;
  font-weight: 600;
  padding: 10px 4px;
  border-radius: 8px;
  font-size: 14px;
}

.schedule th.corner {
  width: 92px;
}

.period-cell {
  background: #fafbfe;
  border-radius: 8px;
  text-align: center;
  padding: 8px 4px;
}

.period-name {
  font-size: 13px;
  font-weight: 600;
  color: #5a6378;
}

.period-time {
  font-size: 11px;
  color: #aab1c5;
  margin-top: 2px;
}

.course-cell {
  height: 54px;
  border-radius: 8px;
  cursor: pointer;
  padding: 0;
}

.course {
  height: 100%;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1px;
  font-weight: 600;
  font-size: 14px;
  color: #3a4157;
  border: 1px solid rgba(90, 99, 120, 0.1);
  transition: transform 0.12s ease;
  overflow: hidden;
}

.course-teacher {
  font-size: 11px;
  font-weight: 400;
  color: #8a93a8;
  line-height: 1.2;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 0 4px;
}

.course-cell:hover .course {
  transform: scale(1.04);
}

.course-empty {
  height: 100%;
  border-radius: 8px;
  border: 1.5px dashed #dde2ef;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #c9d0e2;
  font-size: 16px;
  transition: all 0.12s ease;
}

.course-cell:hover .course-empty {
  border-color: #4f6bed;
  color: #4f6bed;
}

.break-row td {
  background: linear-gradient(90deg, transparent, #eef0f7 20%, #eef0f7 80%, transparent);
  color: #aab1c5;
  text-align: center;
  font-size: 12px;
  letter-spacing: 8px;
  padding: 5px 0;
  border-radius: 8px;
}

.cfg-section {
  margin-bottom: 18px;
}

.cfg-label {
  font-size: 13px;
  font-weight: 600;
  color: #5a6378;
  margin-bottom: 8px;
}

.period-cfg {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
</style>
