<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchClasses, fetchStudents, get, put } from '@/api'

interface SeatStudent {
  id: number
  name: string
  gender: string
  student_no: string
}

const className = ref('')
const classes = ref<string[]>([])
const students = ref<SeatStudent[]>([])
const rowCount = ref(6)
const colCount = ref(8)
const groupSize = ref(2)
const grid = ref<(SeatStudent | null)[][]>([])
const pool = ref<SeatStudent[]>([])
const loaded = ref(false)
const saving = ref(false)
const dragOver = ref<string | null>(null)
const withPoolShuffle = ref(true)

// 位置颜色：键为 “行-列”（0 起）
const colors = ref<Record<string, string>>({})
const paletteMode = ref(false)
const activeColor = ref('#fde8e8')
const eraser = ref(false)
// 点击选中（再点另一个位置即互换/落座）
const selected = ref<{ type: 'seat'; row: number; col: number } | { type: 'pool'; id: number } | null>(null)

const PALETTE = [
  '#fde8e8', '#fdeeda', '#fef7d6', '#e3f6df', '#ddf3f0', '#deedfd',
  '#e6e6fb', '#f6e2f0', '#f0f1f5', '#ffe3c7', '#d9f2e6', '#e8f3d6',
]

const byId = computed(() => new Map(students.value.map((s) => [s.id, s])))

function emptyGrid(rows: number, cols: number) {
  return Array.from({ length: rows }, () => Array.from({ length: cols }, () => null as SeatStudent | null))
}

function rebuildPool() {
  const seated = new Set<number>()
  for (const row of grid.value) for (const c of row) if (c) seated.add(c.id)
  pool.value = students.value.filter((s) => !seated.has(s.id))
}

async function onClassChange() {
  if (!className.value) return
  loaded.value = false
  const [stus, layout] = await Promise.all([fetchStudents(className.value), get(`/api/seats/${encodeURIComponent(className.value)}`)])
  students.value = stus.filter((s: any) => s.status === '在读')
  colCount.value = layout.cols ?? 8
  groupSize.value = layout.group_size ?? 2
  let rows = layout.rows ?? Math.max(4, Math.ceil(students.value.length / colCount.value)) + 0
  if (!layout.rows) rows = Math.max(4, Math.ceil(students.value.length / colCount.value))
  rowCount.value = rows
  grid.value = emptyGrid(rows, colCount.value)
  for (const a of layout.assignments ?? []) {
    if (a.row < rows && a.col < colCount.value) {
      const stu = byId.value.get(a.student_id)
      if (stu) grid.value[a.row]![a.col] = stu
    }
  }
  colors.value = { ...(layout.colors ?? {}) }
  selected.value = null
  rebuildPool()
  loaded.value = true
}

function applyConfig() {
  const old = grid.value
  const next = emptyGrid(rowCount.value, colCount.value)
  for (let r = 0; r < Math.min(old.length, next.length); r++) {
    for (let c = 0; c < Math.min(old[r]!.length, next[r]!.length); c++) {
      next[r]![c] = old[r]![c]
    }
  }
  grid.value = next
  // 裁剪超出新范围的座位颜色
  for (const k of Object.keys(colors.value)) {
    const [r, c] = k.split('-').map(Number)
    if (r! >= rowCount.value || c! >= colCount.value) delete colors.value[k]
  }
  selected.value = null
  rebuildPool()
}

// ---------- 拖拽 ----------
function onDragStart(e: DragEvent, src: { from: 'seat'; row: number; col: number } | { from: 'pool'; id: number }) {
  e.dataTransfer?.setData('text/plain', JSON.stringify(src))
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDropSeat(e: DragEvent, row: number, col: number) {
  e.preventDefault()
  dragOver.value = null
  selected.value = null
  const raw = e.dataTransfer?.getData('text/plain')
  if (!raw) return
  const src = JSON.parse(raw)
  const target = grid.value[row]![col]
  if (src.from === 'seat') {
    if (src.row === row && src.col === col) return
    const moving = dragSnapshot ?? grid.value[src.row]![src.col]
    grid.value[src.row]![src.col] = target
    grid.value[row]![col] = moving
    dragSnapshot = null
  } else {
    const stu = byId.value.get(src.id)
    if (!stu) return
    grid.value[row]![col] = stu
    rebuildPool()
  }
}

let dragSnapshot: SeatStudent | null = null
function captureDrag(e: DragEvent, row: number, col: number) {
  dragSnapshot = grid.value[row]![col]
  onDragStart(e, { from: 'seat', row, col })
}

function onDropPool(e: DragEvent) {
  e.preventDefault()
  dragOver.value = null
  selected.value = null
  const raw = e.dataTransfer?.getData('text/plain')
  if (!raw) return
  const src = JSON.parse(raw)
  if (src.from === 'seat') {
    grid.value[src.row]![src.col] = null
    rebuildPool()
  }
}

// ---------- 点击互换 / 上色 ----------
function seatKey(r: number, c: number) {
  return `${r}-${c}`
}

function seatStyle(r: number, c: number) {
  const bg = colors.value[seatKey(r, c)]
  return bg ? { background: bg, borderStyle: 'solid', borderColor: bg } : {}
}

function paint(r: number, c: number) {
  const k = seatKey(r, c)
  if (eraser.value) delete colors.value[k]
  else colors.value[k] = activeColor.value
}

function onSeatClick(r: number, c: number) {
  if (paletteMode.value) {
    paint(r, c)
    return
  }
  const cur = grid.value[r]![c]
  const sel = selected.value
  if (sel?.type === 'seat') {
    if (sel.row === r && sel.col === c) {
      selected.value = null
      return
    }
    const moving = grid.value[sel.row]![sel.col]
    grid.value[sel.row]![sel.col] = cur
    grid.value[r]![c] = moving
    selected.value = null
    rebuildPool()
    return
  }
  if (sel?.type === 'pool') {
    const stu = byId.value.get(sel.id)
    if (stu) {
      grid.value[r]![c] = stu
      rebuildPool()
    }
    selected.value = null
    return
  }
  if (cur) selected.value = { type: 'seat', row: r, col: c }
}

function onPoolChipClick(s: SeatStudent) {
  if (paletteMode.value) return
  const sel = selected.value
  selected.value = sel?.type === 'pool' && sel.id === s.id ? null : { type: 'pool', id: s.id }
}

function togglePalette() {
  paletteMode.value = !paletteMode.value
  selected.value = null
}

function clearColors() {
  colors.value = {}
}

// ---------- 操作 ----------
function autoArrange() {
  selected.value = null
  const empty: [number, number][] = []
  grid.value.forEach((row, r) => row.forEach((c, col) => { if (!c) empty.push([r, col]) }))
  const sorted = [...pool.value].sort((a, b) => (Number(a.student_no) || 999) - (Number(b.student_no) || 999) || a.name.localeCompare(b.name, 'zh'))
  empty.forEach(([r, c], i) => {
    if (i < sorted.length) grid.value[r]![c] = sorted[i]!
  })
  rebuildPool()
  ElMessage.success('已按学号顺序自动填充空位')
}

function shuffle() {
  selected.value = null
  const seats: [number, number][] = []
  const occupants: SeatStudent[] = []
  grid.value.forEach((row, r) =>
    row.forEach((c, col) => {
      if (c) {
        seats.push([r, col])
        occupants.push(c)
      }
    }),
  )
  if (withPoolShuffle.value && pool.value.length) {
    occupants.push(...pool.value)
    const empties: [number, number][] = []
    grid.value.forEach((row, r) => row.forEach((c, col) => { if (!c) empties.push([r, col]) }))
    for (const pos of empties) {
      if (seats.length < occupants.length) seats.push(pos)
    }
  }
  grid.value = emptyGrid(rowCount.value, colCount.value)
  for (let i = occupants.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[occupants[i], occupants[j]] = [occupants[j]!, occupants[i]!]
  }
  seats.sort((a, b) => a[0] - b[0] || a[1] - b[1])
  seats.forEach(([r, c], i) => {
    if (i < occupants.length) grid.value[r]![c] = occupants[i]!
  })
  rebuildPool()
  ElMessage.success('已随机打乱座位')
}

function clearAll() {
  selected.value = null
  grid.value = emptyGrid(rowCount.value, colCount.value)
  rebuildPool()
}

async function save() {
  const assignments: any[] = []
  grid.value.forEach((row, r) =>
    row.forEach((c, col) => {
      if (c) assignments.push({ row: r, col, student_id: c.id })
    }),
  )
  saving.value = true
  try {
    await put(`/api/seats/${encodeURIComponent(className.value)}`, {
      rows: rowCount.value,
      cols: colCount.value,
      group_size: groupSize.value,
      assignments,
      colors: colors.value,
    })
    ElMessage.success('座位表已保存')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

// 分组渲染：把列按 groupSize 切成组
const groups = computed(() => {
  const out: number[][] = []
  let cur: number[] = []
  for (let c = 0; c < colCount.value; c++) {
    cur.push(c)
    if (cur.length === groupSize.value) {
      out.push(cur)
      cur = []
    }
  }
  if (cur.length) out.push(cur)
  return out
})

const WEEK_SEAT_TIP = '拖动或点击学生卡片后再点另一个位置即可互换；开启上色模式可给座位标颜色'

onMounted(async () => {
  classes.value = await fetchClasses()
  if (classes.value.length) {
    className.value = classes.value[0]!
    await onClassChange()
  }
})
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">座位表</h2>
        <div class="page-sub">{{ WEEK_SEAT_TIP }}</div>
      </div>
    </div>

    <div class="card" style="margin-bottom: 16px">
      <div class="toolbar">
        <el-select v-model="className" placeholder="选择班级" style="width: 160px" @change="onClassChange">
          <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
        </el-select>
        <div class="cfg"><span>行数</span><el-input-number v-model="rowCount" :min="3" :max="12" size="small" style="width: 84px" @change="applyConfig" /></div>
        <div class="cfg"><span>列数</span><el-input-number v-model="colCount" :min="4" :max="14" size="small" style="width: 84px" @change="applyConfig" /></div>
        <div class="cfg"><span>过道分组</span><el-input-number v-model="groupSize" :min="1" :max="4" size="small" style="width: 84px" /></div>
        <div style="flex: 1" />
        <template v-if="loaded">
          <el-checkbox v-model="withPoolShuffle" size="small">打乱时包含待安排学生</el-checkbox>
          <el-button :type="paletteMode ? 'warning' : 'default'" :plain="!paletteMode" @click="togglePalette">
            <el-icon style="margin-right: 4px"><Brush /></el-icon>{{ paletteMode ? '退出上色' : '上色模式' }}
          </el-button>
          <el-button @click="autoArrange"><el-icon style="margin-right: 4px"><MagicStick /></el-icon>自动排列</el-button>
          <el-button @click="shuffle"><el-icon style="margin-right: 4px"><Refresh /></el-icon>随机打乱</el-button>
          <el-button @click="clearAll"><el-icon style="margin-right: 4px"><Delete /></el-icon>清空</el-button>
          <el-button type="primary" :loading="saving" @click="save"><el-icon style="margin-right: 4px"><Check /></el-icon>保存座位表</el-button>
        </template>
      </div>
      <!-- 色板 -->
      <div v-if="loaded && paletteMode" class="palette-bar">
        <span class="palette-label">点选颜色后点击座位即可上色：</span>
        <button
          v-for="p in PALETTE"
          :key="p"
          class="swatch"
          :class="{ active: !eraser && activeColor === p }"
          :style="{ background: p }"
          @click="activeColor = p; eraser = false"
        />
        <el-color-picker v-model="activeColor" size="small" :predefine="PALETTE" @change="eraser = false" />
        <button class="swatch eraser" :class="{ active: eraser }" title="橡皮擦：点击座位清除颜色" @click="eraser = true">
          <el-icon><Remove /></el-icon>
        </button>
        <el-button link size="small" type="danger" style="margin-left: 6px" @click="clearColors">清除全部颜色</el-button>
      </div>
    </div>

    <template v-if="loaded">
      <div class="card classroom">
        <!-- 黑板 -->
        <div class="blackboard">黑 板</div>
        <!-- 讲台 -->
        <div class="podium-wrap">
          <div class="podium">
            <span class="podium-text">讲 台</span>
          </div>
        </div>

        <!-- 座位区 -->
        <div class="seat-area">
          <div class="rows-wrap">
            <div class="row-labels">
              <div v-for="r in rowCount" :key="r" class="row-label">{{ r }}</div>
            </div>
            <div class="rows">
              <div v-for="r in rowCount" :key="r" class="seat-row" :style="{ gap: '28px' }">
                <div v-for="(grp, gi) in groups" :key="gi" class="seat-group">
                  <div
                    v-for="c in grp"
                    :key="c"
                    class="seat"
                    :class="{
                      filled: grid[r - 1]?.[c],
                      female: !colors[`${r - 1}-${c}`] && grid[r - 1]?.[c]?.gender === '女',
                      male: !colors[`${r - 1}-${c}`] && grid[r - 1]?.[c]?.gender === '男',
                      over: dragOver === `${r - 1}-${c}`,
                      pick: selected?.type === 'seat' && selected.row === r - 1 && selected.col === c,
                      clickable: paletteMode || !!grid[r - 1]?.[c] || !!selected,
                    }"
                    :style="seatStyle(r - 1, c)"
                    :draggable="!!grid[r - 1]?.[c] && !paletteMode"
                    @click="onSeatClick(r - 1, c)"
                    @dragstart="(e) => captureDrag(e as DragEvent, r - 1, c)"
                    @dragover.prevent="dragOver = `${r - 1}-${c}`"
                    @dragleave="dragOver = null"
                    @drop="(e) => onDropSeat(e as DragEvent, r - 1, c)"
                  >
                    <template v-if="grid[r - 1]?.[c]">
                      <div class="seat-name">{{ grid[r - 1]![c]!.name }}</div>
                      <div class="seat-no">{{ grid[r - 1]![c]!.student_no || '' }}</div>
                    </template>
                    <span v-else class="seat-empty">+</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="legend">
            <span class="lg"><i class="dot male"></i>男生</span>
            <span class="lg"><i class="dot female"></i>女生</span>
            <span class="lg"><i class="dot empty"></i>空位</span>
            <span class="lg" style="margin-left: auto">左门 ← → 后门</span>
          </div>
        </div>
      </div>

      <!-- 待安排 -->
      <div class="card" style="margin-top: 16px" :class="{ over: dragOver === 'pool' }" @dragover.prevent="dragOver = 'pool'" @dragleave="dragOver = null" @drop="onDropPool">
        <div class="card-title">待安排座位（{{ pool.length }} 人）<span class="hint">拖到这里可从座位上移除；点选学生再点座位可直接落座</span></div>
        <div v-if="pool.length" class="pool">
          <div
            v-for="s in pool"
            :key="s.id"
            class="pool-chip"
            :class="{ female: s.gender === '女', pick: selected?.type === 'pool' && selected.id === s.id }"
            :draggable="!paletteMode"
            @click="onPoolChipClick(s)"
            @dragstart="(e) => onDragStart(e as DragEvent, { from: 'pool', id: s.id })"
          >
            {{ s.name }}
          </div>
        </div>
        <div v-else class="empty-tip" style="padding: 14px 0">所有学生都已有座位 ✅</div>
      </div>
    </template>

    <div v-else class="card">
      <div class="empty-tip" style="padding: 60px 0">
        {{ classes.length ? '请选择班级查看座位表' : '还没有班级数据，请先在「学生花名册」中添加学生并填写班级' }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.cfg {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #8a93a8;
  font-size: 13px;
}

.classroom {
  background: #fbfcff;
  background: linear-gradient(to bottom, #fbfcff, #f7f9fe);
}

.blackboard {
  width: 62%;
  margin: 0 auto 10px;
  background: #36544e;
  background: linear-gradient(to bottom, #36544e, #2c4742);
  color: #cfe3dd;
  border-radius: 6px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  letter-spacing: 10px;
  font-size: 14px;
  box-shadow: inset 0 0 0 3px #7a5c3e, 0 4px 10px rgba(30, 41, 72, 0.12);
}

.podium-wrap {
  display: flex;
  justify-content: center;
  margin-bottom: 26px;
}

.podium {
  width: 220px;
  height: 54px;
  background: #e8edf7;
  background: linear-gradient(to bottom, #e8edf7, #d7deee);
  clip-path: polygon(6% 0, 94% 0, 100% 100%, 0 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #5a6378;
  font-weight: 600;
  letter-spacing: 6px;
  box-shadow: 0 6px 14px rgba(30, 41, 72, 0.1);
}

.seat-area {
  padding: 4px 8px 8px;
}

.rows-wrap {
  display: flex;
  gap: 12px;
  justify-content: center;
}

.row-labels {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 2px;
}

.row-label {
  height: 56px;
  width: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #b0b7c9;
  font-size: 12px;
  border-right: 2px dashed #e3e7f2;
  padding-right: 8px;
}

.rows {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.seat-row {
  display: flex;
}

.seat-group {
  display: flex;
  gap: 8px;
}

.seat {
  width: 78px;
  height: 56px;
  border-radius: 10px;
  border: 1.6px dashed #d5dbe9;
  background: #fff;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
  position: relative;
  user-select: none;
}

.seat.filled {
  border-style: solid;
  cursor: grab;
}

.seat.filled:active {
  cursor: grabbing;
}

.seat.male {
  background: #eef3ff;
  border-color: #bfd0fb;
  box-shadow: 0 2px 5px rgba(79, 107, 237, 0.08);
}

.seat.female {
  background: #fdeef5;
  border-color: #f3bfd6;
  box-shadow: 0 2px 5px rgba(208, 90, 139, 0.09);
}

.seat.over {
  outline: 2px solid #4f6bed;
  transform: scale(1.06);
  z-index: 2;
}

.seat.clickable {
  cursor: pointer;
}

.seat.pick {
  outline: 2.5px solid #4f6bed;
  outline-offset: -2.5px;
  transform: scale(1.06);
  z-index: 2;
  box-shadow: 0 4px 12px rgba(79, 107, 237, 0.28);
}

.palette-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed #e3e7f2;
  flex-wrap: wrap;
}

.palette-label {
  color: #8a93a8;
  font-size: 13px;
}

.swatch {
  width: 26px;
  height: 26px;
  border-radius: 8px;
  border: 1px solid rgba(90, 99, 120, 0.18);
  cursor: pointer;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.1s ease;
}

.swatch:hover {
  transform: scale(1.12);
}

.swatch.active {
  outline: 2.5px solid #4f6bed;
  outline-offset: 1.5px;
}

.swatch.eraser {
  background: #fff;
  color: #d05a5a;
  font-size: 14px;
}

.pool-chip.pick {
  outline: 2.5px solid #4f6bed;
  outline-offset: -1px;
  box-shadow: 0 3px 10px rgba(79, 107, 237, 0.28);
  cursor: pointer;
}

.seat-name {
  font-weight: 650;
  font-size: 13.5px;
  color: #2a3145;
}

.seat-no {
  font-size: 10.5px;
  color: #98a0b5;
  margin-top: 1px;
}

.seat-empty {
  color: #c6cddd;
  font-size: 16px;
}

.legend {
  display: flex;
  align-items: center;
  gap: 18px;
  margin-top: 18px;
  color: #8a93a8;
  font-size: 12.5px;
}

.dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 3px;
  margin-right: 5px;
}

.dot.male {
  background: #bfd0fb;
}
.dot.female {
  background: #f3bfd6;
}
.dot.empty {
  border: 1.5px dashed #c6cddd;
  background: #fff;
}

.pool {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pool-chip {
  padding: 7px 14px;
  border-radius: 999px;
  background: #eef3ff;
  border: 1px solid #cfdafc;
  color: #3d52c9;
  font-size: 13px;
  font-weight: 600;
  cursor: grab;
  transition: transform 0.12s ease;
}

.pool-chip:hover {
  transform: translateY(-1px);
}

.pool-chip.female {
  background: #fdeef5;
  border-color: #f3bfd6;
  color: #c24e85;
}

.card.over {
  outline: 2px dashed #4f6bed;
  outline-offset: -2px;
}

.hint {
  font-weight: 400;
  color: #b0b7c9;
  font-size: 12px;
  margin-left: 8px;
}
</style>
