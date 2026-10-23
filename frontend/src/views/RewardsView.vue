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
const filter = reactive({ class_name: '', kind: '', keyword: '', dateRange: null as null | [string, string] })
const page = ref(1)
const pageSize = ref(15)
const dialogVisible = ref(false)
const editing = ref<any>(null)
const saving = ref(false)
const importVisible = ref(false)
const stats = ref<any>(null)
const formClass = ref('') // 弹窗内用于缩小学生检索范围的班级

const form = ref<any>({})
const blank = () => ({ student_id: null, kind: '表扬', level: '班级', title: '', record_date: new Date().toISOString().slice(0, 10), reason: '', remark: '' })

const importColumns = [
  { key: 'student_name', label: '姓名', required: true },
  { key: 'class_name', label: '班级' },
  { key: 'kind', label: '类别' },
  { key: 'level', label: '级别' },
  { key: 'title', label: '事项' },
  { key: 'record_date', label: '日期' },
  { key: 'reason', label: '原因' },
  { key: 'remark', label: '备注' },
]
const importSample = [{ 姓名: '张三', 班级: '一年级1班', 类别: '表扬', 级别: '班级', 事项: '课堂发言积极', 日期: '2026-10-15', 原因: '', 备注: '' }]

const KIND_META: Record<string, { type: string; emoji: string }> = {
  表扬: { type: 'success', emoji: '👍' },
  奖励: { type: 'success', emoji: '🏅' },
  批评: { type: 'warning', emoji: '⚠️' },
  处分: { type: 'danger', emoji: '⛔' },
}

const paged = computed(() => list.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (filter.class_name) params.set('class_name', filter.class_name)
    if (filter.keyword) params.set('keyword', filter.keyword)
    if (filter.kind) params.set('kind', filter.kind)
    if (filter.dateRange?.[0]) params.set('date_from', filter.dateRange[0])
    if (filter.dateRange?.[1]) params.set('date_to', filter.dateRange[1])
    list.value = await get(`/api/rewards?${params}`)
    page.value = 1
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  stats.value = await get('/api/stats/rewards')
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
  if (!form.value.title?.trim()) return ElMessage.warning('请填写事项')
  saving.value = true
  try {
    if (editing.value) await put(`/api/rewards/${editing.value.id}`, form.value)
    else await post('/api/rewards', form.value)
    ElMessage.success('已保存')
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
  await ElMessageBox.confirm(`确定删除这条${row.kind}记录？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  await del(`/api/rewards/${row.id}`)
  ElMessage.success('已删除')
  load()
  loadStats()
}

async function onImport(records: any[]) {
  const res = await post('/api/rewards/batch', { records })
  if (res.fail) ElMessage.warning(`导入完成：成功 ${res.ok} 条，失败 ${res.fail} 条。${res.errors?.[0] ?? ''}`)
  else ElMessage.success(`成功导入 ${res.ok} 条奖惩记录`)
  load()
  loadStats()
}

const exportRows = computed(() =>
  list.value.map((r) => ({ 姓名: r.student_name, 班级: r.class_name, 类别: r.kind, 级别: r.level, 事项: r.title, 日期: r.record_date, 原因: r.reason, 备注: r.remark })),
)

const kindPie = computed(() => ({
  color: ['#22b8a5', '#4f6bed', '#f2a545', '#ee6b6e'],
  tooltip: { trigger: 'item' },
  legend: { bottom: 0 },
  series: [{
    type: 'pie', radius: ['45%', '70%'], center: ['50%', '45%'],
    itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
    label: { formatter: '{b} {c}条' },
    data: (stats.value?.byKind ?? []).map((d: any) => ({ name: d.k, value: d.v })),
  }],
}))

const monthLine = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: 10, right: 16, top: 24, bottom: 0, containLabel: true },
  xAxis: { type: 'category', data: (stats.value?.byMonth ?? []).map((d: any) => d.k), boundaryGap: false, axisTick: { show: false } },
  yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: '#eef0f6' } } },
  series: [{
    type: 'line', smooth: true, symbolSize: 7,
    data: (stats.value?.byMonth ?? []).map((d: any) => d.v),
    lineStyle: { width: 3, color: '#22b8a5' }, itemStyle: { color: '#22b8a5' },
    areaStyle: { color: 'rgba(34,184,165,0.15)' },
  }],
}))
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">奖惩记录</h2>
        <div class="page-sub">记录学生的表扬、奖励与批评教育，留下成长轨迹</div>
      </div>
    </div>

    <el-tabs v-model="tab" class="card">
      <el-tab-pane label="📋 记录列表" name="list">
        <div class="toolbar" style="margin-bottom: 14px">
          <el-input v-model="filter.keyword" placeholder="搜索姓名 / 事项" clearable style="width: 160px" @keyup.enter="load" @clear="load" />
          <el-select v-model="filter.class_name" placeholder="班级" clearable filterable style="width: 130px" @change="load">
            <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
          </el-select>
          <el-select v-model="filter.kind" placeholder="类别" clearable style="width: 96px" @change="load">
            <el-option v-for="k in ['表扬', '奖励', '批评', '处分']" :key="k" :value="k" :label="k" />
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
                <el-dropdown-item @click="exportExcel(exportRows, '奖惩记录')">导出 Excel</el-dropdown-item>
                <el-dropdown-item @click="exportCsv(exportRows, '奖惩记录')">导出 CSV</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button type="primary" @click="openAdd"><el-icon style="margin-right: 4px"><Plus /></el-icon>添加记录</el-button>
        </div>

        <el-table :data="paged" v-loading="loading" stripe>
          <el-table-column prop="record_date" label="日期" width="110" sortable />
          <el-table-column prop="student_name" label="姓名" width="95">
            <template #default="{ row }"><b>{{ row.student_name }}</b></template>
          </el-table-column>
          <el-table-column prop="class_name" label="班级" width="115" />
          <el-table-column prop="kind" label="类别" width="86">
            <template #default="{ row }">
              <el-tag size="small" :type="(KIND_META[row.kind]?.type as any) ?? 'info'" effect="light">{{ KIND_META[row.kind]?.emoji }} {{ row.kind }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="level" label="级别" width="76" />
          <el-table-column prop="title" label="事项" min-width="170" show-overflow-tooltip />
          <el-table-column prop="reason" label="原因 / 说明" min-width="150" show-overflow-tooltip />
          <el-table-column label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button link type="danger" @click="remove(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><div class="empty-tip">暂无奖惩记录</div></template>
        </el-table>

        <div style="display: flex; justify-content: flex-end; margin-top: 12px">
          <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="list.length" :page-sizes="[15, 30, 50]" layout="total, sizes, prev, pager, next" background />
        </div>
      </el-tab-pane>

      <el-tab-pane label="📊 统计分析" name="stats" lazy>
        <template v-if="stats">
          <div class="grid-cards" style="grid-template-columns: repeat(auto-fit, minmax(320px, 1fr))">
            <div class="card" style="box-shadow: none">
              <div class="card-title">类别分布</div>
              <EChart :option="kindPie" height="250px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">按月记录趋势</div>
              <EChart :option="monthLine" height="250px" />
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">表扬榜 TOP 10 🌟</div>
              <el-table :data="stats.topGood" size="small" stripe>
                <el-table-column label="#" width="44"><template #default="{ $index }">{{ $index + 1 }}</template></el-table-column>
                <el-table-column prop="k" label="姓名" width="90" />
                <el-table-column prop="class_name" label="班级" min-width="100" show-overflow-tooltip />
                <el-table-column prop="v" label="次数" width="64" />
              </el-table>
            </div>
            <div class="card" style="box-shadow: none">
              <div class="card-title">需关注学生（批评/处分较多）</div>
              <el-table :data="stats.topBad" size="small" stripe>
                <el-table-column label="#" width="44"><template #default="{ $index }">{{ $index + 1 }}</template></el-table-column>
                <el-table-column prop="k" label="姓名" width="90" />
                <el-table-column prop="class_name" label="班级" min-width="100" show-overflow-tooltip />
                <el-table-column prop="v" label="次数" width="64" />
              </el-table>
            </div>
          </div>
        </template>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑记录' : '添加奖惩记录'" width="540px">
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
        <el-form-item label="类别">
          <el-radio-group v-model="form.kind">
            <el-radio-button v-for="k in ['表扬', '奖励', '批评', '处分']" :key="k" :value="k">{{ k }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="级别">
              <el-select v-model="form.level" style="width: 100%">
                <el-option v-for="l in ['班级', '年级', '校级', '县区', '市级', '省级']" :key="l" :value="l" :label="l" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="日期"><el-date-picker v-model="form.record_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" /></el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="事项" required><el-input v-model="form.title" placeholder="如：课堂发言积极 / 作业拖欠" /></el-form-item>
        <el-form-item label="原因说明"><el-input v-model="form.reason" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <ImportDialog v-model="importVisible" title="奖惩记录" :columns="importColumns" :sample="importSample"
      tip="姓名需与花名册一致；类别支持「表扬 / 奖励 / 批评 / 处分」" @confirm="onImport" />
  </div>
</template>
