<script setup lang="ts">
import { onMounted, computed, ref } from 'vue'
import { get } from '@/api'
import EChart from '@/components/EChart.vue'
import { ElMessage } from 'element-plus'

const data = ref<any>({
  students: 0, classes: 0, exams: 0, monthLeaves: 0, pendingLeaves: 0, rewards: 0,
  recentLeaves: [], genderDist: [], classDist: [], leaveTrend: [],
})

const palette = ['#4f6bed', '#7c5bf5', '#22b8a5', '#f2a545', '#ee6b6e', '#5aa9e6']

const genderOption = computed(() => ({
  color: [palette[0], '#ee8fb3', palette[4]],
  tooltip: { trigger: 'item' },
  legend: { bottom: 0 },
  series: [{
    type: 'pie', radius: ['45%', '70%'], center: ['50%', '45%'],
    itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
    label: { formatter: '{b} {c}人' },
    data: [...data.value.genderDist]
      .sort((a: any, b: any) => (a.k === '男' ? -1 : 0) - (b.k === '男' ? -1 : 0))
      .map((d: any) => ({ name: d.k || '未填', value: d.v })),
  }],
}))

const classOption = computed(() => ({
  color: palette,
  tooltip: { trigger: 'axis' },
  grid: { left: 10, right: 16, top: 24, bottom: 0, containLabel: true },
  xAxis: { type: 'category', data: data.value.classDist.map((d: any) => d.k), axisLine: { show: false }, axisTick: { show: false } },
  yAxis: { type: 'value', splitLine: { lineStyle: { color: '#eef0f6' } } },
  series: [{
    type: 'bar', data: data.value.classDist.map((d: any) => d.v), barWidth: 22,
    itemStyle: { borderRadius: [6, 6, 0, 0], color: '#4f6bed' },
    label: { show: true, position: 'top', color: '#8a93a8' },
  }],
}))

const leaveTrendOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: 10, right: 16, top: 24, bottom: 0, containLabel: true },
  xAxis: { type: 'category', data: data.value.leaveTrend.map((d: any) => d.k), boundaryGap: false, axisLine: { show: false }, axisTick: { show: false } },
  yAxis: { type: 'value', splitLine: { lineStyle: { color: '#eef0f6' } }, minInterval: 1 },
  series: [{
    type: 'line', smooth: true, data: data.value.leaveTrend.map((d: any) => d.v),
    symbolSize: 7, lineStyle: { width: 3, color: '#f2a545' }, itemStyle: { color: '#f2a545' },
    areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(242,165,69,0.28)' }, { offset: 1, color: 'rgba(242,165,69,0.02)' }] } },
  }],
}))

const cards = computed(() => [
  { label: '在读学生', num: data.value.students, icon: '🎓', bg: '#eef2ff' },
  { label: '班级数量', num: data.value.classes, icon: '🏫', bg: '#effaf7' },
  { label: '考试记录', num: data.value.exams, icon: '📝', bg: '#f4f0ff' },
  { label: '本月请假人次', num: data.value.monthLeaves, icon: '📅', bg: '#fef6ec' },
  { label: '未销假', num: data.value.pendingLeaves, icon: '⏳', bg: '#fdeeee' },
  { label: '表扬/奖励', num: data.value.rewards, icon: '🏅', bg: '#edf9ff' },
])

onMounted(async () => {
  try {
    data.value = await get('/api/stats/overview')
  } catch (e: any) {
    ElMessage.error(e.message)
  }
})
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">工作台总览</h2>
        <div class="page-sub">一眼掌握班级、请假、考试的整体情况</div>
      </div>
    </div>

    <div class="stat-cards">
      <div v-for="c in cards" :key="c.label" class="stat-card">
        <div class="stat-icon" :style="{ background: c.bg }">{{ c.icon }}</div>
        <div>
          <div class="stat-num">{{ c.num }}</div>
          <div class="stat-label">{{ c.label }}</div>
        </div>
      </div>
    </div>

    <div class="grid-cards" style="margin-top: 16px">
      <div class="card">
        <div class="card-title">班级人数分布</div>
        <EChart v-if="data.classDist.length" :option="classOption" height="260px" />
        <div v-else class="empty-tip">暂无班级数据，请先在「学生花名册」中添加学生</div>
      </div>
      <div class="card">
        <div class="card-title">在读学生性别比</div>
        <EChart v-if="data.genderDist.length" :option="genderOption" height="260px" />
        <div v-else class="empty-tip">暂无学生数据</div>
      </div>
    </div>

    <div class="grid-cards" style="margin-top: 16px; grid-template-columns: 1fr 1fr">
      <div class="card">
        <div class="card-title">近 12 个月请假人次</div>
        <EChart v-if="data.leaveTrend.length" :option="leaveTrendOption" height="250px" />
        <div v-else class="empty-tip">暂无请假记录</div>
      </div>
      <div class="card">
        <div class="card-title">最近请假动态</div>
        <el-table v-if="data.recentLeaves.length" :data="data.recentLeaves" size="small" :show-header="false">
          <el-table-column width="90">
            <template #default="{ row }"><b>{{ row.student_name }}</b></template>
          </el-table-column>
          <el-table-column prop="class_name" width="90" />
          <el-table-column width="70">
            <template #default="{ row }">
              <el-tag size="small" :type="row.type === '病假' ? 'danger' : 'warning'" effect="light">{{ row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="start_date" width="105" />
          <el-table-column>
            <template #default="{ row }">
              <span style="color: #8a93a8; font-size: 12.5px">{{ row.reason || '—' }}</span>
            </template>
          </el-table-column>
        </el-table>
        <div v-else class="empty-tip">近期没有请假记录 👍</div>
      </div>
    </div>
  </div>
</template>
