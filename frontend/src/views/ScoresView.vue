<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { del, fetchClasses, fetchStudents, fetchSubjects, get, post, put } from '@/api'
import { exportExcel } from '@/utils/excel'
import ImportDialog from '@/components/ImportDialog.vue'
import EChart from '@/components/EChart.vue'

const tab = ref('exams')
const exams = ref<any[]>([])
const classes = ref<string[]>([])
const subjects = ref<string[]>([])

// ---------- 考试管理 ----------
const examDialog = ref(false)
const examForm = ref<any>({})
const examEditing = ref<any>(null)
const examSaving = ref(false)

function openExamAdd() {
  examEditing.value = null
  examForm.value = { name: '', exam_date: new Date().toISOString().slice(0, 10), term: '上学期', type: '期中', class_name: '', remark: '' }
  examDialog.value = true
}
function openExamEdit(row: any) {
  examEditing.value = row
  examForm.value = { ...row }
  examDialog.value = true
}
async function saveExam() {
  if (!examForm.value.name?.trim()) return ElMessage.warning('请填写考试名称')
  examSaving.value = true
  try {
    if (examEditing.value) await put(`/api/scores/exams/${examEditing.value.id}`, examForm.value)
    else await post('/api/scores/exams', examForm.value)
    ElMessage.success('已保存')
    examDialog.value = false
    loadExams()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    examSaving.value = false
  }
}
async function removeExam(row: any) {
  await ElMessageBox.confirm(`确定删除「${row.name}」？该考试的全部成绩会一并删除。`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  await del(`/api/scores/exams/${row.id}`)
  ElMessage.success('已删除')
  loadExams()
}

async function loadExams() {
  exams.value = await get('/api/scores/exams')
}

// ---------- 成绩录入 ----------
const entryExamId = ref<number | null>(null)
const entryClass = ref('')
const entrySubjects = ref<string[]>([])
const fullScore = ref(100)
const classStudents = ref<any[]>([])
const entryMatrix = ref<any[]>([]) // {student_id, name, ..., scores:{}, dirty:{}}
const entryLoading = ref(false)
const importVisible = ref(false)

const entryExam = computed(() => exams.value.find((e) => e.id === entryExamId.value))

async function loadEntry() {
  if (!entryExamId.value || !entryClass.value) {
    entryMatrix.value = []
    return
  }
  entryLoading.value = true
  try {
    const [students, scores] = await Promise.all([
      fetchStudents(entryClass.value),
      get(`/api/scores?exam_id=${entryExamId.value}`),
    ])
    classStudents.value = students
    // 恢复本场考试的科目勾选记忆
    const savedKey = `exam_subjects_${entryExamId.value}`
    const saved = localStorage.getItem(savedKey)
    if (saved) entrySubjects.value = JSON.parse(saved)
    else if (!entrySubjects.value.length) entrySubjects.value = subjects.value.slice(0, 3)
    entryMatrix.value = students.map((st: any) => {
      const cell: any = { student_id: st.id, student_no: st.student_no, name: st.name, class_name: st.class_name, scores: {} as Record<string, any>, touched: {} as Record<string, boolean> }
      for (const s of scores.filter((x: any) => x.student_id === st.id)) {
        cell.scores[s.subject] = s.score
      }
      return cell
    })
  } finally {
    entryLoading.value = false
  }
}

function persistSubjectChoice() {
  if (entryExamId.value) localStorage.setItem(`exam_subjects_${entryExamId.value}`, JSON.stringify(entrySubjects.value))
}

function totalOf(row: any) {
  return entrySubjects.value.reduce((a, s) => a + (Number(row.scores[s]) || 0), 0)
}

async function saveEntry() {
  const records: any[] = []
  for (const row of entryMatrix.value) {
    for (const s of entrySubjects.value) {
      const v = row.scores[s]
      if (v === '' || v === null || v === undefined) continue
      records.push({ student_id: row.student_id, subject: s, score: Number(v), full_score: fullScore.value })
    }
  }
  if (!records.length) return ElMessage.warning('没有可保存的成绩，请先录入')
  await post('/api/scores/bulk', { exam_id: entryExamId.value, records })
  ElMessage.success(`已保存 ${records.length} 条成绩`)
}

const importScoreColumns = computed(() => [
  { key: 'name', label: '姓名', required: true },
  ...entrySubjects.value.map((s) => ({ key: s, label: s })),
])
const importScoreSample = computed(() => {
  const row: any = { 姓名: '张三' }
  entrySubjects.value.forEach((s) => (row[s] = 90))
  return [row]
})

async function onImportScores(records: any[]) {
  const byName = new Map(classStudents.value.map((s: any) => [s.name, s]))
  let matched = 0
  const missed: string[] = []
  const payload: any[] = []
  for (const r of records) {
    const stu = byName.get(String(r.name).trim())
    if (!stu) {
      missed.push(r.name)
      continue
    }
    for (const s of entrySubjects.value) {
      const v = r[s]
      if (v === '' || v === undefined) continue
      payload.push({ student_id: stu.id, subject: s, score: Number(v), full_score: fullScore.value })
    }
    matched++
  }
  if (!payload.length) return ElMessage.warning(`没有匹配到可导入的成绩${missed.length ? `（未找到学生：${missed.slice(0, 5).join('、')}）` : ''}`)
  await post('/api/scores/bulk', { exam_id: entryExamId.value, records: payload })
  ElMessage.success(`已导入 ${matched} 名学生的成绩${missed.length ? `，${missed.length} 人未匹配：${missed.slice(0, 5).join('、')}` : ''}`)
  loadEntry()
}

function exportMatrix() {
  const rows = entryMatrix.value.map((r, i) => {
    const out: any = { 名次: i + 1, 学号: r.student_no, 姓名: r.name, 班级: r.class_name }
    for (const s of entrySubjects.value) out[s] = r.scores[s] ?? ''
    out['总分'] = entrySubjects.value.every((s) => r.scores[s] !== undefined && r.scores[s] !== '') ? totalOf(r) : ''
    return out
  })
  exportExcel(rows, `${entryExam.value?.name ?? '成绩'}_${entryClass.value}`)
}

const sortedMatrix = computed(() => [...entryMatrix.value].sort((a, b) => totalOf(b) - totalOf(a)))

// ---------- 统计分析 ----------
async function loadStats() {
  if (!entryExamId.value) return
  statsData.value = await get(`/api/stats/scores?exam_id=${entryExamId.value}&full_score=${fullScore.value}`)
}
const statsData = ref<any>(null)

const avgOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  legend: { bottom: 0 },
  grid: { left: 6, right: 6, top: 42, bottom: 30, containLabel: true },
  xAxis: { type: 'category', data: (statsData.value?.subjects ?? []).map((s: any) => s.k), axisTick: { show: false } },
  yAxis: [
    { type: 'value', name: '平均分', splitLine: { lineStyle: { color: '#eef0f6' } } },
    { type: 'value', name: '及格率%', max: 100, splitLine: { show: false } },
  ],
  series: [
    { name: '平均分', type: 'bar', barWidth: 24, itemStyle: { borderRadius: [6, 6, 0, 0], color: '#4f6bed' }, data: (statsData.value?.subjects ?? []).map((s: any) => s.avg) },
    { name: '及格率', type: 'line', yAxisIndex: 1, smooth: true, itemStyle: { color: '#f2a545' }, data: (statsData.value?.subjects ?? []).map((s: any) => s.passRate) },
  ],
}))

const distOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: 10, right: 16, top: 24, bottom: 0, containLabel: true },
  xAxis: { type: 'category', data: (statsData.value?.dist ?? []).map((d: any) => d.range), axisTick: { show: false }, name: '总分' },
  yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: '#eef0f6' } } },
  series: [{ type: 'bar', barWidth: 20, itemStyle: { borderRadius: [6, 6, 0, 0], color: '#7c5bf5' }, data: (statsData.value?.dist ?? []).map((d: any) => d.v) }],
}))

const bandOption = computed(() => {
  const rows = statsData.value?.bandStat ?? []
  const colors = ['#22b8a5', '#5aa9e6', '#f2a545', '#ee6b6e']
  const kinds = ['优秀', '良好', '及格', '不及格']
  return {
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0 },
    grid: { left: 10, right: 16, top: 24, bottom: 30, containLabel: true },
    xAxis: { type: 'category', data: rows.map((r: any) => r.subject), axisTick: { show: false } },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: '#eef0f6' } } },
    series: kinds.map((k, i) => ({
      name: k, type: 'bar', stack: 'total', barWidth: 26,
      itemStyle: { color: colors[i] },
      data: rows.map((r: any) => r[k]),
    })),
  }
})

onMounted(async () => {
  try {
    const [ex, cls, subs] = await Promise.all([get('/api/scores/exams'), fetchClasses(), fetchSubjects()])
    exams.value = ex
    classes.value = cls
    subjects.value = subs.map((s) => s.name)
  } catch (e: any) {
    ElMessage.error(e.message)
  }
})
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">成绩管理</h2>
        <div class="page-sub">登记每场考试的各科成绩，自动汇总排名并生成统计分析</div>
      </div>
    </div>

    <el-tabs v-model="tab" class="card">
      <!-- ============ 考试管理 ============ -->
      <el-tab-pane label="🗂 考试管理" name="exams">
        <div class="toolbar" style="margin-bottom: 14px">
          <div style="flex: 1; color: #98a0b5; font-size: 13px">共 {{ exams.length }} 场考试</div>
          <el-button type="primary" @click="openExamAdd"><el-icon style="margin-right: 4px"><Plus /></el-icon>新建考试</el-button>
        </div>
        <el-table :data="exams" stripe>
          <el-table-column prop="name" label="考试名称" min-width="180" />
          <el-table-column prop="exam_date" label="日期" width="115" />
          <el-table-column prop="term" label="学期" width="90" />
          <el-table-column prop="type" label="类型" width="90">
            <template #default="{ row }"><el-tag size="small" effect="light">{{ row.type }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="class_name" label="班级" width="120" />
          <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="((entryExamId = row.id), (tab = 'entry'))">录入成绩</el-button>
              <el-button link type="primary" @click="openExamEdit(row)">编辑</el-button>
              <el-button link type="danger" @click="removeExam(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><div class="empty-tip">暂无考试记录，点击「新建考试」开始</div></template>
        </el-table>
      </el-tab-pane>

      <!-- ============ 成绩录入 ============ -->
      <el-tab-pane label="✍️ 成绩录入" name="entry" lazy>
        <div class="toolbar" style="margin-bottom: 14px">
          <el-select v-model="entryExamId" placeholder="选择考试" style="width: 220px" @change="loadEntry">
            <el-option v-for="e in exams" :key="e.id" :value="e.id" :label="`${e.name}（${e.exam_date || '未定'}）`" />
          </el-select>
          <el-select v-model="entryClass" placeholder="选择班级" style="width: 160px" @change="loadEntry">
            <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
          </el-select>
          <el-select v-model="entrySubjects" multiple collapse-tags collapse-tags-tooltip placeholder="选择科目" style="width: 260px" @change="persistSubjectChoice">
            <el-option v-for="s in subjects" :key="s" :value="s" :label="s" />
          </el-select>
          <div style="display: flex; align-items: center; gap: 6px; color: #8a93a8; font-size: 13px">
            满分 <el-input-number v-model="fullScore" :min="10" :max="300" :step="10" size="small" style="width: 90px" />
          </div>
          <div style="flex: 1" />
          <template v-if="entryMatrix.length">
            <el-button @click="importVisible = true"><el-icon style="margin-right: 4px"><Upload /></el-icon>导入</el-button>
            <el-button @click="exportMatrix"><el-icon style="margin-right: 4px"><Download /></el-icon>导出 Excel</el-button>
            <el-button type="primary" @click="saveEntry"><el-icon style="margin-right: 4px"><Check /></el-icon>保存成绩</el-button>
          </template>
        </div>

        <template v-if="entryMatrix.length">
          <el-table :data="sortedMatrix" v-loading="entryLoading" max-height="560" stripe>
            <el-table-column label="名次" width="66" fixed>
              <template #default="{ $index }">
                <span :style="{ fontWeight: $index < 3 ? 700 : 400, color: ['#d4a017', '#8a93a8', '#b06c3b'][$index] ?? '#5a6378' }">{{ $index + 1 }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="student_no" label="学号" width="80" fixed />
            <el-table-column prop="name" label="姓名" width="96" fixed>
              <template #default="{ row }"><b>{{ row.name }}</b></template>
            </el-table-column>
            <el-table-column v-for="s in entrySubjects" :key="s" :label="s" width="96">
              <template #default="{ row }">
                <el-input v-model="row.scores[s]" class="cell-input" size="small" placeholder="—" />
              </template>
            </el-table-column>
            <el-table-column label="总分" width="80" fixed="right">
              <template #default="{ row }"><b style="color: #4f6bed">{{ totalOf(row) }}</b></template>
            </el-table-column>
          </el-table>
          <div style="color: #98a0b5; font-size: 12.5px; margin-top: 8px">💡 修改后直接点「保存成绩」即可批量提交；留空的科目不会保存。</div>
        </template>
        <div v-else class="empty-tip" style="padding: 60px 0">
          请先选择「考试」和「班级」<span v-if="!entryClass">，班级来自学生花名册</span>
        </div>
      </el-tab-pane>

      <!-- ============ 统计分析 ============ -->
      <el-tab-pane label="📊 统计分析" name="stats" lazy>
        <div class="toolbar" style="margin-bottom: 14px">
          <el-select v-model="entryExamId" placeholder="选择考试" style="width: 240px" @change="loadStats">
            <el-option v-for="e in exams" :key="e.id" :value="e.id" :label="`${e.name}（${e.exam_date || '未定'}）`" />
          </el-select>
          <el-button type="primary" plain @click="loadStats" :disabled="!entryExamId">刷新统计</el-button>
        </div>
        <template v-if="statsData && statsData.subjects.length">
          <div class="stat-cards" style="margin-bottom: 16px">
            <div class="stat-card" v-for="s in statsData.subjects" :key="s.k">
              <div>
                <div class="stat-num">{{ s.avg }}<span style="font-size: 13px; color: #98a0b5; font-weight: 400"> 分</span></div>
                <div class="stat-label">{{ s.k }}（最高 {{ s.max }} / 及格率 {{ s.passRate }}%）</div>
              </div>
            </div>
          </div>
          <div class="grid-cards" style="grid-template-columns: repeat(auto-fit, minmax(340px, 1fr))">
            <div class="card" style="box-shadow: none">
              <div class="card-title">单科平均分与及格率</div>
              <EChart :option="avgOption" height="280px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">总分人数分布</div>
              <EChart :option="distOption" height="280px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">单科等级分布（优秀/良好/及格/不及格）</div>
              <EChart :option="bandOption" height="280px" />
            </div>
          </div>
        </template>
        <div v-else class="empty-tip" style="padding: 60px 0">选择一场已有成绩的考试后查看统计</div>
      </el-tab-pane>
    </el-tabs>

    <!-- 考试编辑 -->
    <el-dialog v-model="examDialog" :title="examEditing ? '编辑考试' : '新建考试'" width="520px">
      <el-form :model="examForm" label-width="84px">
        <el-form-item label="考试名称" required><el-input v-model="examForm.name" placeholder="如：2026 秋季期中考试" /></el-form-item>
        <el-form-item label="日期"><el-date-picker v-model="examForm.exam_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" /></el-form-item>
        <el-form-item label="学期">
          <el-select v-model="examForm.term" style="width: 100%">
            <el-option value="上学期" label="上学期" />
            <el-option value="下学期" label="下学期" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="examForm.type">
            <el-radio v-for="t in ['单元测', '月考', '期中', '期末']" :key="t" :value="t">{{ t }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="班级">
          <el-select v-model="examForm.class_name" clearable style="width: 100%">
            <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="examForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="examDialog = false">取消</el-button>
        <el-button type="primary" :loading="examSaving" @click="saveExam">保存</el-button>
      </template>
    </el-dialog>

    <ImportDialog v-model="importVisible" title="成绩" :columns="importScoreColumns" :sample="importScoreSample"
      tip="按「姓名 + 勾选的科目」列导入，姓名需与花名册一致；只导入选中科目的成绩" @confirm="onImportScores" />
  </div>
</template>
