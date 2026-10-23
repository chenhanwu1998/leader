<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { del, fetchClasses, get, post, put } from '@/api'
import { exportCsv, exportExcel } from '@/utils/excel'
import ImportDialog from '@/components/ImportDialog.vue'
import StudentSelect from '@/components/StudentSelect.vue'
import EChart from '@/components/EChart.vue'

const tab = ref('list')
const loading = ref(false)
const list = ref<any[]>([])
const classes = ref<string[]>([])
const filter = reactive({ class_name: '', status: '', type: '', keyword: '', dateRange: null as null | [string, string] })
const page = ref(1)
const pageSize = ref(15)

const dialogVisible = ref(false)
const editing = ref<any>(null)
const saving = ref(false)
const importVisible = ref(false)
const stats = ref<any>(null)
const statsClass = ref('')
const formClass = ref('') // 弹窗内用于缩小学生检索范围的班级

const form = ref<any>({})
const blank = () => ({ student_id: null, type: '事假', start_date: new Date().toISOString().slice(0, 10), end_date: new Date().toISOString().slice(0, 10), days: 1, reason: '', status: '请假中', remark: '' })

const importColumns = [
  { key: 'student_name', label: '姓名', required: true },
  { key: 'class_name', label: '班级' },
  { key: 'type', label: '类型' },
  { key: 'start_date', label: '开始日期' },
  { key: 'end_date', label: '结束日期' },
  { key: 'days', label: '天数' },
  { key: 'reason', label: '事由' },
  { key: 'status', label: '状态' },
]
const importSample = [{ 姓名: '张三', 班级: '一年级1班', 类型: '病假', 开始日期: '2026-10-20', 结束日期: '2026-10-21', 天数: 2, 事由: '感冒发烧', 状态: '已销假' }]

const paged = computed(() => list.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (filter.class_name) params.set('class_name', filter.class_name)
    if (filter.keyword) params.set('keyword', filter.keyword)
    if (filter.status) params.set('status', filter.status)
    if (filter.type) params.set('type', filter.type)
    if (filter.dateRange?.[0]) params.set('date_from', filter.dateRange[0])
    if (filter.dateRange?.[1]) params.set('date_to', filter.dateRange[1])
    list.value = await get(`/api/leaves?${params}`)
    page.value = 1
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  stats.value = await get(`/api/stats/leaves${statsClass.value ? `?class_name=${encodeURIComponent(statsClass.value)}` : ''}`)
}

onMounted(async () => {
  classes.value = await fetchClasses()
  await Promise.all([load(), loadStats()])
})

function openAdd() {
  editing.value = null
  form.value = blank()
  formClass.value = ''
  dialogVisible.value = true
}
function openEdit(row: any) {
  editing.value = row
  form.value = { ...row }
  formClass.value = row.class_name ?? ''
  dialogVisible.value = true
}

async function save() {
  if (!form.value.student_id) return ElMessage.warning('请选择学生')
  saving.value = true
  try {
    if (editing.value) await put(`/api/leaves/${editing.value.id}`, form.value)
    else await post('/api/leaves', form.value)
    ElMessage.success('已保存请假记录')
    dialogVisible.value = false
    load()
    loadStats()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row: any) {
  await ElMessageBox.confirm(`确定删除「${row.student_name}」的这条请假记录？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  await del(`/api/leaves/${row.id}`)
  ElMessage.success('已删除')
  load()
  loadStats()
}

async function quickFinish(row: any) {
  await put(`/api/leaves/${row.id}`, { status: '已销假' })
  ElMessage.success('已标记为销假')
  load()
  loadStats()
}

async function onImport(records: any[]) {
  const res = await post('/api/leaves/batch', { records })
  if (res.fail) ElMessage.warning(`导入完成：成功 ${res.ok} 条，失败 ${res.fail} 条。${res.errors?.[0] ?? ''}`)
  else ElMessage.success(`成功导入 ${res.ok} 条请假记录`)
  load()
  loadStats()
}

const exportRows = computed(() =>
  list.value.map((r) => ({ 姓名: r.student_name, 班级: r.class_name, 类型: r.type, 开始日期: r.start_date, 结束日期: r.end_date, 天数: r.days, 事由: r.reason, 状态: r.status, 备注: r.remark })),
)

const typePie = computed(() => ({
  color: ['#f2a545', '#ee6b6e', '#5aa9e6', '#8a93a8'],
  tooltip: { trigger: 'item' },
  legend: { bottom: 0 },
  series: [{
    type: 'pie', radius: ['45%', '70%'], center: ['50%', '45%'],
    itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
    label: { formatter: '{b} {c}次' },
    data: (stats.value?.byType ?? []).map((d: any) => ({ name: d.k, value: d.v })),
  }],
}))

const monthBar = computed(() => ({
  tooltip: { trigger: 'axis' },
  legend: { bottom: 0 },
  grid: { left: 6, right: 6, top: 42, bottom: 30, containLabel: true },
  xAxis: { type: 'category', data: (stats.value?.byMonth ?? []).map((d: any) => d.k), axisTick: { show: false } },
  yAxis: [
    { type: 'value', name: '人次', minInterval: 1, splitLine: { lineStyle: { color: '#eef0f6' } } },
    { type: 'value', name: '天数', splitLine: { show: false } },
  ],
  series: [
    { name: '请假人次', type: 'bar', barWidth: 20, itemStyle: { borderRadius: [6, 6, 0, 0], color: '#4f6bed' }, data: (stats.value?.byMonth ?? []).map((d: any) => d.v) },
    { name: '请假天数', type: 'line', yAxisIndex: 1, smooth: true, itemStyle: { color: '#f2a545' }, data: (stats.value?.byMonth ?? []).map((d: any) => d.d) },
  ],
}))
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">请假管理</h2>
        <div class="page-sub">记录学生请假、销假情况，关注高频请假学生</div>
      </div>
    </div>

    <el-tabs v-model="tab" class="card">
      <el-tab-pane label="📋 请假记录" name="list">
        <div class="toolbar" style="margin-bottom: 14px">
          <el-input v-model="filter.keyword" placeholder="搜索姓名 / 事由" clearable style="width: 160px" @keyup.enter="load" @clear="load" />
          <el-select v-model="filter.class_name" placeholder="班级" clearable filterable style="width: 130px" @change="load">
            <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
          </el-select>
          <el-select v-model="filter.type" placeholder="类型" clearable style="width: 96px" @change="load">
            <el-option v-for="t in ['事假', '病假', '其他']" :key="t" :value="t" :label="t" />
          </el-select>
          <el-select v-model="filter.status" placeholder="状态" clearable style="width: 104px" @change="load">
            <el-option v-for="s in ['请假中', '已销假']" :key="s" :value="s" :label="s" />
          </el-select>
          <el-date-picker v-model="filter.dateRange" type="daterange" value-format="YYYY-MM-DD" range-separator="至"
            start-placeholder="开始日期" end-placeholder="结束日期" style="width: 240px" @change="load" />
          <el-button type="primary" plain @click="load">查询</el-button>
          <div style="flex: 1" />
          <el-button @click="importVisible = true"><el-icon style="margin-right: 4px"><Upload /></el-icon>导入</el-button>
          <el-dropdown>
            <el-button><el-icon style="margin-right: 4px"><Download /></el-icon>导出<el-icon style="margin-left: 4px"><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="exportExcel(exportRows, '请假记录')">导出 Excel</el-dropdown-item>
                <el-dropdown-item @click="exportCsv(exportRows, '请假记录')">导出 CSV</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button type="primary" @click="openAdd"><el-icon style="margin-right: 4px"><Plus /></el-icon>登记请假</el-button>
        </div>

        <el-table :data="paged" v-loading="loading" stripe>
          <el-table-column prop="student_name" label="姓名" width="95">
            <template #default="{ row }"><b>{{ row.student_name }}</b></template>
          </el-table-column>
          <el-table-column prop="class_name" label="班级" width="115" />
          <el-table-column prop="type" label="类型" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="row.type === '病假' ? 'danger' : row.type === '事假' ? 'warning' : 'info'" effect="light">{{ row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="请假时间" width="200">
            <template #default="{ row }">{{ row.start_date }} <span style="color: #b0b7c9">至</span> {{ row.end_date }}</template>
          </el-table-column>
          <el-table-column prop="days" label="天数" width="70" />
          <el-table-column prop="reason" label="事由" min-width="150" show-overflow-tooltip />
          <el-table-column prop="status" label="状态" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === '已销假' ? 'success' : 'primary'" effect="light">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="170" fixed="right">
            <template #default="{ row }">
              <el-button v-if="row.status !== '已销假'" link type="success" @click="quickFinish(row)">销假</el-button>
              <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button link type="danger" @click="remove(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><div class="empty-tip">暂无请假记录</div></template>
        </el-table>

        <div style="display: flex; justify-content: flex-end; margin-top: 12px">
          <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="list.length" :page-sizes="[15, 30, 50]" layout="total, sizes, prev, pager, next" background />
        </div>
      </el-tab-pane>

      <el-tab-pane label="📊 统计分析" name="stats" lazy>
        <div class="toolbar" style="margin-bottom: 14px">
          <el-select v-model="statsClass" placeholder="全部班级" clearable style="width: 160px" @change="loadStats">
            <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
          </el-select>
        </div>
        <template v-if="stats">
          <div class="stat-cards" style="margin-bottom: 16px">
            <div class="stat-card">
              <div class="stat-icon" style="background: #eef2ff">📅</div>
              <div><div class="stat-num">{{ stats.byType.reduce((a: number, t: any) => a + t.v, 0) }}</div><div class="stat-label">累计请假人次</div></div>
            </div>
            <div class="stat-card">
              <div class="stat-icon" style="background: #fef6ec">🕐</div>
              <div><div class="stat-num">{{ stats.totalDays }}</div><div class="stat-label">累计请假天数</div></div>
            </div>
            <div class="stat-card">
              <div class="stat-icon" style="background: #fdeeee">⏳</div>
              <div><div class="stat-num">{{ stats.byStatus.find((s: any) => s.k === '请假中')?.v ?? 0 }}</div><div class="stat-label">当前请假中</div></div>
            </div>
          </div>
          <div class="grid-cards" style="grid-template-columns: repeat(auto-fit, minmax(320px, 1fr))">
            <div class="card" style="box-shadow: none">
              <div class="card-title">请假类型分布</div>
              <EChart :option="typePie" height="260px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">按月请假趋势</div>
              <EChart :option="monthBar" height="260px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">请假天数 TOP 10</div>
              <el-table :data="stats.topStudents" size="small" stripe>
                <el-table-column label="#" width="50">
                  <template #default="{ $index }">
                    <span :style="{ color: ['#ee6b6e', '#f2a545', '#5aa9e6'][$index] ?? '#8a93a8', fontWeight: 700 }">{{ $index + 1 }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="k" label="姓名" width="90" />
                <el-table-column prop="class_name" label="班级" min-width="110" show-overflow-tooltip />
                <el-table-column prop="v" label="次数" width="70" />
                <el-table-column prop="d" label="天数" width="70">
                  <template #default="{ row }"><b style="color: #ee6b6e">{{ row.d }}</b></template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </template>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑请假记录' : '登记请假'" width="560px">
      <el-form :model="form" label-width="84px">
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="班级">
              <el-select v-model="formClass" placeholder="先选班级便于检索" clearable filterable style="width: 100%" @change="form.student_id = null">
                <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="学生" required label-width="64px">
              <StudentSelect v-model="form.student_id" :class-name="formClass" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="类型">
              <el-radio-group v-model="form.type">
                <el-radio v-for="t in ['事假', '病假', '其他']" :key="t" :value="t">{{ t }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio value="请假中">请假中</el-radio>
                <el-radio value="已销假">已销假</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="请假时间">
          <el-date-picker v-model="form.start_date" type="date" value-format="YYYY-MM-DD" style="width: 46%" />
          <span style="margin: 0 8px; color: #b0b7c9">至</span>
          <el-date-picker v-model="form.end_date" type="date" value-format="YYYY-MM-DD" style="width: 46%" />
        </el-form-item>
        <el-form-item label="天数"><el-input-number v-model="form.days" :min="0.5" :step="0.5" /></el-form-item>
        <el-form-item label="事由"><el-input v-model="form.reason" type="textarea" :rows="2" placeholder="如：感冒发烧 / 家中有事" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <ImportDialog v-model="importVisible" title="请假记录" :columns="importColumns" :sample="importSample"
      tip="姓名需与花名册一致才能匹配；日期格式如 2026-10-20" @confirm="onImport" />
  </div>
</template>
