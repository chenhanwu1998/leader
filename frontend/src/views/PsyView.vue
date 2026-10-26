<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { get, post, put, del, fetchClasses, downloadFile } from '@/api'

const router = useRouter()
const RISK_LEVELS = ['常规关注', '重点关注', '高危预警']
const riskType = (l: string) => (l === '高危预警' ? 'danger' : l === '重点关注' ? 'warning' : 'info')

interface ScaleOption {
  label: string
  score: number
}
interface ScaleThreshold {
  min: number
  level: string
  note: string
}
interface Scale {
  id: string
  name: string
  hint: string
  builtin: boolean
  options: ScaleOption[]
  items: string[]
  reverse: number[]
  score_mode: 'sum' | 'std125'
  thresholds: ScaleThreshold[]
  flag_item: number
  flag_min: number
  flag_note: string
}

// —— 总览 ——
const stats = ref<{ by_risk: { risk_level: string; n: number }[]; recent_assessments: any[]; flagged: number; sessions_this_month: number } | null>(null)
async function loadStats() {
  stats.value = await get('/api/psy/stats')
}
const riskCount = computed(() => {
  const m: Record<string, number> = {}
  for (const r of stats.value?.by_risk ?? []) m[r.risk_level] = r.n
  return m
})

// —— 档案列表 ——
const tab = ref('overview')
const profiles = ref<any[]>([])
const loading = ref(false)
const filter = reactive({ keyword: '', risk_level: '', class_name: '' })
const classOptions = ref<string[]>([])

async function loadProfiles() {
  loading.value = true
  try {
    const qs = new URLSearchParams()
    if (filter.keyword) qs.set('keyword', filter.keyword)
    if (filter.risk_level) qs.set('risk_level', filter.risk_level)
    if (filter.class_name) qs.set('class_name', filter.class_name)
    profiles.value = await get(`/api/psy/profiles?${qs}`)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  loadStats()
  loadProfiles()
  loadFollowups()
  loadScales()
  try {
    classOptions.value = await fetchClasses()
  } catch {
    /* 班级可空 */
  }
})

// —— 新建 / 编辑档案 ——
const profileDialog = ref(false)
const profileForm = reactive({ id: 0, student_name: '', class_name: '', gender: '男', risk_level: '常规关注', tags: '', note: '' })
const savingProfile = ref(false)

function openProfileDialog(p?: any) {
  Object.assign(profileForm, p ?? { id: 0, student_name: '', class_name: '', gender: '男', risk_level: '常规关注', tags: '', note: '' })
  profileDialog.value = true
}

async function saveProfile() {
  if (!profileForm.student_name.trim()) {
    ElMessage.warning('请填写学生姓名')
    return
  }
  savingProfile.value = true
  const body = {
    student_name: profileForm.student_name.trim(),
    class_name: profileForm.class_name,
    gender: profileForm.gender,
    risk_level: profileForm.risk_level,
    tags: profileForm.tags,
    note: profileForm.note,
  }
  try {
    if (profileForm.id) await put(`/api/psy/profiles/${profileForm.id}`, body)
    else await post('/api/psy/profiles', body)
    ElMessage.success('已保存')
    profileDialog.value = false
    loadProfiles()
    loadStats()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    savingProfile.value = false
  }
}

async function removeProfile(p: any) {
  try {
    await ElMessageBox.confirm(`删除「${p.student_name}」的心理档案会同时删除其辅导记录与测评历史，确定？`, '删除档案', { type: 'warning' })
  } catch {
    return
  }
  await del(`/api/psy/profiles/${p.id}`)
  ElMessage.success('已删除')
  loadProfiles()
  loadStats()
}

// —— 档案详情抽屉（辅导记录 + 测评历史）——
const drawer = ref(false)
const detail = ref<any>(null)
async function openDetail(p: any) {
  detail.value = await get(`/api/psy/profiles/${p.id}`)
  drawer.value = true
  Object.assign(sessionForm, { session_date: new Date().toISOString().slice(0, 10), mode: '面谈', topic: '', content: '', next_follow: '' })
}

const sessionForm = reactive({ session_date: '', mode: '面谈', topic: '', content: '', next_follow: '' })
const savingSession = ref(false)
async function addSession() {
  if (!sessionForm.content.trim()) {
    ElMessage.warning('请填写辅导内容')
    return
  }
  savingSession.value = true
  try {
    await post(`/api/psy/profiles/${detail.value.id}/sessions`, { ...sessionForm })
    ElMessage.success('辅导记录已添加')
    detail.value = await get(`/api/psy/profiles/${detail.value.id}`)
    Object.assign(sessionForm, { topic: '', content: '', next_follow: '' })
    loadFollowups()
    loadStats()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    savingSession.value = false
  }
}

async function removeSession(s: any) {
  await del(`/api/psy/sessions/${s.id}`)
  detail.value = await get(`/api/psy/profiles/${detail.value.id}`)
}

async function removeAssessment(a: any) {
  await del(`/api/psy/assessments/${a.id}`)
  detail.value = await get(`/api/psy/profiles/${detail.value.id}`)
}

// —— 量表数据 ——
const scales = ref<Scale[]>([])
async function loadScales() {
  scales.value = await get('/api/psy/scales')
}

// —— 心理测评 tab ——
const quizProfileId = ref<number | ''>('')

function startQuizFor(s: Scale) {
  if (!quizProfileId.value) {
    ElMessage.warning('请先选择要测评的学生（来自心理档案）')
    return
  }
  const p = profiles.value.find((x) => x.id === quizProfileId.value)
  if (!p) return
  startQuiz(p, s.id)
}

// —— 测评问卷 ——
const quizDialog = ref(false)
const quiz = reactive({ profile: null as any, scale: null as Scale | null, answers: [] as number[], result: null as any })

function startQuiz(profile: any, scaleId = '') {
  quiz.profile = profile
  quiz.scale = scales.value.find((s) => s.id === scaleId) ?? null
  quiz.answers = quiz.scale ? Array(quiz.scale.items.length).fill(-1) : []
  quiz.result = null
  quizDialog.value = true
}

function pickScale(id: string) {
  quiz.scale = scales.value.find((s) => s.id === id) ?? null
  quiz.answers = quiz.scale ? Array(quiz.scale.items.length).fill(-1) : []
  quiz.result = null
}

const quizProgress = computed(() => (quiz.scale ? quiz.answers.filter((a) => a >= 0).length : 0))
const submittingQuiz = ref(false)

// 结果解释：得分命中的分档 note + 触发预警题时的说明
const resultNote = computed(() => {
  if (!quiz.result || !quiz.scale) return ''
  const parts: string[] = []
  const hit = quiz.scale.thresholds.find((t) => quiz.result.score >= t.min)
  if (hit?.note?.trim()) parts.push(hit.note.trim())
  if (quiz.result.flagged && quiz.scale.flag_note?.trim()) parts.push(quiz.scale.flag_note.trim())
  return parts.join('\n')
})

async function submitQuiz() {
  if (!quiz.scale || quiz.answers.some((a) => a < 0)) {
    ElMessage.warning('还有题目未作答')
    return
  }
  submittingQuiz.value = true
  try {
    const r = await post(`/api/psy/profiles/${quiz.profile.id}/assessments`, { scale: quiz.scale.id, answers: quiz.answers })
    quiz.result = r
    if (detail.value?.id === quiz.profile.id) detail.value = await get(`/api/psy/profiles/${detail.value.id}`)
    loadStats()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    submittingQuiz.value = false
  }
}

// —— 问卷导出 / 打印 ——
async function exportScaleDocx(s: Scale) {
  try {
    await downloadFile(`/api/psy/scales/${encodeURIComponent(s.id)}/export.docx`, `${s.name}-问卷.docx`)
  } catch (e: any) {
    ElMessage.error(e.message || '导出失败')
  }
}

function escHtml(t: string) {
  return t.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!)
}

function buildPrintHtml(s: Scale) {
  const rows = s.items
    .map(
      (item, i) => `
      <div class="item">
        <div class="q">${i + 1}. ${escHtml(item)}</div>
        <div class="opts">${s.options.map((o) => `<span class="opt">□ ${escHtml(o.label)}</span>`).join('')}</div>
      </div>`
    )
    .join('')
  return `<!DOCTYPE html><html><head><meta charset="utf-8"><title>${escHtml(s.name)}</title>
  <style>
    @page { margin: 18mm; }
    body { font-family: 'Microsoft YaHei', 'SimSun', sans-serif; color: #222; font-size: 13px; line-height: 1.7; }
    h1 { text-align: center; font-size: 20px; margin: 0 0 8px; }
    .hint { background: #f6f7fb; border-radius: 6px; padding: 8px 12px; margin-bottom: 10px; color: #445; }
    .meta { display: flex; gap: 36px; margin: 10px 0 14px; border-bottom: 1px solid #ccc; padding-bottom: 10px; }
    .item { margin: 12px 0; page-break-inside: avoid; border-bottom: 1px dashed #eee; padding-bottom: 8px; }
    .q { font-weight: 600; margin-bottom: 4px; }
    .opts { display: flex; flex-wrap: wrap; gap: 18px; color: #333; }
    .foot { margin-top: 18px; color: #999; font-size: 11px; }
  </style></head><body>
  <h1>${escHtml(s.name)}</h1>
  <div class="hint">指导语：${escHtml(s.hint || '请根据最近一段时间的实际情况作答，答案没有对错之分。')}</div>
  <div class="meta"><span>班级：＿＿＿＿＿＿＿＿</span><span>姓名：＿＿＿＿＿＿＿＿</span><span>日期：＿＿＿＿＿＿＿＿</span></div>
  ${rows}
  <div class="foot">本问卷仅用于校园心理健康筛查参考，不能替代专业医疗诊断。</div>
  </body></html>`
}

function printScale(s: Scale) {
  // 隐藏 iframe 承载打印页，浏览器与 WebView2 都能正常弹出系统打印对话框
  const iframe = document.createElement('iframe')
  iframe.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0'
  document.body.appendChild(iframe)
  const doc = iframe.contentWindow?.document
  if (!doc) {
    iframe.remove()
    return ElMessage.error('打印窗口创建失败')
  }
  doc.open()
  doc.write(buildPrintHtml(s))
  doc.close()
  iframe.contentWindow?.focus()
  setTimeout(() => {
    iframe.contentWindow?.print()
    setTimeout(() => iframe.remove(), 1000)
  }, 200)
}

// —— 量表管理 ——
const scaleDialog = ref(false)
const savingScales = ref(false)
const scaleForm = reactive({
  editing: false,
  builtin: false,
  id: '',
  name: '',
  hint: '',
  itemsText: '',
  options: [] as ScaleOption[],
  reverseText: '',
  score_mode: 'sum' as 'sum' | 'std125',
  thresholds: [] as ScaleThreshold[],
  flag_item: 0,
  flag_min: 1,
  flag_note: '',
})

function openScaleDialog(s?: Scale) {
  if (s) {
    Object.assign(scaleForm, {
      editing: true,
      builtin: s.builtin,
      id: s.id,
      name: s.name,
      hint: s.hint,
      itemsText: s.items.join('\n'),
      options: s.options.map((o) => ({ ...o })),
      reverseText: s.reverse.map((i) => i + 1).join(', '),
      score_mode: s.score_mode,
      thresholds: s.thresholds.map((t) => ({ ...t })),
      flag_item: s.flag_item,
      flag_min: s.flag_min,
      flag_note: s.flag_note,
    })
  } else {
    Object.assign(scaleForm, {
      editing: false,
      builtin: false,
      id: '',
      name: '',
      hint: '请根据最近一段时间的实际情况作答，答案没有对错之分。',
      itemsText: '',
      options: [
        { label: '完全不会', score: 0 },
        { label: '偶尔', score: 1 },
        { label: '经常', score: 2 },
        { label: '几乎每天', score: 3 },
      ],
      reverseText: '',
      score_mode: 'sum' as const,
      thresholds: [{ min: 0, level: '正常范围', note: '' }],
      flag_item: 0,
      flag_min: 1,
      flag_note: '',
    })
  }
  scaleDialog.value = true
}

function parseReverse(text: string, itemCount: number) {
  const seen = new Set<number>()
  for (const part of text.split(/[,，、\s]+/).filter(Boolean)) {
    const n = Number(part)
    if (Number.isInteger(n) && n >= 1 && n <= itemCount) seen.add(n - 1)
  }
  return [...seen].sort((a, b) => a - b)
}

async function saveScaleDialog() {
  const items = scaleForm.itemsText
    .split('\n')
    .map((t) => t.trim())
    .filter(Boolean)
  if (!scaleForm.id.trim() || !/^[A-Za-z0-9_-]{1,32}$/.test(scaleForm.id.trim())) return ElMessage.warning('量表编号须为 1-32 位字母/数字/中划线/下划线')
  if (!scaleForm.name.trim()) return ElMessage.warning('请填写量表名称')
  if (!items.length) return ElMessage.warning('请至少填写一道题目（每行一题）')
  if (scaleForm.options.length < 2) return ElMessage.warning('请至少保留两个选项')
  if (scaleForm.options.some((o) => !o.label.trim())) return ElMessage.warning('选项文字不能为空')
  if (!scaleForm.thresholds.length || scaleForm.thresholds.some((t) => !t.level.trim())) return ElMessage.warning('请至少保留一档结果分档，且等级名称不能为空')

  const scale: Scale = {
    id: scaleForm.id.trim(),
    name: scaleForm.name.trim(),
    hint: scaleForm.hint.trim(),
    builtin: scaleForm.builtin,
    options: scaleForm.options.map((o) => ({ label: o.label.trim(), score: Number(o.score) || 0 })),
    items,
    reverse: parseReverse(scaleForm.reverseText, items.length),
    score_mode: scaleForm.score_mode,
    thresholds: [...scaleForm.thresholds].map((t) => ({ min: Number(t.min) || 0, level: t.level.trim(), note: (t.note ?? '').trim() })).sort((a, b) => b.min - a.min),
    flag_item: Number(scaleForm.flag_item) || 0,
    flag_min: Number(scaleForm.flag_min) || 0,
    flag_note: scaleForm.flag_note.trim(),
  }
  const list = scaleForm.editing ? scales.value.map((s) => (s.id === scale.id ? scale : s)) : [...scales.value.filter((s) => s.id !== scale.id), scale]
  savingScales.value = true
  try {
    scales.value = await put('/api/psy/scales', { scales: list })
    ElMessage.success('量表已保存')
    scaleDialog.value = false
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    savingScales.value = false
  }
}

async function removeScale(s: Scale) {
  try {
    await ElMessageBox.confirm(`删除量表「${s.name}」？已有测评记录的历史结果不受影响。`, '删除量表', { type: 'warning' })
  } catch {
    return
  }
  try {
    scales.value = await put('/api/psy/scales', { scales: scales.value.filter((x) => x.id !== s.id) })
    ElMessage.success('已删除')
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

async function resetScales() {
  try {
    await ElMessageBox.confirm('将清除所有自定义量表与修改，恢复为内置的 PHQ-9 / GAD-7 / SAS / SDS，确定？', '恢复默认', { type: 'warning' })
  } catch {
    return
  }
  scales.value = await post('/api/psy/scales/reset', {})
  ElMessage.success('已恢复内置默认量表')
}

// —— 跟进提醒 ——
const followups = ref<any[]>([])
async function loadFollowups() {
  followups.value = await get('/api/psy/followups')
}

function goAgent(p: any) {
  router.push('/agent')
}
</script>

<template>
  <div class="psy-page">
    <el-tabs v-model="tab" class="tabs">
      <!-- 总览 -->
      <el-tab-pane label="总览" name="overview">
        <div class="stat-cards">
          <el-card shadow="never" class="stat danger"><div class="num">{{ riskCount['高危预警'] ?? 0 }}</div><div class="label">高危预警</div></el-card>
          <el-card shadow="never" class="stat warning"><div class="num">{{ riskCount['重点关注'] ?? 0 }}</div><div class="label">重点关注</div></el-card>
          <el-card shadow="never" class="stat info"><div class="num">{{ riskCount['常规关注'] ?? 0 }}</div><div class="label">常规关注</div></el-card>
          <el-card shadow="never" class="stat"><div class="num">{{ stats?.sessions_this_month ?? 0 }}</div><div class="label">本月辅导次数</div></el-card>
          <el-card shadow="never" class="stat flag"><div class="num">{{ stats?.flagged ?? 0 }}</div><div class="label">量表预警标记</div></el-card>
        </div>

        <div class="overview-grid">
          <el-card shadow="never" class="card">
            <template #header>最近测评</template>
            <el-table :data="stats?.recent_assessments ?? []" size="small">
              <el-table-column prop="student_name" label="学生" width="100" />
              <el-table-column prop="class_name" label="班级" width="110" />
              <el-table-column prop="scale" label="量表" width="90" />
              <el-table-column prop="score" label="得分" width="70" />
              <el-table-column prop="level" label="结果" min-width="110" />
              <el-table-column label="预警" width="70">
                <template #default="{ row }"><el-tag v-if="row.flagged" type="danger" size="small">预警</el-tag></template>
              </el-table-column>
              <el-table-column prop="created_at" label="时间" width="150" />
              <template #empty><span class="dim">暂无测评记录</span></template>
            </el-table>
          </el-card>

          <el-card shadow="never" class="card">
            <template #header>使用指引</template>
            <ol class="guide">
              <li>在「心理档案」为需要关注的学生建档，标注关注等级。</li>
              <li>在「心理测评」选好学生与量表，平台内直接作答，系统自动计分并给出结果解释。</li>
              <li>需要纸质量表时，到「量表管理」导出 DOCX 或直接打印空白问卷。</li>
              <li>每次谈话或辅导后记一条「辅导记录」，可设置下次跟进日期，到期会在「跟进提醒」出现。</li>
              <li>拿不准怎么应对时，到「智能助手」打开心理模式提问，会结合校园心理健康知识库回答。</li>
              <li>量表得分出现预警标记（如 PHQ-9 第9题 ≥ 1 分）时，请马上进行安全谈话并联系监护人与专业人员。</li>
            </ol>
          </el-card>
        </div>
      </el-tab-pane>

      <!-- 心理档案 -->
      <el-tab-pane label="心理档案" name="profiles">
        <div class="toolbar">
          <el-input v-model="filter.keyword" placeholder="搜索姓名 / 标签" clearable style="width: 200px" @change="loadProfiles" />
          <el-select v-model="filter.class_name" placeholder="班级" clearable style="width: 150px" @change="loadProfiles">
            <el-option v-for="c in classOptions" :key="c" :value="c" :label="c" />
          </el-select>
          <el-select v-model="filter.risk_level" placeholder="关注等级" clearable style="width: 130px" @change="loadProfiles">
            <el-option v-for="l in RISK_LEVELS" :key="l" :value="l" :label="l" />
          </el-select>
          <el-button :loading="loading" @click="loadProfiles">查询</el-button>
          <div class="spacer" />
          <el-button type="primary" @click="openProfileDialog()"><el-icon><Plus /></el-icon>新建档案</el-button>
        </div>

        <el-table :data="profiles" v-loading="loading" stripe>
          <el-table-column prop="student_name" label="姓名" width="110" />
          <el-table-column prop="class_name" label="班级" width="120" />
          <el-table-column prop="gender" label="性别" width="70" />
          <el-table-column label="关注等级" width="110">
            <template #default="{ row }"><el-tag :type="riskType(row.risk_level)" size="small">{{ row.risk_level }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="tags" label="标签" min-width="130">
            <template #default="{ row }">
              <el-tag v-for="t in row.tags.split(/[,，]/).filter(Boolean)" :key="t" size="small" effect="plain" class="tag">{{ t.trim() }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="session_count" label="辅导次数" width="90" />
          <el-table-column prop="updated_at" label="最近更新" width="150" />
          <el-table-column label="操作" width="270" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openDetail(row)">详情</el-button>
              <el-button size="small" text type="warning" @click="startQuiz(row)">测评</el-button>
              <el-button size="small" text @click="openProfileDialog(row)">编辑</el-button>
              <el-button size="small" text type="danger" @click="removeProfile(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><span class="dim">还没有心理档案，点击右上角「新建档案」开始</span></template>
        </el-table>
      </el-tab-pane>

      <!-- 心理测评 -->
      <el-tab-pane label="心理测评" name="assess">
        <div class="toolbar">
          <span class="dim">测评学生：</span>
          <el-select v-model="quizProfileId" placeholder="从心理档案中选择学生" filterable style="width: 260px">
            <el-option v-for="p in profiles" :key="p.id" :value="p.id" :label="`${p.student_name}（${p.class_name || '未填班级'}）`" />
          </el-select>
          <span v-if="!profiles.length" class="dim">还没有心理档案，请先在「心理档案」页建档</span>
        </div>

        <div class="scale-grid">
          <el-card v-for="s in scales" :key="s.id" shadow="hover" class="scale-card">
            <div class="sc-head">
              <b>{{ s.id }}</b>
              <el-tag v-if="s.builtin" size="small" effect="plain">内置</el-tag>
              <el-tag v-else size="small" type="success" effect="plain">自定义</el-tag>
            </div>
            <div class="sc-name">{{ s.name }}</div>
            <div class="sc-hint">{{ s.hint }}</div>
            <div class="sc-meta">
              {{ s.items.length }} 题 · {{ s.options.length }} 个选项 · {{ s.score_mode === 'std125' ? '标准分' : '总分计分' }}
              <el-tag v-if="s.flag_item > 0" type="danger" size="small" effect="plain" style="margin-left: 4px">含预警题</el-tag>
            </div>
            <div class="sc-actions">
              <el-button type="primary" size="small" :disabled="!quizProfileId" @click="startQuizFor(s)">开始测评</el-button>
              <el-button size="small" plain @click="exportScaleDocx(s)">导出问卷</el-button>
              <el-button size="small" plain @click="printScale(s)">打印</el-button>
            </div>
          </el-card>
        </div>
        <el-empty v-if="!scales.length" description="暂无可用量表" :image-size="70" />
      </el-tab-pane>

      <!-- 量表管理 -->
      <el-tab-pane label="量表管理" name="scales">
        <div class="toolbar">
          <span class="dim">维护量表题库与计分规则，所有量表即时在「心理测评」中生效</span>
          <div class="spacer" />
          <el-button @click="resetScales">恢复默认</el-button>
          <el-button type="primary" @click="openScaleDialog()"><el-icon><Plus /></el-icon>新增量表</el-button>
        </div>

        <el-table :data="scales" stripe>
          <el-table-column label="编号" width="100">
            <template #default="{ row }">
              <b>{{ row.id }}</b>
              <el-tag v-if="row.builtin" size="small" effect="plain" style="margin-left: 4px">内置</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="name" label="名称" min-width="180" />
          <el-table-column label="题数" width="70">
            <template #default="{ row }">{{ row.items.length }}</template>
          </el-table-column>
          <el-table-column label="计分" width="110">
            <template #default="{ row }">{{ row.score_mode === 'std125' ? '标准分×1.25' : '总分计分' }}</template>
          </el-table-column>
          <el-table-column label="分档" min-width="200">
            <template #default="{ row }">
              <span class="dim">{{ row.thresholds.map((t: ScaleThreshold) => `${t.min}+ ${t.level}`).join(' / ') }}</span>
            </template>
          </el-table-column>
          <el-table-column label="预警题" width="90">
            <template #default="{ row }">
              <el-tag v-if="row.flag_item > 0" type="danger" size="small">第 {{ row.flag_item }} 题</el-tag>
              <span v-else class="dim">无</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="290" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openScaleDialog(row)">编辑</el-button>
              <el-button size="small" text @click="exportScaleDocx(row)">导出问卷</el-button>
              <el-button size="small" text @click="printScale(row)">打印</el-button>
              <el-button v-if="!row.builtin" size="small" text type="danger" @click="removeScale(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 跟进提醒 -->
      <el-tab-pane name="followups">
        <template #label>
          跟进提醒<el-badge v-if="followups.length" :value="followups.length" class="badge" />
        </template>
        <el-table :data="followups" stripe>
          <el-table-column label="跟进日期" width="120">
            <template #default="{ row }">
              <span :class="{ overdue: row.overdue }">{{ row.next_follow }}</span>
              <el-tag v-if="row.overdue" type="danger" size="small" style="margin-left: 6px">已逾期</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="student_name" label="学生" width="100" />
          <el-table-column prop="class_name" label="班级" width="110" />
          <el-table-column label="等级" width="100">
            <template #default="{ row }"><el-tag :type="riskType(row.risk_level)" size="small">{{ row.risk_level }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="topic" label="上次辅导主题" min-width="160" />
          <el-table-column prop="session_date" label="上次日期" width="110" />
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openDetail({ id: row.profile_id })">去处理</el-button>
            </template>
          </el-table-column>
          <template #empty><span class="dim">未来 7 天内没有待跟进的辅导计划</span></template>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- 档案编辑对话框 -->
    <el-dialog v-model="profileDialog" :title="profileForm.id ? '编辑档案' : '新建心理档案'" width="520px">
      <el-form label-width="90px">
        <el-form-item label="学生姓名" required>
          <el-input v-model="profileForm.student_name" maxlength="32" />
        </el-form-item>
        <el-form-item label="班级">
          <el-select v-model="profileForm.class_name" filterable allow-create clearable>
            <el-option v-for="c in classOptions" :key="c" :value="c" :label="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="性别">
          <el-radio-group v-model="profileForm.gender">
            <el-radio value="男">男</el-radio>
            <el-radio value="女">女</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="关注等级">
          <el-radio-group v-model="profileForm.risk_level">
            <el-radio-button v-for="l in RISK_LEVELS" :key="l" :value="l">{{ l }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="profileForm.tags" placeholder="逗号分隔，如：厌学, 家庭变故" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="profileForm.note" type="textarea" :rows="3" maxlength="500" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="profileDialog = false">取消</el-button>
        <el-button type="primary" :loading="savingProfile" @click="saveProfile">保存</el-button>
      </template>
    </el-dialog>

    <!-- 档案详情抽屉 -->
    <el-drawer v-model="drawer" size="620px" :title="detail ? `${detail.student_name} · 心理档案` : ''">
      <div v-if="detail" class="detail">
        <div class="detail-head">
          <el-tag :type="riskType(detail.risk_level)">{{ detail.risk_level }}</el-tag>
          <span class="dim">{{ detail.class_name }} · {{ detail.gender }}</span>
          <span class="dim" v-if="detail.tags">标签：{{ detail.tags }}</span>
          <div class="spacer" />
          <el-button size="small" type="warning" plain @click="startQuiz(detail)">发起测评</el-button>
          <el-button size="small" type="primary" plain @click="goAgent(detail)">问智能助手</el-button>
        </div>
        <p v-if="detail.note" class="note">{{ detail.note }}</p>

        <div class="sec-title">辅导记录（{{ detail.sessions.length }}）</div>
        <el-timeline v-if="detail.sessions.length" class="timeline">
          <el-timeline-item v-for="s in detail.sessions" :key="s.id" :timestamp="`${s.session_date} · ${s.mode}`" placement="top">
            <el-card shadow="never" class="session-card">
              <div class="s-head">
                <b>{{ s.topic || '（无主题）' }}</b>
                <el-button size="small" text type="danger" @click="removeSession(s)">删除</el-button>
              </div>
              <p class="s-content">{{ s.content }}</p>
              <div v-if="s.next_follow" class="s-follow">下次跟进：{{ s.next_follow }}</div>
            </el-card>
          </el-timeline-item>
        </el-timeline>
        <p v-else class="dim">还没有辅导记录</p>

        <el-card shadow="never" class="card add-session">
          <template #header>添加辅导记录</template>
          <el-form label-width="80px" size="default">
            <el-form-item label="日期">
              <el-date-picker v-model="sessionForm.session_date" type="date" value-format="YYYY-MM-DD" style="width: 160px" />
            </el-form-item>
            <el-form-item label="方式">
              <el-select v-model="sessionForm.mode" style="width: 160px">
                <el-option v-for="m in ['面谈', '电话', '线上沟通', '家访', '其他']" :key="m" :value="m" :label="m" />
              </el-select>
            </el-form-item>
            <el-form-item label="主题">
              <el-input v-model="sessionForm.topic" maxlength="50" placeholder="如：考前焦虑谈话" />
            </el-form-item>
            <el-form-item label="内容" required>
              <el-input v-model="sessionForm.content" type="textarea" :rows="3" maxlength="2000" placeholder="谈话要点、学生表现、约定事项…" />
            </el-form-item>
            <el-form-item label="下次跟进">
              <el-date-picker v-model="sessionForm.next_follow" type="date" value-format="YYYY-MM-DD" style="width: 160px" placeholder="可不填" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="savingSession" @click="addSession">保存记录</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <div class="sec-title">测评历史（{{ detail.assessments.length }}）</div>
        <el-table v-if="detail.assessments.length" :data="detail.assessments" size="small">
          <el-table-column prop="scale" label="量表" width="90" />
          <el-table-column prop="score" label="得分" width="70" />
          <el-table-column prop="level" label="结果" min-width="120" />
          <el-table-column label="预警" width="70">
            <template #default="{ row }"><el-tag v-if="row.flagged" type="danger" size="small">预警</el-tag></template>
          </el-table-column>
          <el-table-column prop="created_at" label="时间" width="150" />
          <el-table-column label="操作" width="70" fixed="right">
            <template #default="{ row }"><el-button size="small" text type="danger" @click="removeAssessment(row)">删除</el-button></template>
          </el-table-column>
        </el-table>
        <p v-else class="dim">还没有测评记录</p>
      </div>
    </el-drawer>

    <!-- 测评问卷对话框 -->
    <el-dialog v-model="quizDialog" width="680px" :title="quiz.profile ? `心理测评：${quiz.profile.student_name}` : '心理测评'" top="4vh">
      <div class="quiz-head">
        <span>选择量表：</span>
        <el-select :model-value="quiz.scale?.id ?? ''" placeholder="选择量表" style="width: 300px" @update:model-value="(v: any) => pickScale(v)">
          <el-option v-for="s in scales" :key="s.id" :value="s.id" :label="`${s.id} · ${s.name}`" />
        </el-select>
      </div>

      <template v-if="quiz.scale">
        <p class="dim quiz-hint">{{ quiz.scale.name }} — {{ quiz.scale.hint }}</p>
        <el-progress :percentage="Math.round((quizProgress / quiz.scale.items.length) * 100)" :stroke-width="6" class="quiz-progress" />
        <div class="quiz-scroll">
          <div v-for="(item, i) in quiz.scale.items" :key="i" class="quiz-item">
            <div class="q-title">
              {{ i + 1 }}. {{ item }}
              <el-tag v-if="quiz.scale.flag_item === i + 1" type="danger" size="small" effect="plain" style="margin-left: 6px">预警题</el-tag>
            </div>
            <el-radio-group v-model="quiz.answers[i]">
              <el-radio v-for="(opt, oi) in quiz.scale.options" :key="oi" :value="opt.score" class="q-opt">{{ opt.label }}</el-radio>
            </el-radio-group>
          </div>
        </div>
      </template>
      <el-empty v-else description="请先选择要使用的量表" :image-size="70" />

      <el-alert v-if="quiz.result" class="quiz-result" :type="quiz.result.flagged ? 'error' : quiz.result.level === '正常范围' || quiz.result.level.includes('无') ? 'success' : 'warning'" :closable="false">
        <template #title>
          测评结果：{{ quiz.result.score }} 分 · {{ quiz.result.level }}
          <template v-if="quiz.result.flagged"> · ⚠️ 出现预警题目，建议马上进行安全谈话并联系专业人员</template>
        </template>
        <div>结果已保存到该学生档案。量表结果仅为筛查参考，不能替代临床诊断。</div>
        <div v-if="resultNote" class="result-note">{{ resultNote }}</div>
      </el-alert>

      <template #footer>
        <el-button @click="quizDialog = false">关闭</el-button>
        <el-button v-if="!quiz.result" type="primary" :disabled="!quiz.scale || quizProgress < (quiz.scale?.items.length ?? 0)" :loading="submittingQuiz" @click="submitQuiz">
          提交并计分（{{ quizProgress }}/{{ quiz.scale?.items.length ?? 0 }}）
        </el-button>
      </template>
    </el-dialog>

    <!-- 量表编辑对话框 -->
    <el-dialog v-model="scaleDialog" :title="scaleForm.editing ? `编辑量表：${scaleForm.id}` : '新增量表'" width="720px" top="4vh">
      <el-form label-width="100px" label-position="left">
        <el-form-item label="量表编号" required>
          <el-input v-model="scaleForm.id" :disabled="scaleForm.editing" placeholder="如 SSRS-40（字母/数字/中划线/下划线）" maxlength="32" style="width: 300px" />
        </el-form-item>
        <el-form-item label="量表名称" required>
          <el-input v-model="scaleForm.name" maxlength="100" placeholder="如 社会支持评定量表" style="width: 420px" />
        </el-form-item>
        <el-form-item label="指导语">
          <el-input v-model="scaleForm.hint" type="textarea" :rows="2" maxlength="200" placeholder="作答前展示给学生的说明" />
        </el-form-item>
        <el-form-item label="题目（每行一题）" required>
          <el-input v-model="scaleForm.itemsText" type="textarea" :rows="8" placeholder="每行一道题目" />
          <div class="dim form-tip">当前 {{ scaleForm.itemsText.split('\n').filter((t) => t.trim()).length }} 题</div>
        </el-form-item>
        <el-form-item label="选项与分值" required>
          <div class="opt-rows">
            <div v-for="(o, i) in scaleForm.options" :key="i" class="opt-row">
              <el-input v-model="o.label" placeholder="选项文字" style="width: 220px" maxlength="40" />
              <el-input-number v-model="o.score" :min="0" :max="10" size="default" style="width: 110px" />
              <el-button size="small" text type="danger" :disabled="scaleForm.options.length <= 2" @click="scaleForm.options.splice(i, 1)">删除</el-button>
            </div>
            <el-button size="small" :disabled="scaleForm.options.length >= 6" @click="scaleForm.options.push({ label: '', score: scaleForm.options.length })"><el-icon><Plus /></el-icon>添加选项</el-button>
          </div>
          <div class="dim form-tip">2-6 个选项，分值 0-10 且不能重复</div>
        </el-form-item>
        <el-form-item label="反向计分题">
          <el-input v-model="scaleForm.reverseText" placeholder="题号，逗号分隔，如 3, 7, 12（留空表示无）" style="width: 360px" />
        </el-form-item>
        <el-form-item label="计分方式">
          <el-radio-group v-model="scaleForm.score_mode">
            <el-radio value="sum">总分计分（各项相加）</el-radio>
            <el-radio value="std125">标准分（总分 × 1.25 取整，SAS/SDS 口径）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="结果分档" required>
          <div class="opt-rows">
            <div v-for="(t, i) in scaleForm.thresholds" :key="i" class="opt-row">
              <el-input-number v-model="t.min" :min="0" :max="999" size="default" style="width: 110px" />
              <span class="dim">分起 →</span>
              <el-input v-model="t.level" placeholder="等级，如 轻度焦虑" style="width: 160px" maxlength="20" />
              <el-input v-model="t.note" placeholder="结果解释与建议（可空）" style="width: 240px" maxlength="500" />
              <el-button size="small" text type="danger" :disabled="scaleForm.thresholds.length <= 1" @click="scaleForm.thresholds.splice(i, 1)">删除</el-button>
            </div>
            <el-button size="small" :disabled="scaleForm.thresholds.length >= 10" @click="scaleForm.thresholds.unshift({ min: 0, level: '', note: '' })"><el-icon><Plus /></el-icon>添加分档</el-button>
          </div>
          <div class="dim form-tip">按「分起」从高到低生效，最后一档须从 0 分起（保存时自动排序）</div>
        </el-form-item>
        <el-form-item label="预警题">
          <div class="flag-row">
            <span>第</span>
            <el-input-number v-model="scaleForm.flag_item" :min="0" :max="200" size="default" style="width: 100px" />
            <span>题得分 ≥</span>
            <el-input-number v-model="scaleForm.flag_min" :min="0" :max="10" size="default" style="width: 100px" />
            <span>时标记预警（题号填 0 表示不启用）</span>
          </div>
        </el-form-item>
        <el-form-item v-if="scaleForm.flag_item > 0" label="预警说明">
          <el-input v-model="scaleForm.flag_note" type="textarea" :rows="2" maxlength="200" placeholder="触发预警时给老师的处置说明，如：第 9 题涉及自伤念头…" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="scaleDialog = false">取消</el-button>
        <el-button type="primary" :loading="savingScales" @click="saveScaleDialog">保存量表</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.psy-page {
  padding: 8px 20px 20px;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.stat {
  text-align: center;
  border: 1px solid #eef0f6;
  border-radius: 14px;
}

.num {
  font-size: 26px;
  font-weight: 700;
  color: #4f6bed;
}

.stat.danger .num {
  color: #e64545;
}

.stat.warning .num {
  color: #e6a23c;
}

.stat.info .num {
  color: #7a839b;
}

.stat.flag .num {
  color: #d95f8b;
}

.label {
  color: #7a839b;
  font-size: 12.5px;
  margin-top: 4px;
}

.card {
  border: 1px solid #eef0f6;
  border-radius: 14px;
}

.overview-grid {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 14px;
}

.guide {
  margin: 0;
  padding-left: 18px;
  color: #5a6378;
  line-height: 1.9;
  font-size: 13.5px;
}

.toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
  align-items: center;
}

.spacer {
  flex: 1;
}

.dim {
  color: #a0a8bd;
  font-size: 13px;
}

.tag {
  margin-right: 5px;
}

.overdue {
  color: #e64545;
  font-weight: 600;
}

.badge {
  margin-left: 6px;
  vertical-align: text-top;
}

.scale-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
}

.scale-card {
  border: 1px solid #eef0f6;
  border-radius: 14px;
}

.sc-head {
  display: flex;
  gap: 8px;
  align-items: center;
}

.sc-name {
  font-weight: 600;
  margin: 8px 0 4px;
  color: #2f3646;
}

.sc-hint {
  color: #7a839b;
  font-size: 12.5px;
  min-height: 36px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.sc-meta {
  color: #a0a8bd;
  font-size: 12.5px;
  margin: 8px 0 10px;
}

.sc-actions {
  display: flex;
  gap: 4px;
}

.detail {
  padding-bottom: 30px;
}

.detail-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.note {
  background: #faf8f2;
  border: 1px solid #f0ead8;
  border-radius: 8px;
  padding: 10px 12px;
  color: #77684a;
  font-size: 13px;
}

.sec-title {
  font-weight: 700;
  margin: 18px 0 12px;
  font-size: 15px;
}

.timeline {
  padding-left: 4px;
}

.session-card {
  border: 1px solid #eef0f6;
}

.s-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.s-content {
  color: #4a5268;
  font-size: 13.5px;
  white-space: pre-wrap;
  margin: 6px 0;
}

.s-follow {
  font-size: 12.5px;
  color: #4f6bed;
}

.add-session {
  background: #fafbfe;
}

.quiz-head {
  margin-bottom: 12px;
}

.quiz-hint {
  margin: 4px 0 10px;
}

.quiz-progress {
  margin-bottom: 10px;
}

.quiz-scroll {
  max-height: 46vh;
  overflow-y: auto;
  padding-right: 6px;
}

.quiz-item {
  padding: 10px 0;
  border-bottom: 1px dashed #f0f1f6;
}

.q-title {
  margin-bottom: 6px;
  color: #2f3646;
  font-size: 14px;
}

.q-opt {
  margin-right: 18px;
}

.quiz-result {
  margin-top: 14px;
}

.result-note {
  margin-top: 6px;
  white-space: pre-line;
}

.opt-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.opt-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.flag-row {
  display: flex;
  gap: 8px;
  align-items: center;
  color: #5a6378;
  font-size: 13px;
}

.form-tip {
  margin-top: 4px;
  line-height: 1.4;
}
</style>
