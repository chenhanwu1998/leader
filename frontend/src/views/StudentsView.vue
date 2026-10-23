<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { del, fetchClasses, get, post, put } from '@/api'
import { exportCsv, exportExcel } from '@/utils/excel'
import ImportDialog from '@/components/ImportDialog.vue'
import EChart from '@/components/EChart.vue'

const tab = ref('list')
const loading = ref(false)
const list = ref<any[]>([])
const classes = ref<string[]>([])
const selection = ref<any[]>([])
const filter = reactive({ keyword: '', class_name: '', status: '', gender: '' })
const page = ref(1)
const pageSize = ref(15)

const importVisible = ref(false)
const dialogVisible = ref(false)
const editing = ref<any>(null)
const profileVisible = ref(false)
const profile = ref<any>(null)
const profileInfo = ref<any>({})
const stats = ref<any>(null)
const saving = ref(false)

const form = ref<any>({})
const blank = () => ({ student_no: '', name: '', gender: '男', class_name: '', birth_date: '', phone: '', parent_name: '', parent_phone: '', address: '', status: '在读', remark: '' })

const importColumns = [
  { key: 'student_no', label: '学号', synonyms: ['编号', '学籍号'] },
  { key: 'name', label: '姓名', required: true },
  { key: 'gender', label: '性别' },
  { key: 'class_name', label: '班级' },
  { key: 'birth_date', label: '出生日期', synonyms: ['生日'] },
  { key: 'phone', label: '联系电话', synonyms: ['电话'] },
  { key: 'parent_name', label: '家长姓名' },
  { key: 'parent_phone', label: '家长电话', synonyms: ['家长联系方式'] },
  { key: 'address', label: '家庭住址', synonyms: ['地址', '住址'] },
  { key: 'status', label: '状态' },
  { key: 'remark', label: '备注' },
]
const importSample = [{ 学号: '1', 姓名: '张三', 性别: '男', 班级: '一年级1班', 出生日期: '2017-09-01', 联系电话: '', 家长姓名: '张父', 家长电话: '13800000000', 家庭住址: '', 状态: '在读', 备注: '' }]

const paged = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return list.value.slice(start, start + pageSize.value)
})

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (filter.keyword) params.set('keyword', filter.keyword)
    if (filter.class_name) params.set('class_name', filter.class_name)
    if (filter.status) params.set('status', filter.status)
    if (filter.gender) params.set('gender', filter.gender)
    list.value = await get(`/api/students?${params}`)
    page.value = 1
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function loadClasses() {
  classes.value = await fetchClasses()
}

async function loadStats() {
  stats.value = await get('/api/stats/students')
}

onMounted(() => {
  load()
  loadClasses()
  loadStats()
})

function openAdd() {
  editing.value = null
  form.value = blank()
  dialogVisible.value = true
}

function openEdit(row: any) {
  editing.value = row
  form.value = { ...row }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name?.trim()) {
    ElMessage.warning('请填写学生姓名')
    return
  }
  saving.value = true
  try {
    if (editing.value) await put(`/api/students/${editing.value.id}`, form.value)
    else await post('/api/students', form.value)
    ElMessage.success(editing.value ? '已更新学生信息' : '已添加学生')
    dialogVisible.value = false
    load()
    loadClasses()
    loadStats()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row: any) {
  await ElMessageBox.confirm(`确定删除学生「${row.name}」？其成绩、请假等关联记录会一并删除。`, '删除确认', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  await del(`/api/students/${row.id}`)
  ElMessage.success('已删除')
  load()
  loadStats()
}

async function removeBatch() {
  await ElMessageBox.confirm(`确定删除选中的 ${selection.value.length} 名学生？关联记录会一并删除。`, '批量删除', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  await post('/api/students/delete-batch', { ids: selection.value.map((s) => s.id) })
  ElMessage.success('已批量删除')
  load()
  loadStats()
}

async function onImport(records: any[]) {
  const res = await post('/api/students/batch', records)
  if (res.fail) ElMessage.warning(`导入完成：成功 ${res.ok} 条，失败 ${res.fail} 条。${res.errors?.[0] ?? ''}`)
  else ElMessage.success(`成功导入 ${res.ok} 名学生`)
  load()
  loadClasses()
  loadStats()
}

const exportRows = computed(() =>
  list.value.map((s) => ({
    学号: s.student_no, 姓名: s.name, 性别: s.gender, 班级: s.class_name, 出生日期: s.birth_date,
    联系电话: s.phone, 家长姓名: s.parent_name, 家长电话: s.parent_phone, 家庭住址: s.address, 状态: s.status, 备注: s.remark,
  })),
)

async function openProfile(row: any) {
  profileVisible.value = true
  profileInfo.value = await get(`/api/stats/student/${row.id}`)
  profile.value = row
}

const profileChart = computed(() => {
  const scores = profileInfo.value.scores ?? []
  const exams = [...new Map(scores.map((s: any) => [s.exam_id, s.exam_name])).values()]
  const byExam = new Map<number, any[]>()
  for (const s of scores) {
    if (!byExam.has(s.exam_id)) byExam.set(s.exam_id, [])
    byExam.get(s.exam_id)!.push(s)
  }
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: 10, right: 20, top: 30, bottom: 0, containLabel: true },
    xAxis: { type: 'category', data: exams.map((e: any) => e), axisTick: { show: false } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: '#eef0f6' } } },
    series: [{
      name: '总分', type: 'line', smooth: true, symbolSize: 8,
      data: [...byExam.values()].map((arr) => arr.reduce((a, b) => a + b.score, 0)),
      lineStyle: { width: 3, color: '#4f6bed' }, itemStyle: { color: '#4f6bed' },
      areaStyle: { color: 'rgba(79,107,237,0.12)' },
    }],
  }
})

const genderPie = computed(() => ({
  color: ['#4f6bed', '#ee8fb3', '#f2a545'],
  tooltip: { trigger: 'item' },
  legend: { bottom: 0 },
  series: [{
    type: 'pie', radius: ['45%', '70%'], center: ['50%', '45%'],
    itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
    label: { formatter: '{b} {c}人' },
    data: [...(stats.value?.gender ?? [])]
      .sort((a: any, b: any) => (a.k === '男' ? -1 : 0) - (b.k === '男' ? -1 : 0))
      .map((d: any) => ({ name: d.k, value: d.v })),
  }],
}))

const classBar = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: 10, right: 16, top: 24, bottom: 0, containLabel: true },
  xAxis: { type: 'category', data: (stats.value?.byClass ?? []).map((d: any) => d.k || '未分班'), axisTick: { show: false } },
  yAxis: { type: 'value', splitLine: { lineStyle: { color: '#eef0f6' } }, minInterval: 1 },
  series: [{ type: 'bar', barWidth: 24, itemStyle: { borderRadius: [6, 6, 0, 0], color: '#22b8a5' }, data: (stats.value?.byClass ?? []).map((d: any) => d.v) }],
}))

const statusPie = computed(() => ({
  color: ['#22b8a5', '#f2a545', '#ee6b6e', '#8a93a8'],
  tooltip: { trigger: 'item' },
  legend: { bottom: 0 },
  series: [{
    type: 'pie', radius: ['45%', '70%'], center: ['50%', '45%'],
    itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
    data: (stats.value?.byStatus ?? []).map((d: any) => ({ name: d.k, value: d.v })),
  }],
}))
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">学生花名册</h2>
        <div class="page-sub">维护学生基本信息，支持 Excel / CSV 批量导入导出</div>
      </div>
    </div>

    <el-tabs v-model="tab" class="card tabs-card">
      <el-tab-pane label="📋 名单管理" name="list">
        <div class="toolbar" style="margin-bottom: 14px">
          <el-input v-model="filter.keyword" placeholder="搜索姓名 / 学号" clearable style="width: 180px" @keyup.enter="load" @clear="load">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="filter.class_name" placeholder="班级" clearable filterable style="width: 140px" @change="load">
            <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
          </el-select>
          <el-select v-model="filter.gender" placeholder="性别" clearable style="width: 90px" @change="load">
            <el-option value="男" label="男" />
            <el-option value="女" label="女" />
          </el-select>
          <el-select v-model="filter.status" placeholder="状态" clearable style="width: 110px" @change="load">
            <el-option v-for="s in ['在读', '转出', '休学', '毕业']" :key="s" :value="s" :label="s" />
          </el-select>
          <el-button type="primary" plain @click="load">查询</el-button>
          <div style="flex: 1" />
          <el-button v-if="selection.length" type="danger" plain @click="removeBatch">删除所选（{{ selection.length }}）</el-button>
          <el-button @click="importVisible = true"><el-icon style="margin-right: 4px"><Upload /></el-icon>导入</el-button>
          <el-dropdown>
            <el-button><el-icon style="margin-right: 4px"><Download /></el-icon>导出<el-icon style="margin-left: 4px"><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="exportExcel(exportRows, '学生花名册')">导出 Excel</el-dropdown-item>
                <el-dropdown-item @click="exportCsv(exportRows, '学生花名册')">导出 CSV</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button type="primary" @click="openAdd"><el-icon style="margin-right: 4px"><Plus /></el-icon>添加学生</el-button>
        </div>

        <el-table :data="paged" v-loading="loading" @selection-change="selection = $event" stripe>
          <el-table-column type="selection" width="42" />
          <el-table-column prop="student_no" label="学号" width="80" sortable />
          <el-table-column prop="name" label="姓名" width="100">
            <template #default="{ row }">
              <a style="color: #4f6bed; cursor: pointer; font-weight: 600" @click="openProfile(row)">{{ row.name }}</a>
            </template>
          </el-table-column>
          <el-table-column prop="gender" label="性别" width="66">
            <template #default="{ row }">
              <el-tag size="small" :color="row.gender === '女' ? '#fdeef5' : '#eef2ff'" :style="{ color: row.gender === '女' ? '#d05a8b' : '#4f6bed', border: 'none' }">{{ row.gender }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="class_name" label="班级" width="120" />
          <el-table-column prop="parent_name" label="家长" width="90" show-overflow-tooltip />
          <el-table-column prop="parent_phone" label="家长电话" width="125" />
          <el-table-column prop="status" label="状态" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === '在读' ? 'success' : 'info'" effect="light">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
          <el-table-column label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button link type="danger" @click="remove(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><div class="empty-tip">暂无学生，点击右上角「添加学生」或「导入」开始</div></template>
        </el-table>

        <div style="display: flex; justify-content: flex-end; margin-top: 12px">
          <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="list.length" :page-sizes="[15, 30, 50, 100]" layout="total, sizes, prev, pager, next" background />
        </div>
      </el-tab-pane>

      <el-tab-pane label="📊 统计分析" name="stats" lazy>
        <template v-if="stats">
          <div class="stat-cards" style="margin-bottom: 16px">
            <div class="stat-card" v-if="stats.alerts.withoutParentPhone">
              <div class="stat-icon" style="background: #fdeeee">⚠️</div>
              <div><div class="stat-num">{{ stats.alerts.withoutParentPhone }}</div><div class="stat-label">缺家长电话（需补录）</div></div>
            </div>
            <div class="stat-card" v-if="stats.alerts.withoutNo">
              <div class="stat-icon" style="background: #fef6ec">⚠️</div>
              <div><div class="stat-num">{{ stats.alerts.withoutNo }}</div><div class="stat-label">缺学号（需补录）</div></div>
            </div>
            <div class="stat-card">
              <div class="stat-icon" style="background: #effaf7">✅</div>
              <div><div class="stat-num">{{ stats.byStatus.find((s: any) => s.k === '在读')?.v ?? 0 }}</div><div class="stat-label">在读学生</div></div>
            </div>
          </div>
          <div class="grid-cards" style="grid-template-columns: repeat(auto-fit, minmax(280px, 1fr))">
            <div class="card" style="box-shadow: none; border: 1px solid #eef0f6">
              <div class="card-title">性别分布</div>
              <EChart :option="genderPie" height="250px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">各班人数</div>
              <EChart :option="classBar" height="250px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">学籍状态</div>
              <EChart :option="statusPie" height="250px" />
            </div>
          </div>
        </template>
      </el-tab-pane>
    </el-tabs>

    <!-- 编辑 / 新增 -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑学生' : '添加学生'" width="640px">
      <el-form :model="form" label-width="84px">
        <el-row :gutter="14">
          <el-col :span="12"><el-form-item label="姓名" required><el-input v-model="form.name" placeholder="必填" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="学号"><el-input v-model="form.student_no" /></el-form-item></el-col>
          <el-col :span="12">
            <el-form-item label="性别">
              <el-radio-group v-model="form.gender">
                <el-radio value="男">男</el-radio>
                <el-radio value="女">女</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="班级">
              <el-select v-model="form.class_name" filterable clearable placeholder="从已有班级中选择" style="width: 100%">
                <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
                <template #empty>
                  <div style="padding: 10px 14px; color: #8a93a8; font-size: 13px; line-height: 1.7">
                    <template v-if="classes.length">没有匹配的班级，只能从已有班级中选择。</template>
                    <template v-else>还没有班级。<br />请先到「班级管理」新建班级，或通过批量导入自动建班。</template>
                  </div>
                </template>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12"><el-form-item label="出生日期"><el-date-picker v-model="form.birth_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-select v-model="form.status" style="width: 100%">
                <el-option v-for="s in ['在读', '转出', '休学', '毕业']" :key="s" :value="s" :label="s" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12"><el-form-item label="联系电话"><el-input v-model="form.phone" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="家长姓名"><el-input v-model="form.parent_name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="家长电话"><el-input v-model="form.parent_phone" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="家庭住址"><el-input v-model="form.address" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 学生档案抽屉 -->
    <el-drawer v-model="profileVisible" :title="`${profile?.name ?? ''} 的个人档案`" size="520px">
      <template v-if="profile">
        <div class="stat-cards" style="grid-template-columns: repeat(3, 1fr); margin-bottom: 16px">
          <div class="stat-card"><div><div class="stat-num">{{ profileInfo.leaves?.c ?? 0 }}</div><div class="stat-label">请假 {{ profileInfo.leaves?.d ?? 0 }} 天</div></div></div>
          <div class="stat-card"><div><div class="stat-num">{{ (profileInfo.rewards ?? []).reduce((a: number, r: any) => a + r.c, 0) }}</div><div class="stat-label">奖惩记录</div></div></div>
          <div class="stat-card"><div><div class="stat-num">{{ profileInfo.contacts ?? 0 }}</div><div class="stat-label">家校沟通</div></div></div>
        </div>
        <div class="card" style="box-shadow: none; margin-bottom: 14px">
          <div class="card-title">历次考试总分走势</div>
          <EChart v-if="(profileInfo.scores ?? []).length" :option="profileChart" height="220px" />
          <div v-else class="empty-tip" style="padding: 20px 0">暂无成绩记录</div>
        </div>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="学号">{{ profile.student_no || '—' }}</el-descriptions-item>
          <el-descriptions-item label="班级">{{ profile.class_name || '—' }}</el-descriptions-item>
          <el-descriptions-item label="性别">{{ profile.gender }}</el-descriptions-item>
          <el-descriptions-item label="出生日期">{{ profile.birth_date || '—' }}</el-descriptions-item>
          <el-descriptions-item label="家长">{{ profile.parent_name || '—' }}</el-descriptions-item>
          <el-descriptions-item label="家长电话">{{ profile.parent_phone || '—' }}</el-descriptions-item>
          <el-descriptions-item label="家庭住址" :span="2">{{ profile.address || '—' }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ profile.remark || '—' }}</el-descriptions-item>
        </el-descriptions>
      </template>
    </el-drawer>

    <ImportDialog v-model="importVisible" title="学生花名册" :columns="importColumns" :sample="importSample"
      tip="性别支持「男 / 女」，状态支持「在读 / 转出 / 休学 / 毕业」，日期格式如 2017-09-01" @confirm="onImport" />
  </div>
</template>

<style scoped>
.tabs-card :deep(.el-tabs__content) {
  padding-top: 4px;
}
</style>
