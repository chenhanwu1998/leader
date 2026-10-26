<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { get, post, put, del, fetchClasses } from '@/api'

const router = useRouter()
const RISK_LEVELS = ['常规关注', '重点关注', '高危预警']
const riskType = (l: string) => (l === '高危预警' ? 'danger' : l === '重点关注' ? 'warning' : 'info')

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

// —— 测评 ——
const scales = ref<{ id: string; name: string; hint: string; options: string[]; items: string[]; min: number }[]>([])
async function loadScales() {
  scales.value = await get('/api/psy/scales')
}

const quizDialog = ref(false)
const quiz = reactive({ profile: null as any, scale: null as any, answers: [] as number[], result: null as any })

function startQuiz(profile: any, scaleId = '') {
  quiz.profile = profile
  quiz.scale = scales.value.find((s) => s.id === scaleId) ?? null
  quiz.answers = []
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
              <li>定期用「心理测评」给学生做 PHQ-9 / GAD-7 / SAS / SDS 筛查，系统自动计分并给出结果解释。</li>
              <li>每次谈话或辅导后记一条「辅导记录」，可设置下次跟进日期，到龄会在「跟进提醒」出现。</li>
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
        <el-radio-group :model-value="quiz.scale?.id ?? ''" @update:model-value="(v: any) => pickScale(v)">
          <el-radio-button v-for="s in scales" :key="s.id" :value="s.id">{{ s.id }}</el-radio-button>
        </el-radio-group>
      </div>

      <template v-if="quiz.scale">
        <p class="dim quiz-hint">{{ quiz.scale.name }} — {{ quiz.scale.hint }}</p>
        <el-progress :percentage="Math.round((quizProgress / quiz.scale.items.length) * 100)" :stroke-width="6" class="quiz-progress" />
        <div class="quiz-scroll">
          <div v-for="(item, i) in quiz.scale.items" :key="i" class="quiz-item">
            <div class="q-title">{{ i + 1 }}. {{ item }}</div>
            <el-radio-group v-model="quiz.answers[i]">
              <el-radio v-for="(opt, oi) in quiz.scale.options" :key="oi" :value="oi + quiz.scale.min" class="q-opt">{{ opt }}</el-radio>
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
        结果已保存到该学生档案。量表结果仅为筛查参考，不能替代临床诊断。
      </el-alert>

      <template #footer>
        <el-button @click="quizDialog = false">关闭</el-button>
        <el-button v-if="!quiz.result" type="primary" :disabled="!quiz.scale || quizProgress < (quiz.scale?.items.length ?? 0)" :loading="submittingQuiz" @click="submitQuiz">
          提交并计分（{{ quizProgress }}/{{ quiz.scale?.items.length ?? 0 }}）
        </el-button>
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
</style>
